package manager

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	. "github.com/tuna/tunasync/internal"
)

const (
	_errorKey = "error"
	_infoKey  = "message"
)

var manager *Manager

// A Manager represents a manager server
type Manager struct {
	cfg        *Config
	engine     *gin.Engine
	adapter    dbAdapter
	httpClient *http.Client
}

// GetTUNASyncManager returns the manager from config
func GetTUNASyncManager(cfg *Config) *Manager {
	if manager != nil {
		return manager
	}

	// create gin engine
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Manager{
		cfg:     cfg,
		adapter: nil,
	}

	s.engine = gin.New()
	s.engine.Use(gin.Recovery())
	if cfg.Debug {
		s.engine.Use(gin.Logger())
	}

	if cfg.Files.CACert != "" {
		httpClient, err := CreateHTTPClient(cfg.Files.CACert)
		if err != nil {
			logger.Errorf("Error initializing HTTP client: %s", err.Error())
			return nil
		}
		s.httpClient = httpClient
	}

	if cfg.Files.DBFile != "" {
		adapter, err := makeDBAdapter(cfg.Files.DBType, cfg.Files.DBFile)
		if err != nil {
			logger.Errorf("Error initializing DB adapter: %s", err.Error())
			return nil
		}
		s.setDBAdapter(adapter)
	}

	// common log middleware
	s.engine.Use(contextErrorLogger)

	s.engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{_infoKey: "pong"})
	})
	// list jobs, status page
	s.engine.GET("/jobs", s.listAllJobs)
	// flush disabled jobs
	s.engine.DELETE("/jobs/disabled", s.flushDisabledJobs)

	// list workers
	s.engine.GET("/workers", s.listWorkers)
	// worker online
	s.engine.POST("/workers", s.registerWorker)

	// workerID should be valid in this route group
	workerValidateGroup := s.engine.Group("/workers", s.workerIDValidator)
	{
		// delete specified worker
		workerValidateGroup.DELETE(":id", s.deleteWorker)
		// get job list
		workerValidateGroup.GET(":id/jobs", s.listJobsOfWorker)
		// post job status
		workerValidateGroup.POST(":id/jobs/:job", s.updateJobOfWorker)
		workerValidateGroup.POST(":id/jobs/:job/size", s.updateMirrorSize)
	}

	// for tunasynctl to post commands
	s.engine.POST("/cmd", s.handleClientCmd)

	manager = s
	return s
}

func (s *Manager) setDBAdapter(adapter dbAdapter) {
	s.adapter = adapter
}

// Run runs the manager server forever
func (s *Manager) Run() {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Addr, s.cfg.Server.Port)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if s.cfg.Server.SSLCert == "" && s.cfg.Server.SSLKey == "" {
		if err := httpServer.ListenAndServe(); err != nil {
			panic(err)
		}
	} else {
		if err := httpServer.ListenAndServeTLS(s.cfg.Server.SSLCert, s.cfg.Server.SSLKey); err != nil {
			panic(err)
		}
	}
}

