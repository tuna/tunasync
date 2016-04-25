package manager

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	. "github.com/tuna/tunasync/internal"
)

const (
	maxQueuedCmdNum = 3
	cmdPollTime     = 10 * time.Second
)

const (
	_errorKey = "error"
	_infoKey  = "message"
)

type worker struct {
	ID         string    `json:"id"`          // worker name
	Token      string    `json:"token"`       // session token
	LastOnline time.Time `json:"last_online"` // last seen
}

var (
	workerChannelMu sync.RWMutex
	workerChannels  = make(map[string]chan WorkerCmd)
)

type managerServer struct {
	*gin.Engine
	adapter dbAdapter
}

// listAllJobs repond with all jobs of specified workers
func (s *managerServer) listAllJobs(c *gin.Context) {
	mirrorStatusList, err := s.adapter.ListAllMirrorStatus()
	if err != nil {
		err := fmt.Errorf("failed to list all mirror status: %s",
			err.Error(),
		)
		c.Error(err)
		s.returnErrJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, mirrorStatusList)
}

// listWrokers respond with informations of all the workers
func (s *managerServer) listWorkers(c *gin.Context) {
	var workerInfos []WorkerInfoMsg
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
			WorkerInfoMsg{w.ID, w.LastOnline})
	}
	c.JSON(http.StatusOK, workerInfos)
}

// registerWorker register an newly-online worker
func (s *managerServer) registerWorker(c *gin.Context) {
	var _worker worker
	c.BindJSON(&_worker)
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
	workerChannelMu.Lock()
	defer workerChannelMu.Unlock()
	workerChannels[_worker.ID] = make(chan WorkerCmd, maxQueuedCmdNum)
	c.JSON(http.StatusOK, newWorker)
}

// listJobsOfWorker respond with all the jobs of the specified worker
func (s *managerServer) listJobsOfWorker(c *gin.Context) {
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

func (s *managerServer) returnErrJSON(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{
		_errorKey: err.Error(),
	})
}

func (s *managerServer) updateJobOfWorker(c *gin.Context) {
	workerID := c.Param("id")
	var status mirrorStatus
	c.BindJSON(&status)
	mirrorName := status.Name
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

func (s *managerServer) handleClientCmd(c *gin.Context) {
	workerChannelMu.RLock()
	defer workerChannelMu.RUnlock()
	var clientCmd ClientCmd
	c.BindJSON(&clientCmd)
	// TODO: decide which worker should do this mirror when WorkerID is null string
	workerID := clientCmd.WorkerID
	if workerID == "" {
		// TODO: decide which worker should do this mirror when WorkerID is null string
		logger.Error("handleClientCmd case workerID == \" \" not implemented yet")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	workerChannel, ok := workerChannels[workerID]
	if !ok {
		err := fmt.Errorf("worker %s is not registered yet", workerID)
		s.returnErrJSON(c, http.StatusBadRequest, err)
		return
	}
	// parse client cmd into worker cmd
	workerCmd := WorkerCmd{
		Cmd:      clientCmd.Cmd,
		MirrorID: clientCmd.MirrorID,
		Args:     clientCmd.Args,
	}
	select {
	case workerChannel <- workerCmd:
		// successfully insert command to channel
		c.JSON(http.StatusOK, struct{}{})
	default:
		// pending commands for that worker exceed
		// the maxQueuedCmdNum threshold
		err := fmt.Errorf("pending commands for worker %s exceed"+
			"the %d threshold, the command is dropped",
			workerID, maxQueuedCmdNum)
		c.Error(err)
		s.returnErrJSON(c, http.StatusServiceUnavailable, err)
		return
	}
}

func (s *managerServer) getCmdOfWorker(c *gin.Context) {
	workerID := c.Param("id")
	workerChannelMu.RLock()
	defer workerChannelMu.RUnlock()

	workerChannel := workerChannels[workerID]
	for {
		select {
		case _ = <-workerChannel:
			// TODO: push new command to worker client
			continue
		case <-time.After(cmdPollTime):
			// time limit exceeded, close the connection
			break
		}
	}
}

func (s *managerServer) setDBAdapter(adapter dbAdapter) {
	s.adapter = adapter
}

func makeHTTPServer(debug bool) *managerServer {
	// create gin engine
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &managerServer{
		gin.Default(),
		nil,
	}

	// common log middleware
	s.Use(contextErrorLogger)

	s.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{_infoKey: "pong"})
	})
	// list jobs, status page
	s.GET("/jobs", s.listAllJobs)

	// list workers
	s.GET("/workers", s.listWorkers)
	// worker online
	s.POST("/workers", s.registerWorker)

	// workerID should be valid in this route group
	workerValidateGroup := s.Group("/workers", s.workerIDValidator)
	// get job list
	workerValidateGroup.GET(":id/jobs", s.listJobsOfWorker)
	// post job status
	workerValidateGroup.POST(":id/jobs/:job", s.updateJobOfWorker)

	// worker command polling
	workerValidateGroup.GET(":id/cmd_stream", s.getCmdOfWorker)

	// for tunasynctl to post commands
	s.POST("/cmd/", s.handleClientCmd)

	return s
}
