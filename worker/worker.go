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

	mirrorStatus map[string]SyncStatus
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

		schedule:     newScheduleQueue(),
		mirrorStatus: make(map[string]SyncStatus),
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
			mirrorDir = c.Global.MirrorDir
		}
		logDir = formatLogDir(logDir, mirror)

		var provider mirrorProvider

		switch mirror.Provider {
		case ProvCommand:
			pc := cmdConfig{
				name:        mirror.Name,
				upstreamURL: mirror.Upstream,
				command:     mirror.Command,
				workingDir:  filepath.Join(mirrorDir, mirror.Name),
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
				password:    mirror.Password,
				excludeFile: mirror.ExcludeFile,
				workingDir:  filepath.Join(mirrorDir, mirror.Name),
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
				password:      mirror.Password,
				excludeFile:   mirror.ExcludeFile,
				workingDir:    filepath.Join(mirrorDir, mirror.Name),
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
		go w.jobs[name].Run(w.managerChan, w.semaphore)
		w.mirrorStatus[name] = Paused
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
		// if job disabled, start them first
		switch cmd.Cmd {
		case CmdStart, CmdRestart:
			if job.Disabled() {
				go job.Run(w.managerChan, w.semaphore)
			}
		}
		switch cmd.Cmd {
		case CmdStart:
			job.ctrlChan <- jobStart
		case CmdStop:
			job.ctrlChan <- jobStop
		case CmdRestart:
			job.ctrlChan <- jobRestart
		case CmdDisable:
			w.schedule.Remove(job.Name())
			job.ctrlChan <- jobDisable
			<-job.disabled
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
	for _, m := range mirrorList {
		if job, ok := w.jobs[m.Name]; ok {
			stime := m.LastUpdate.Add(job.provider.Interval())
			w.schedule.AddJob(stime, job)
			delete(unset, m.Name)
		}
	}
	for name := range unset {
		job := w.jobs[name]
		w.schedule.AddJob(time.Now(), job)
	}

	for {
		select {
		case jobMsg := <-w.managerChan:
			// got status update from job
			w.updateStatus(jobMsg)
			status := w.mirrorStatus[jobMsg.name]
			if status == Disabled || status == Paused {
				continue
			}
			w.mirrorStatus[jobMsg.name] = jobMsg.status
			switch jobMsg.status {
			case Success, Failed:
				job := w.jobs[jobMsg.name]
				w.schedule.AddJob(
					time.Now().Add(job.provider.Interval()),
					job,
				)
			}

		case <-time.Tick(10 * time.Second):
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

	msg := WorkerInfoMsg{
		ID:  w.Name(),
		URL: w.URL(),
	}

	if _, err := PostJSON(url, msg, w.tlsConfig); err != nil {
		logger.Error("Failed to register worker")
	}
}

func (w *Worker) updateStatus(jobMsg jobMessage) {
	url := fmt.Sprintf(
		"%s/%s/jobs/%s",
		w.cfg.Manager.APIBase,
		w.Name(),
		jobMsg.name,
	)
	p := w.providers[jobMsg.name]
	smsg := MirrorStatus{
		Name:       jobMsg.name,
		Worker:     w.cfg.Global.Name,
		IsMaster:   true,
		Status:     jobMsg.status,
		LastUpdate: time.Now(),
		Upstream:   p.Upstream(),
		Size:       "unknown",
		ErrorMsg:   jobMsg.msg,
	}

	if _, err := PostJSON(url, smsg, w.tlsConfig); err != nil {
		logger.Error("Failed to update mirror(%s) status: %s", jobMsg.name, err.Error())
	}
}

func (w *Worker) fetchJobStatus() []MirrorStatus {
	var mirrorList []MirrorStatus

	url := fmt.Sprintf(
		"%s/%s/jobs",
		w.cfg.Manager.APIBase,
		w.Name(),
	)

	if _, err := GetJSON(url, &mirrorList, w.tlsConfig); err != nil {
		logger.Error("Failed to fetch job status: %s", err.Error())
	}

	return mirrorList
}
