package worker

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

type workTestFunc func(*Worker)

var managerPort = 5001
var workerPort = 5002

func makeMockManagerServer(recvData chan interface{}) *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"_infoKey": "pong"})
	})
	r.POST("/workers", func(c *gin.Context) {
		var _worker WorkerStatus
		c.BindJSON(&_worker)
		_worker.LastOnline = time.Now()
		_worker.LastRegister = time.Now()
		recvData <- _worker
		c.JSON(http.StatusOK, _worker)
	})
	r.POST("/workers/dut/schedules", func(c *gin.Context) {
		var _sch MirrorSchedules
		c.BindJSON(&_sch)
		recvData <- _sch
		c.JSON(http.StatusOK, empty{})
	})
	r.POST("/workers/dut/jobs/:job", func(c *gin.Context) {
		var status MirrorStatus
		c.BindJSON(&status)
		recvData <- status
		c.JSON(http.StatusOK, status)
	})
	r.GET("/workers/dut/jobs", func(c *gin.Context) {
		mirrorStatusList := []MirrorStatus{}
		c.JSON(http.StatusOK, mirrorStatusList)
	})

	return r
}

func startWorkerThenStop(cfg *Config, tester workTestFunc) {
	exitedChan := make(chan int)
	w := NewTUNASyncWorker(cfg)
	So(w, ShouldNotBeNil)
	go func() {
		w.Run()
		exitedChan <- 1
	}()

	tester(w)

	w.Halt()
	select {
	case exited := <-exitedChan:
		So(exited, ShouldEqual, 1)
	case <-time.After(2 * time.Second):
		So(0, ShouldEqual, 1)
	}

}
func sendCommandToWorker(workerURL string, httpClient *http.Client, cmd CmdVerb, mirror string) {
	workerCmd := WorkerCmd{
		Cmd:      cmd,
		MirrorID: mirror,
	}
	logger.Debugf("POST to %s with cmd %s", workerURL, cmd)
	_, err := PostJSON(workerURL, workerCmd, httpClient)
	So(err, ShouldBeNil)
}