// listAllJobs repond with all jobs of specified workers
func (s *Manager) listAllJobs(c *gin.Context) {
	mirrorStatusList, err := s.adapter.ListAllMirrorStatus()
	if err != nil {
		err := fmt.Errorf("failed to list all mirror status: %s",
			err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	webMirStatusList := []WebMirrorStatus{}
	for _, m := range mirrorStatusList {
		webMirStatusList = append(
			webMirStatusList,
			BuildWebMirrorStatus(m),
		)
	}
	c.JSON(http.StatusOK, webMirStatusList)
}

// flushDisabledJobs deletes all jobs that marks as deleted
func (s *Manager) flushDisabledJobs(c *gin.Context) {
	err := s.adapter.FlushDisabledJobs()
	if err != nil {
		err := fmt.Errorf("failed to flush disabled jobs: %s",
			err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{_infoKey: "flushed"})
}

// deleteWorker deletes one worker by id
func (s *Manager) deleteWorker(c *gin.Context) {
	workerID := c.Param("id")
	err := s.adapter.DeleteWorker(workerID)
	if err != nil {
		err := fmt.Errorf("failed to delete worker: %s",
			err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	logger.Noticef("Worker <%s> deleted", workerID)
	c.JSON(http.StatusOK, gin.H{_infoKey: "deleted"})
}

// listWrokers respond with informations of all the workers
func (s *Manager) listWorkers(c *gin.Context) {
	var workerInfos []WorkerStatus
	workers, err := s.adapter.ListWorkers()
	if err != nil {
		err := fmt.Errorf("failed to list workers: %s",
			err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	for _, w := range workers {
		workerInfos = append(workerInfos,
			WorkerStatus{
				ID:         w.ID,
				LastOnline: w.LastOnline,
			})
	}
	c.JSON(http.StatusOK, workerInfos)
}

// registerWorker register an newly-online worker
func (s *Manager) registerWorker(c *gin.Context) {
	var _worker WorkerStatus
	c.BindJSON(&_worker)
	_worker.LastOnline = time.Now()
	newWorker, err := s.adapter.CreateWorker(_worker)
	if err != nil {
		err := fmt.Errorf("failed to register worker: %s",
			err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}

	logger.Noticef("Worker <%s> registered", _worker.ID)
	// create workerCmd channel for this worker
	c.JSON(http.StatusOK, newWorker)
}

// listJobsOfWorker respond with all the jobs of the specified worker
func (s *Manager) listJobsOfWorker(c *gin.Context) {
	workerID := c.Param("id")
	mirrorStatusList, err := s.adapter.ListMirrorStatus(workerID)
	if err != nil {
		err := fmt.Errorf("failed to list jobs of worker %s: %s",
			workerID, err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, mirrorStatusList)
}

func (s *Manager) returnErrJSON(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{
		_errorKey: err.Error(),
	})
}

func (s *Manager) updateJobOfWorker(c *gin.Context) {
	workerID := c.Param("id")
	var status MirrorStatus
	c.BindJSON(&status)
	mirrorName := status.Name
	if len(mirrorName) == 0 {
		s.returnErrJSON(
			c, http.StatusBadRequest,
			errors.New("Mirror Name should not be empty"),
		)
	}

	curStatus, _ := s.adapter.GetMirrorStatus(workerID, mirrorName)

	// Only successful syncing needs last_update
	if status.Status == Success {
		status.LastUpdate = time.Now()
	} else {
		status.LastUpdate = curStatus.LastUpdate
	}
	if status.Status == Success || status.Status == Failed {
		status.LastEnded = time.Now()
	} else {
		status.LastEnded = curStatus.LastEnded
	}

	// Only message with meaningful size updates the mirror size
	if len(curStatus.Size) > 0 && curStatus.Size != "unknown" {
		if len(status.Size) == 0 || status.Size == "unknown" {
			status.Size = curStatus.Size
		}
	}

	// for logging
	switch status.Status {
	case Syncing:
		logger.Noticef("Job [%s] @<%s> starts syncing", status.Name, status.Worker)
	default:
		logger.Noticef("Job [%s] @<%s> %s", status.Name, status.Worker, status.Status)
	}

	newStatus, err := s.adapter.UpdateMirrorStatus(workerID, mirrorName, status)
	if err != nil {
		err := fmt.Errorf("failed to update job %s of worker %s: %s",
			mirrorName, workerID, err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, newStatus)
}

func (s *Manager) updateMirrorSize(c *gin.Context) {
	workerID := c.Param("id")
	type SizeMsg struct {
		Name string `json:"name"`
		Size string `json:"size"`
	}
	var msg SizeMsg
	c.BindJSON(&msg)

	mirrorName := msg.Name
	status, err := s.adapter.GetMirrorStatus(workerID, mirrorName)
	if err != nil {
		logger.Errorf(
			"Failed to get status of mirror %s @<%s>: %s",
			mirrorName, workerID, err.Error(),
		)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}

	// Only message with meaningful size updates the mirror size
	if len(msg.Size) > 0 || msg.Size != "unknown" {
		status.Size = msg.Size
	}

	logger.Noticef("Mirror size of [%s] @<%s>: %s", status.Name, status.Worker, status.Size)

	newStatus, err := s.adapter.UpdateMirrorStatus(workerID, mirrorName, status)
	if err != nil {
		err := fmt.Errorf("failed to update job %s of worker %s: %s",
			mirrorName, workerID, err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, newStatus)
}

func (s *Manager) handleClientCmd(c *gin.Context) {
	var clientCmd ClientCmd
	c.BindJSON(&clientCmd)
	workerID := clientCmd.WorkerID
	if workerID == "" {
		// TODO: decide which worker should do this mirror when WorkerID is null string
		logger.Errorf("handleClientCmd case workerID == \" \" not implemented yet")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	w, err := s.adapter.GetWorker(workerID)
	if err != nil {
		err := fmt.Errorf("worker %s is not registered yet", workerID)
		s.returnErrJSON(c, http.StatusBadRequest, err)
		return
	}
	workerURL := w.URL
	// parse client cmd into worker cmd
	workerCmd := WorkerCmd{
		Cmd:      clientCmd.Cmd,
		MirrorID: clientCmd.MirrorID,
		Args:     clientCmd.Args,
		Options:  clientCmd.Options,
	}

	// update job status, even if the job did not disable successfully,
	// this status should be set as disabled
	curStat, _ := s.adapter.GetMirrorStatus(clientCmd.WorkerID, clientCmd.MirrorID)
	changed := false
	switch clientCmd.Cmd {
	case CmdDisable:
		curStat.Status = Disabled
		changed = true
	case CmdStop:
		curStat.Status = Paused
		changed = true
	}
	if changed {
		s.adapter.UpdateMirrorStatus(clientCmd.WorkerID, clientCmd.MirrorID, curStat)
	}

	logger.Noticef("Posting command '%s %s' to <%s>", clientCmd.Cmd, clientCmd.MirrorID, clientCmd.WorkerID)
	// post command to worker
	_, err = PostJSON(workerURL, workerCmd, s.httpClient)
	if err != nil {
		err := fmt.Errorf("post command to worker %s(%s) fail: %s", workerID, workerURL, err.Error())
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	// TODO: check response for success
	c.JSON(http.StatusOK, gin.H{_infoKey: "successfully send command to worker " + workerID})
}
