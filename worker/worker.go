package worker

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/tuna/tunasync/internal"
)

var tunasyncWorker *Worker

// A Worker is a instance of tunasync worker
type Worker struct {
	L    sync.Mutex
	cfg  *Config
	jobs map[string]*mirrorJob

	managerChan chan jobMessage
	semaphore   chan empty
	exit        chan empty

	schedule   *scheduleQueue
	httpEngine *gin.Engine
	httpClient *http.Client
}

// GetTUNASyncWorker returns a singalton worker
func GetTUNASyncWorker(cfg *Config) *Worker {
	if tunasyncWorker != nil {
		return tunasyncWorker
	}

	w := &Worker{
		cfg:  cfg,
		jobs: make(map[string]*mirrorJob),

		managerChan: make(chan jobMessage, 32),
		semaphore:   make(chan empty, cfg.Global.Concurrent),
		exit:        make(chan empty),

		schedule: newScheduleQueue(),
	}

	if cfg.Manager.CACert != "" {
		httpClient, err := CreateHTTPClient(cfg.Manager.CACert)
		if err != nil {
			logger.Errorf("Error initializing HTTP client: %s", err.Error())
			return nil
		}
		w.httpClient = httpClient
	}

	w.initJobs()
	w.makeHTTPServer()
	tunasyncWorker = w
	return w
}

// Run runs worker forever
func (w *Worker) Run() {
	w.registorWorker()
	go w.runHTTPServer()
	w.runSchedule()
}

// Halt stops all jobs
func (w *Worker) Halt() {
	w.L.Lock()
	logger.Notice("Stopping all the jobs")
	for _, job := range w.jobs {
		if job.State() != stateDisabled {
			job.ctrlChan <- jobHalt
		}
	}
	jobsDone.Wait()
	logger.Notice("All the jobs are stopped")
	w.L.Unlock()
	close(w.exit)
}

// ReloadMirrorConfig refresh the providers and jobs
// from new mirror configs
// TODO: deleted job should be removed from manager-side mirror list
func (w *Worker) ReloadMirrorConfig(newMirrors []mirrorConfig) {
	w.L.Lock()
	defer w.L.Unlock()
	logger.Info("Reloading mirror configs")

	oldMirrors := w.cfg.Mirrors
	difference := diffMirrorConfig(oldMirrors, newMirrors)

	// first deal with deletion and modifications
	for _, op := range difference {
		if op.diffOp == diffAdd {
			continue
		}
		name := op.mirCfg.Name
		job, ok := w.jobs[name]
		if !ok {
			logger.Warningf("Job %s not found", name)
			continue
		}
		switch op.diffOp {
		case diffDelete:
			w.disableJob(job)
			delete(w.jobs, name)
			logger.Noticef("Deleted job %s", name)
		case diffModify:
			jobState := job.State()
			w.disableJob(job)
			// set new provider
			provider := newMirrorProvider(op.mirCfg, w.cfg)
			if err := job.SetProvider(provider); err != nil {
				logger.Errorf("Error setting job provider of %s: %s", name, err.Error())
				continue
			}

			// re-schedule job according to its previous state
			if jobState == stateDisabled {
				job.SetState(stateDisabled)
			} else if jobState == statePaused {
				job.SetState(statePaused)
				go job.Run(w.managerChan, w.semaphore)
			} else {
				job.SetState(stateNone)
				go job.Run(w.managerChan, w.semaphore)
				w.schedule.AddJob(time.Now(), job)
			}
			logger.Noticef("Reloaded job %s", name)
		}
	}
	// for added new jobs, just start new jobs
	for _, op := range difference {
		if op.diffOp != diffAdd {
			continue
		}
		provider := newMirrorProvider(op.mirCfg, w.cfg)
		job := newMirrorJob(provider)
		w.jobs[provider.Name()] = job

		job.SetState(stateNone)
		go job.Run(w.managerChan, w.semaphore)
		w.schedule.AddJob(time.Now(), job)
		logger.Noticef("New job %s", job.Name())
	}

	w.cfg.Mirrors = newMirrors
}

func (w *Worker) initJobs() {
	for _, mirror := range w.cfg.Mirrors {
		// Create Provider
		provider := newMirrorProvider(mirror, w.cfg)
		w.jobs[provider.Name()] = newMirrorJob(provider)
	}
}

func (w *Worker) disableJob(job *mirrorJob) {
	w.schedule.Remove(job.Name())
	if job.State() != stateDisabled {
		job.ctrlChan <- jobDisable
		<-job.disabled
	}
}

// Ctrl server receives commands from the manager
func (w *Worker) makeHTTPServer() {
	s := gin.New()
	s.Use(gin.Recovery())

	s.POST("/", func(c *gin.Context) {
		w.L.Lock()
		defer w.L.Unlock()

		var cmd WorkerCmd

		if err := c.BindJSON(&cmd); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Invalid request"})
			return
		}

		logger.Noticef("Received command: %v", cmd)

		if cmd.MirrorID == "" {
			// worker-level commands
			switch cmd.Cmd {
			case CmdReload:
				// send myself a SIGHUP
				pid := os.Getpid()
				syscall.Kill(pid, syscall.SIGHUP)
			default:
				c.JSON(http.StatusNotAcceptable, gin.H{"msg": "Invalid Command"})
				return
			}
		}

		// job level comands
		job, ok := w.jobs[cmd.MirrorID]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"msg": fmt.Sprintf("Mirror ``%s'' not found", cmd.MirrorID)})
			return
		}

		// No matter what command, the existing job
		// schedule should be flushed
		w.schedule.Remove(job.Name())

		// if job disabled, start them first
		switch cmd.Cmd {
		case CmdStart, CmdRestart:
			if job.State() == stateDisabled {
				go job.Run(w.managerChan, w.semaphore)
			}
		}
		switch cmd.Cmd {
		case CmdStart:
			if cmd.Options["force"] {
				job.ctrlChan <- jobForceStart
			} else {
				job.ctrlChan <- jobStart
			}
		case CmdRestart:
			job.ctrlChan <- jobRestart
		case CmdStop:
			// if job is disabled, no goroutine would be there
			// receiving this signal
			if job.State() != stateDisabled {
				job.ctrlChan <- jobStop
			}
		case CmdDisable:
			w.disableJob(job)
		case CmdPing:
			// empty
		default:
			c.JSON(http.StatusNotAcceptable, gin.H{"msg": "Invalid Command"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "OK"})
	})
	w.httpEngine = s
}