func TestWorker(t *testing.T) {
	InitLogger(false, true, false)

	recvDataChan := make(chan interface{})
	_s := makeMockManagerServer(recvDataChan)
	httpServer := &http.Server{
		Addr:         "localhost:" + strconv.Itoa(managerPort),
		Handler:      _s,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}
	go func() {
		err := httpServer.ListenAndServe()
		So(err, ShouldBeNil)
	}()
	// Wait for http server starting
	time.Sleep(500 * time.Millisecond)

	Convey("Worker should work", t, func(ctx C) {

		httpClient, err := CreateHTTPClient("")
		So(err, ShouldBeNil)

		workerPort++

		workerCfg := Config{
			Global: globalConfig{
				Name:       "dut",
				LogDir:     "/tmp",
				MirrorDir:  "/tmp",
				Concurrent: 2,
				Interval:   1,
			},
			Server: serverConfig{
				Hostname: "localhost",
				Addr:     "127.0.0.1",
				Port:     workerPort,
			},
			Manager: managerConfig{
				APIBase: "http://localhost:" + strconv.Itoa(managerPort),
			},
		}
		logger.Debugf("worker port %d", workerPort)
		Convey("with no job", func(ctx C) {
			dummyTester := func(*Worker) {
				registered := false
				for {
					select {
					case data := <-recvDataChan:
						if reg, ok := data.(WorkerStatus); ok {
							So(reg.ID, ShouldEqual, "dut")
							registered = true
							time.Sleep(500 * time.Millisecond)
							sendCommandToWorker(reg.URL, httpClient, CmdStart, "foobar")
						} else if sch, ok := data.(MirrorSchedules); ok {
							So(len(sch.Schedules), ShouldEqual, 0)
						}
					case <-time.After(2 * time.Second):
						So(registered, ShouldBeTrue)
						return
					}
				}
			}

			startWorkerThenStop(&workerCfg, dummyTester)
		})
		Convey("with one job", func(ctx C) {
			workerCfg.Mirrors = []mirrorConfig{
				mirrorConfig{
					Name:     "job-ls",
					Provider: provCommand,
					Command:  "ls",
				},
			}

			dummyTester := func(*Worker) {
				url := ""
				jobRunning := false
				lastStatus := SyncStatus(None)
				for {
					select {
					case data := <-recvDataChan:
						if reg, ok := data.(WorkerStatus); ok {
							So(reg.ID, ShouldEqual, "dut")
							url = reg.URL
							time.Sleep(500 * time.Millisecond)
							sendCommandToWorker(url, httpClient, CmdStart, "job-ls")
						} else if sch, ok := data.(MirrorSchedules); ok {
							if !jobRunning {
								So(len(sch.Schedules), ShouldEqual, 1)
								So(sch.Schedules[0].MirrorName, ShouldEqual, "job-ls")
								So(sch.Schedules[0].NextSchedule,
									ShouldHappenBetween,
									time.Now().Add(-2*time.Second),
									time.Now().Add(1*time.Minute))
							}
						} else if status, ok := data.(MirrorStatus); ok {
							logger.Noticef("Job %s status %s", status.Name, status.Status.String())
							jobRunning = status.Status == PreSyncing || status.Status == Syncing
							So(status.Status, ShouldNotEqual, Failed)
							lastStatus = status.Status
						}
					case <-time.After(2 * time.Second):
						So(url, ShouldNotEqual, "")
						So(jobRunning, ShouldBeFalse)
						So(lastStatus, ShouldEqual, Success)
						return
					}
				}
			}

			startWorkerThenStop(&workerCfg, dummyTester)
		})
		Convey("with several jobs", func(ctx C) {
			workerCfg.Mirrors = []mirrorConfig{
				mirrorConfig{
					Name:     "job-ls-1",
					Provider: provCommand,
					Command:  "ls",
				},
				mirrorConfig{
					Name:     "job-fail",
					Provider: provCommand,
					Command:  "non-existent-command-xxxx",
				},
				mirrorConfig{
					Name:     "job-ls-2",
					Provider: provCommand,
					Command:  "ls",
				},
			}

			dummyTester := func(*Worker) {
				url := ""
				lastStatus := make(map[string]SyncStatus)
				nextSch := make(map[string]time.Time)
				for {
					select {
					case data := <-recvDataChan:
						if reg, ok := data.(WorkerStatus); ok {
							So(reg.ID, ShouldEqual, "dut")
							url = reg.URL
							time.Sleep(500 * time.Millisecond)
							sendCommandToWorker(url, httpClient, CmdStart, "job-fail")
							sendCommandToWorker(url, httpClient, CmdStart, "job-ls-1")
							sendCommandToWorker(url, httpClient, CmdStart, "job-ls-2")
						} else if sch, ok := data.(MirrorSchedules); ok {
							//So(len(sch.Schedules), ShouldEqual, 3)
							for _, item := range sch.Schedules {
								nextSch[item.MirrorName] = item.NextSchedule
							}
						} else if status, ok := data.(MirrorStatus); ok {
							logger.Noticef("Job %s status %s", status.Name, status.Status.String())
							jobRunning := status.Status == PreSyncing || status.Status == Syncing
							if !jobRunning {
								if status.Name == "job-fail" {
									So(status.Status, ShouldEqual, Failed)
								} else {
									So(status.Status, ShouldNotEqual, Failed)
								}
							}
							lastStatus[status.Name] = status.Status
						}
					case <-time.After(2 * time.Second):
						So(len(lastStatus), ShouldEqual, 3)
						So(len(nextSch), ShouldEqual, 3)
						return
					}
				}
			}

			startWorkerThenStop(&workerCfg, dummyTester)
		})
	})
}
