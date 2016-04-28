package worker

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/tuna/tunasync/internal"
)

var tunasyncWorker *Worker

// A Worker is a instance of tunasync worker
type Worker struct {
	cfg       *Config
	providers map[string]mirrorProvider
	jobs      map[string]*mirrorJob

	managerChan chan jobMessage
	semaphore   chan empty

	schedule   *scheduleQueue
	httpServer *gin.Engine
	tlsConfig  *tls.Config
}

// GetTUNASyncWorker returns a singalton worker
func GetTUNASyncWorker(cfg *Config) *Worker {
	if tunasyncWorker != nil {
		return tunasyncWorker
	}

	w := &Worker{
		cfg:       cfg,
		providers: make(map[string]mirrorProvider),
		jobs:      make(map[string]*mirrorJob),

		managerChan: make(chan jobMessage, 32),
		semaphore:   make(chan empty, cfg.Global.Concurrent),

		schedule: newScheduleQueue(),
	}

	if cfg.Manager.CACert != "" {
		tlsConfig, err := GetTLSConfig(cfg.Manager.CACert)
		if err != nil {
			logger.Error("Failed to init TLS config: %s", err.Error())
			return nil
		}
		w.tlsConfig = tlsConfig
	}

	w.initJobs()
	w.makeHTTPServer()
	tunasyncWorker = w
	return w
}

func (w *Worker) initProviders() {
	c := w.cfg

	formatLogDir := func(logDir string, m mirrorConfig) string {
		tmpl, err := template.New("logDirTmpl-" + m.Name).Parse(logDir)
		if err != nil {
			panic(err)
		}
		var formatedLogDir bytes.Buffer
		tmpl.Execute(&formatedLogDir, m)
		return formatedLogDir.String()
	}

	for _, mirror := range c.Mirrors {
		logDir := mirror.LogDir
		mirrorDir := mirror.MirrorDir
		if logDir == "" {
			logDir = c.Global.LogDir
		}
		if mirrorDir == "" {
			mirrorDir = filepath.Join(
				c.Global.MirrorDir, mirror.Name,
			)
		}
		if mirror.Interval == 0 {
			mirror.Interval = c.Global.Interval
		}
		logDir = formatLogDir(logDir, mirror)

		var provider mirrorProvider

		switch mirror.Provider {
		case ProvCommand:
			pc := cmdConfig{
				name:        mirror.Name,
				upstreamURL: mirror.Upstream,
				command:     mirror.Command,
				workingDir:  mirrorDir,
				logDir:      logDir,
				logFile:     filepath.Join(logDir, "latest.log"),
				interval:    time.Duration(mirror.Interval) * time.Minute,
				env:         mirror.Env,
			}
			p, err := newCmdProvider(pc)
			if err != nil {
				panic(err)
			}
			provider = p
		case ProvRsync:
			rc := rsyncConfig{
				name:        mirror.Name,
				upstreamURL: mirror.Upstream,
				rsyncCmd:    mirror.Command,
				password:    mirror.Password,
				excludeFile: mirror.ExcludeFile,
				workingDir:  mirrorDir,
				logDir:      logDir,
				logFile:     filepath.Join(logDir, "latest.log"),
				useIPv6:     mirror.UseIPv6,
				interval:    time.Duration(mirror.Interval) * time.Minute,
			}
			p, err := newRsyncProvider(rc)
			if err != nil {
				panic(err)
			}
			provider = p
		case ProvTwoStageRsync:
			rc := twoStageRsyncConfig{
				name:          mirror.Name,
				stage1Profile: mirror.Stage1Profile,
				upstreamURL:   mirror.Upstream,
				rsyncCmd:      mirror.Command,
				password:      mirror.Password,
				excludeFile:   mirror.ExcludeFile,
				workingDir:    mirrorDir,
				logDir:        logDir,
				logFile:       filepath.Join(logDir, "latest.log"),
				useIPv6:       mirror.UseIPv6,
				interval:      time.Duration(mirror.Interval) * time.Minute,
			}
			p, err := newTwoStageRsyncProvider(rc)
			if err != nil {
				panic(err)
			}
			provider = p
		default:
			panic(errors.New("Invalid mirror provider"))

		}

		provider.AddHook(newLogLimiter(provider))
		w.providers[provider.Name()] = provider

	}
}

func (w *Worker) initJobs() {
	w.initProviders()

	for name, provider := range w.providers {
		w.jobs[name] = newMirrorJob(provider)
	}
}

// Ctrl server receives commands from the manager
func (w *Worker) makeHTTPServer() {
	s := gin.New()
	s.Use(gin.Recovery())

	s.POST("/", func(c *gin.Context) {
		var cmd WorkerCmd

		if err := c.BindJSON(&cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Invalid request"})
			return
		}
		job, ok := w.jobs[cmd.MirrorID]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"msg": fmt.Sprintf("Mirror ``%s'' not found", cmd.MirrorID)})
			return
		}
		logger.Info("Received command: %v", cmd)
		// if job disabled, start them first
		switch cmd.Cmd {
		case CmdStart, CmdRestart:
			if job.isDisabled {
				go job.Run(w.managerChan, w.semaphore)
			}
		}
		switch cmd.Cmd {
		case CmdStart:
			job.schedule = true
			job.isDisabled = false
			job.ctrlChan <- jobStart
		case CmdRestart:
			job.schedule = true
			job.isDisabled = false
			job.ctrlChan <- jobRestart
		case CmdStop:
			// if job is disabled, no goroutine would be there
			// receiving this signal
			if !job.isDisabled {
				job.schedule = false
				job.isDisabled = false
				w.schedule.Remove(job.Name())
				job.ctrlChan <- jobStop
			}
		case CmdDisable:
			if !job.isDisabled {
				job.schedule = false
				job.isDisabled = true
				w.schedule.Remove(job.Name())
				job.ctrlChan <- jobDisable
				<-job.disabled
			}
		case CmdPing:
			job.ctrlChan <- jobStart
		default:
			c.JSON(http.StatusNotAcceptable, gin.H{"msg": "Invalid Command"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "OK"})
	})

	w.httpServer = s
}

