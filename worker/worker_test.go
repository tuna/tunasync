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

func makeMockManagerServer(recvData chan interface{}) *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"_infoKey": "pong"})
	})
	r.POST("/workers", func(c *gin.Context) {
		var _worker WorkerStatus
		c.BindJSON(&_worker)
		_worker.LastOnline = time.Now()
		recvData <- _worker
		c.JSON(http.StatusOK, _worker)
	})
	r.POST("/workers/dut/schedules", func(c *gin.Context) {
		var _sch MirrorSchedules
		c.BindJSON(&_sch)
		recvData <- _sch
		c.JSON(http.StatusOK, empty{})
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

func TestWorker(t *testing.T) {
	managerPort := 5001
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

	Convey("Worker should work", t, func(ctx C) {

		workerCfg := Config{
			Global: globalConfig{
				Name:       "dut",
				LogDir:     "/tmp",
				MirrorDir:  "/tmp",
				Concurrent: 2,
				Interval:   1,
			},
			Manager: managerConfig{
				APIBase: "http://localhost:" + strconv.Itoa(managerPort),
			},
		}
		Convey("with no job", func(ctx C) {
			dummyTester := func(*Worker) {
				for {
					select {
					case data := <-recvDataChan:
						if reg, ok := data.(WorkerStatus); ok {
							So(reg.ID, ShouldEqual, "dut")
						} else if sch, ok := data.(MirrorSchedules); ok {
							So(len(sch.Schedules), ShouldEqual, 0)
						}
					case <-time.After(2 * time.Second):
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
				for {
					select {
					case data := <-recvDataChan:
						if reg, ok := data.(WorkerStatus); ok {
							So(reg.ID, ShouldEqual, "dut")
						} else if sch, ok := data.(MirrorSchedules); ok {
							So(len(sch.Schedules), ShouldEqual, 1)
							So(sch.Schedules[0].MirrorName, ShouldEqual, "job-ls")
							So(sch.Schedules[0].NextSchedule,
								ShouldHappenBetween,
								time.Now().Add(-2*time.Second),
								time.Now().Add(1*time.Minute))
						}
					case <-time.After(2 * time.Second):
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
				for {
					select {
					case data := <-recvDataChan:
						if reg, ok := data.(WorkerStatus); ok {
							So(reg.ID, ShouldEqual, "dut")
						} else if sch, ok := data.(MirrorSchedules); ok {
							So(len(sch.Schedules), ShouldEqual, 3)
						}
					case <-time.After(2 * time.Second):
						return
					}
				}
			}

			startWorkerThenStop(&workerCfg, dummyTester)
		})
	})
}