func (w *Worker) runHTTPServer() {
	addr := fmt.Sprintf("%s:%d", w.cfg.Server.Addr, w.cfg.Server.Port)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      w.httpEngine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if w.cfg.Server.SSLCert == "" && w.cfg.Server.SSLKey == "" {
		if err := httpServer.ListenAndServe(); err != nil {
			panic(err)
		}
	} else {
		if err := httpServer.ListenAndServeTLS(w.cfg.Server.SSLCert, w.cfg.Server.SSLKey); err != nil {
			panic(err)
		}
	}
}

func (w *Worker) runSchedule() {
	w.L.Lock()

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
			case Disabled:
				job.SetState(stateDisabled)
				continue
			case Paused:
				job.SetState(statePaused)
				go job.Run(w.managerChan, w.semaphore)
				continue
			default:
				job.SetState(stateNone)
				go job.Run(w.managerChan, w.semaphore)
				stime := m.LastUpdate.Add(job.provider.Interval())
				logger.Debugf("Scheduling job %s @%s", job.Name(), stime.Format("2006-01-02 15:04:05"))
				w.schedule.AddJob(stime, job)
			}
		}
	}
	// some new jobs may be added
	// which does not exist in the
	// manager's mirror list
	for name := range unset {
		job := w.jobs[name]
		job.SetState(stateNone)
		go job.Run(w.managerChan, w.semaphore)
		w.schedule.AddJob(time.Now(), job)
	}

	w.L.Unlock()

	tick := time.Tick(5 * time.Second)
	for {
		select {
		case jobMsg := <-w.managerChan:
			// got status update from job
			w.L.Lock()
			job, ok := w.jobs[jobMsg.name]
			w.L.Unlock()
			if !ok {
				logger.Warningf("Job %s not found", jobMsg.name)
				continue
			}

			if (job.State() != stateReady) && (job.State() != stateHalting) {
				logger.Infof("Job %s state is not ready, skip adding new schedule", jobMsg.name)
				continue
			}

			// syncing status is only meaningful when job
			// is running. If it's paused or disabled
			// a sync failure signal would be emitted
			// which needs to be ignored
			w.updateStatus(job, jobMsg)

			// only successful or the final failure msg
			// can trigger scheduling
			if jobMsg.schedule {
				schedTime := time.Now().Add(job.provider.Interval())
				logger.Noticef(
					"Next scheduled time for %s: %s",
					job.Name(),
					schedTime.Format("2006-01-02 15:04:05"),
				)
				w.schedule.AddJob(schedTime, job)
			}

		case <-tick:
			// check schedule every 5 seconds
			if job := w.schedule.Pop(); job != nil {
				job.ctrlChan <- jobStart
			}
		case <-w.exit:
			// flush status update messages
			w.L.Lock()
			defer w.L.Unlock()
			for {
				select {
				case jobMsg := <-w.managerChan:
					logger.Debugf("status update from %s", jobMsg.name)
					job, ok := w.jobs[jobMsg.name]
					if !ok {
						continue
					}
					if jobMsg.status == Failed || jobMsg.status == Success {
						w.updateStatus(job, jobMsg)
					}
				default:
					return
				}
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
	msg := WorkerStatus{
		ID:  w.Name(),
		URL: w.URL(),
	}

	for _, root := range w.cfg.Manager.APIBaseList() {
		url := fmt.Sprintf("%s/workers", root)
		logger.Debugf("register on manager url: %s", url)
		if _, err := PostJSON(url, msg, w.httpClient); err != nil {
			logger.Errorf("Failed to register worker")
		}
	}
}

func (w *Worker) updateStatus(job *mirrorJob, jobMsg jobMessage) {
	p := job.provider
	smsg := MirrorStatus{
		Name:     jobMsg.name,
		Worker:   w.cfg.Global.Name,
		IsMaster: p.IsMaster(),
		Status:   jobMsg.status,
		Upstream: p.Upstream(),
		Size:     "unknown",
		ErrorMsg: jobMsg.msg,
	}

	for _, root := range w.cfg.Manager.APIBaseList() {
		url := fmt.Sprintf(
			"%s/workers/%s/jobs/%s", root, w.Name(), jobMsg.name,
		)
		logger.Debugf("reporting on manager url: %s", url)
		if _, err := PostJSON(url, smsg, w.httpClient); err != nil {
			logger.Errorf("Failed to update mirror(%s) status: %s", jobMsg.name, err.Error())
		}
	}
}

func (w *Worker) fetchJobStatus() []MirrorStatus {
	var mirrorList []MirrorStatus
	apiBase := w.cfg.Manager.APIBaseList()[0]

	url := fmt.Sprintf("%s/workers/%s/jobs", apiBase, w.Name())

	if _, err := GetJSON(url, &mirrorList, w.httpClient); err != nil {
		logger.Errorf("Failed to fetch job status: %s", err.Error())
	}

	return mirrorList
}