func (w *Worker) runHTTPServer() {
	addr := fmt.Sprintf("%s:%d", w.cfg.Server.Addr, w.cfg.Server.Port)

	if w.cfg.Server.SSLCert == "" && w.cfg.Server.SSLKey == "" {
		if err := w.httpServer.Run(addr); err != nil {
			panic(err)
		}
	} else {
		if err := w.httpServer.RunTLS(addr, w.cfg.Server.SSLCert, w.cfg.Server.SSLKey); err != nil {
			panic(err)
		}
	}
}

// Run runs worker forever
func (w *Worker) Run() {
	w.registorWorker()
	go w.runHTTPServer()
	w.runSchedule()
}

func (w *Worker) runSchedule() {
	mirrorList := w.fetchJobStatus()
	unset := make(map[string]bool)
	for name := range w.jobs {
		unset[name] = true
	}
	// Fetch mirror list stored in the manager
	// put it on the scheduled time
	// if it's disabled, ignore it
	for _, m := range mirrorList {
		if job, ok := w.jobs[m.Name]; ok {
			delete(unset, m.Name)
			switch m.Status {
			case Paused:
				go job.Run(w.managerChan, w.semaphore)
				job.schedule = false
				continue
			case Disabled:
				job.schedule = false
				job.isDisabled = true
				continue
			default:
				go job.Run(w.managerChan, w.semaphore)
				stime := m.LastUpdate.Add(job.provider.Interval())
				logger.Debug("Scheduling job %s @%s", job.Name(), stime.Format("2006-01-02 15:04:05"))
				w.schedule.AddJob(stime, job)
			}
		}
	}
	for name := range unset {
		job := w.jobs[name]
		go job.Run(w.managerChan, w.semaphore)
		w.schedule.AddJob(time.Now(), job)
	}

	for {
		select {
		case jobMsg := <-w.managerChan:
			// got status update from job
			job := w.jobs[jobMsg.name]
			if !job.schedule {
				logger.Info("Job %s disabled/paused, skip adding new schedule", jobMsg.name)
				continue
			}

			w.updateStatus(jobMsg)

			if jobMsg.schedule {
				schedTime := time.Now().Add(job.provider.Interval())
				logger.Info(
					"Next scheduled time for %s: %s",
					job.Name(),
					schedTime.Format("2006-01-02 15:04:05"),
				)
				w.schedule.AddJob(schedTime, job)
			}

		case <-time.Tick(5 * time.Second):
			// check schedule every 5 seconds
			if job := w.schedule.Pop(); job != nil {
				job.ctrlChan <- jobStart
			}
		}

	}

}

// Name returns worker name
func (w *Worker) Name() string {
	return w.cfg.Global.Name
}

// URL returns the url to http server of the worker
func (w *Worker) URL() string {
	proto := "https"
	if w.cfg.Server.SSLCert == "" && w.cfg.Server.SSLKey == "" {
		proto = "http"
	}

	return fmt.Sprintf("%s://%s:%d/", proto, w.cfg.Server.Hostname, w.cfg.Server.Port)
}

func (w *Worker) registorWorker() {
	url := fmt.Sprintf(
		"%s/workers",
		w.cfg.Manager.APIBase,
	)

	msg := WorkerStatus{
		ID:  w.Name(),
		URL: w.URL(),
	}

	if _, err := PostJSON(url, msg, w.tlsConfig); err != nil {
		logger.Error("Failed to register worker")
	}
}

func (w *Worker) updateStatus(jobMsg jobMessage) {
	url := fmt.Sprintf(
		"%s/workers/%s/jobs/%s",
		w.cfg.Manager.APIBase,
		w.Name(),
		jobMsg.name,
	)
	p := w.providers[jobMsg.name]
	smsg := MirrorStatus{
		Name:     jobMsg.name,
		Worker:   w.cfg.Global.Name,
		IsMaster: true,
		Status:   jobMsg.status,
		Upstream: p.Upstream(),
		Size:     "unknown",
		ErrorMsg: jobMsg.msg,
	}

	if _, err := PostJSON(url, smsg, w.tlsConfig); err != nil {
		logger.Error("Failed to update mirror(%s) status: %s", jobMsg.name, err.Error())
	}
}

func (w *Worker) fetchJobStatus() []MirrorStatus {
	var mirrorList []MirrorStatus

	url := fmt.Sprintf(
		"%s/workers/%s/jobs",
		w.cfg.Manager.APIBase,
		w.Name(),
	)

	if _, err := GetJSON(url, &mirrorList, w.tlsConfig); err != nil {
		logger.Error("Failed to fetch job status: %s", err.Error())
	}

	return mirrorList
}
