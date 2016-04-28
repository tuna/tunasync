package manager

import (
	"crypto/tls"
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
	cfg       *Config
	engine    *gin.Engine
	adapter   dbAdapter
	tlsConfig *tls.Config
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
		cfg:       cfg,
		engine:    gin.Default(),
		adapter:   nil,
		tlsConfig: nil,
	}

	if cfg.Files.CACert != "" {
		tlsConfig, err := GetTLSConfig(cfg.Files.CACert)
		if err != nil {
			logger.Error("Error initializing TLS config: %s", err.Error())
			return nil
		}
		s.tlsConfig = tlsConfig
	}

	if cfg.Files.DBFile != "" {
		adapter, err := makeDBAdapter(cfg.Files.DBType, cfg.Files.DBFile)
		if err != nil {
			logger.Error("Error initializing DB adapter: %s", err.Error())
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

	// list workers
	s.engine.GET("/workers", s.listWorkers)
	// worker online
	s.engine.POST("/workers", s.registerWorker)

	// workerID should be valid in this route group
	workerValidateGroup := s.engine.Group("/workers", s.workerIDValidator)
	// get job list
	workerValidateGroup.GET(":id/jobs", s.listJobsOfWorker)
	// post job status
	workerValidateGroup.POST(":id/jobs/:job", s.updateJobOfWorker)

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
	if s.cfg.Server.SSLCert == "" && s.cfg.Server.SSLKey == "" {
		if err := s.engine.Run(addr); err != nil {
			panic(err)
		}
	} else {
		if err := s.engine.RunTLS(addr, s.cfg.Server.SSLCert, s.cfg.Server.SSLKey); err != nil {
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
	webMirStatusList := []webMirrorStatus{}
	for _, m := range mirrorStatusList {
		webMirStatusList = append(
			webMirStatusList,
			convertMirrorStatus(m),
		)
	}
	c.JSON(http.StatusOK, webMirStatusList)
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

	curStatus, _ := s.adapter.GetMirrorStatus(workerID, mirrorName)

	// Only successful syncing needs last_update
	if status.Status == Success {
		status.LastUpdate = time.Now()
	} else {
		status.LastUpdate = curStatus.LastUpdate
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

func (s *Manager) handleClientCmd(c *gin.Context) {
	var clientCmd ClientCmd
	c.BindJSON(&clientCmd)
	workerID := clientCmd.WorkerID
	if workerID == "" {
		// TODO: decide which worker should do this mirror when WorkerID is null string
		logger.Error("handleClientCmd case workerID == \" \" not implemented yet")
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

	// post command to worker
	_, err = PostJSON(workerURL, workerCmd, s.tlsConfig)
	if err != nil {
		err := fmt.Errorf("post command to worker %s(%s) fail: %s", workerID, workerURL, err.Error())
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	// TODO: check response for success
	c.JSON(http.StatusOK, gin.H{_infoKey: "successfully send command to worker " + workerID})
}
