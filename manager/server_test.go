package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

const (
	_magicBadWorkerID = "magic_bad_worker_id"
)

func TestHTTPServer(t *testing.T) {
	var listenPort = 5000
	Convey("HTTP server should work", t, func(ctx C) {
		listenPort++
		port := listenPort
		addr := "127.0.0.1"
		baseURL := fmt.Sprintf("http://%s:%d", addr, port)
		InitLogger(true, true, false)
		s := GetTUNASyncManager(&Config{Debug: true})
		s.cfg.Server.Addr = addr
		s.cfg.Server.Port = port
		So(s, ShouldNotBeNil)
		s.setDBAdapter(&mockDBAdapter{
			workerStore: map[string]WorkerStatus{
				_magicBadWorkerID: {
					ID: _magicBadWorkerID,
				}},
			statusStore: make(map[string]MirrorStatus),
		})
		go s.Run()
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(baseURL + "/ping")
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		So(resp.Header.Get("Content-Type"), ShouldEqual, "application/json; charset=utf-8")
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		var p map[string]string
		err = json.Unmarshal(body, &p)
		So(err, ShouldBeNil)
		So(p[_infoKey], ShouldEqual, "pong")

		Convey("when database fail", func(ctx C) {
			resp, err := http.Get(fmt.Sprintf("%s/workers/%s/jobs", baseURL, _magicBadWorkerID))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
			defer resp.Body.Close()
			var msg map[string]string
			err = json.NewDecoder(resp.Body).Decode(&msg)
			So(err, ShouldBeNil)
			So(msg[_errorKey], ShouldEqual, fmt.Sprintf("failed to list jobs of worker %s: %s", _magicBadWorkerID, "database fail"))
		})

		Convey("when register multiple workers", func(ctx C) {
			N := 10
			var cnt uint32
			for i := 0; i < N; i++ {
				go func(id int) {
					w := WorkerStatus{
						ID: fmt.Sprintf("worker%d", id),
					}
					resp, err := PostJSON(baseURL+"/workers", w, nil)
					ctx.So(err, ShouldBeNil)
					ctx.So(resp.StatusCode, ShouldEqual, http.StatusOK)
					atomic.AddUint32(&cnt, 1)
				}(i)
			}
			time.Sleep(2 * time.Second)
			So(cnt, ShouldEqual, N)

			Convey("list all workers", func(ctx C) {
				resp, err := http.Get(baseURL + "/workers")
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				var actualResponseObj []WorkerStatus
				err = json.NewDecoder(resp.Body).Decode(&actualResponseObj)
				So(err, ShouldBeNil)
				So(len(actualResponseObj), ShouldEqual, N+1)
			})
		})

		Convey("when register a worker", func(ctx C) {
			w := WorkerStatus{
				ID: "test_worker1",
			}
			resp, err := PostJSON(baseURL+"/workers", w, nil)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)

			Convey("list all workers", func(ctx C) {
				resp, err := http.Get(baseURL + "/workers")
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				var actualResponseObj []WorkerStatus
				err = json.NewDecoder(resp.Body).Decode(&actualResponseObj)
				So(err, ShouldBeNil)
				So(len(actualResponseObj), ShouldEqual, 2)
			})

			Convey("delete an existent worker", func(ctx C) {
				req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/workers/%s", baseURL, w.ID), nil)
				So(err, ShouldBeNil)
				clt := &http.Client{}
				resp, err := clt.Do(req)
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				res := map[string]string{}
				err = json.NewDecoder(resp.Body).Decode(&res)
				So(err, ShouldBeNil)
				So(res[_infoKey], ShouldEqual, "deleted")
			})

			Convey("delete non-existent worker", func(ctx C) {
				invalidWorker := "test_worker233"
				req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/workers/%s", baseURL, invalidWorker), nil)
				So(err, ShouldBeNil)
				clt := &http.Client{}
				resp, err := clt.Do(req)
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				res := map[string]string{}
				err = json.NewDecoder(resp.Body).Decode(&res)
				So(err, ShouldBeNil)
				So(res[_errorKey], ShouldEqual, "invalid workerID "+invalidWorker)
			})

			Convey("flush disabled jobs", func(ctx C) {
				req, err := http.NewRequest("DELETE", baseURL+"/jobs/disabled", nil)
				So(err, ShouldBeNil)
				clt := &http.Client{}
				resp, err := clt.Do(req)
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				res := map[string]string{}
				err = json.NewDecoder(resp.Body).Decode(&res)
				So(err, ShouldBeNil)
				So(res[_infoKey], ShouldEqual, "flushed")
			})

			Convey("update mirror status of a existed worker", func(ctx C) {
				status := MirrorStatus{
					Name:     "arch-sync1",
					Worker:   "test_worker1",
					IsMaster: true,
					Status:   Success,
					Upstream: "mirrors.tuna.tsinghua.edu.cn",
					Size:     "unknown",
				}
				resp, err := PostJSON(fmt.Sprintf("%s/workers/%s/jobs/%s", baseURL, status.Worker, status.Name), status, nil)
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				So(resp.StatusCode, ShouldEqual, http.StatusOK)

				Convey("list mirror status of an existed worker", func(ctx C) {
					var ms []MirrorStatus
					resp, err := GetJSON(baseURL+"/workers/test_worker1/jobs", &ms, nil)

					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					// err = json.NewDecoder(resp.Body).Decode(&mirrorStatusList)
					m := ms[0]
					So(m.Name, ShouldEqual, status.Name)
					So(m.Worker, ShouldEqual, status.Worker)
					So(m.Status, ShouldEqual, status.Status)
					So(m.Upstream, ShouldEqual, status.Upstream)
					So(m.Size, ShouldEqual, status.Size)
					So(m.IsMaster, ShouldEqual, status.IsMaster)
					So(time.Since(m.LastUpdate), ShouldBeLessThan, 1*time.Second)
					So(m.LastStarted.IsZero(), ShouldBeTrue) // hasn't been initialized yet
					So(time.Since(m.LastEnded), ShouldBeLessThan, 1*time.Second)

				})

				// start syncing
				status.Status = PreSyncing
				time.Sleep(1 * time.Second)
				resp, err = PostJSON(fmt.Sprintf("%s/workers/%s/jobs/%s", baseURL, status.Worker, status.Name), status, nil)
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				So(resp.StatusCode, ShouldEqual, http.StatusOK)

				Convey("update mirror status to PreSync - starting sync", func(ctx C) {
					var ms []MirrorStatus
					resp, err := GetJSON(baseURL+"/workers/test_worker1/jobs", &ms, nil)

					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					// err = json.NewDecoder(resp.Body).Decode(&mirrorStatusList)
					m := ms[0]
					So(m.Name, ShouldEqual, status.Name)
					So(m.Worker, ShouldEqual, status.Worker)
					So(m.Status, ShouldEqual, status.Status)
					So(m.Upstream, ShouldEqual, status.Upstream)
					So(m.Size, ShouldEqual, status.Size)
					So(m.IsMaster, ShouldEqual, status.IsMaster)
					So(time.Since(m.LastUpdate), ShouldBeLessThan, 3*time.Second)
					So(time.Since(m.LastUpdate), ShouldBeGreaterThan, 1*time.Second)
					So(time.Since(m.LastStarted), ShouldBeLessThan, 2*time.Second)
					So(time.Since(m.LastEnded), ShouldBeLessThan, 3*time.Second)
					So(time.Since(m.LastEnded), ShouldBeGreaterThan, 1*time.Second)

				})

				Convey("list all job status of all workers", func(ctx C) {
					var ms []WebMirrorStatus
					resp, err := GetJSON(baseURL+"/jobs", &ms, nil)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)

					m := ms[0]
					So(m.Name, ShouldEqual, status.Name)
					So(m.Status, ShouldEqual, status.Status)
					So(m.Upstream, ShouldEqual, status.Upstream)
					So(m.Size, ShouldEqual, status.Size)
					So(m.IsMaster, ShouldEqual, status.IsMaster)
					So(time.Since(m.LastUpdate.Time), ShouldBeLessThan, 3*time.Second)
					So(time.Since(m.LastStarted.Time), ShouldBeLessThan, 2*time.Second)
					So(time.Since(m.LastEnded.Time), ShouldBeLessThan, 3*time.Second)

				})

				Convey("Update size of a valid mirror", func(ctx C) {
					msg := struct {
						Name string `json:"name"`
						Size string `json:"size"`
					}{status.Name, "5GB"}

					url := fmt.Sprintf("%s/workers/%s/jobs/%s/size", baseURL, status.Worker, status.Name)
					resp, err := PostJSON(url, msg, nil)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)

					Convey("Get new size of a mirror", func(ctx C) {
						var ms []MirrorStatus
						resp, err := GetJSON(baseURL+"/workers/test_worker1/jobs", &ms, nil)

						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusOK)
						// err = json.NewDecoder(resp.Body).Decode(&mirrorStatusList)
						m := ms[0]
						So(m.Name, ShouldEqual, status.Name)
						So(m.Worker, ShouldEqual, status.Worker)
						So(m.Status, ShouldEqual, status.Status)
						So(m.Upstream, ShouldEqual, status.Upstream)
						So(m.Size, ShouldEqual, "5GB")
						So(m.IsMaster, ShouldEqual, status.IsMaster)
						So(time.Since(m.LastUpdate), ShouldBeLessThan, 3*time.Second)
						So(time.Since(m.LastStarted), ShouldBeLessThan, 2*time.Second)
						So(time.Since(m.LastEnded), ShouldBeLessThan, 3*time.Second)
					})
				})

				Convey("Update schedule of valid mirrors", func(ctx C) {
					msg := MirrorSchedules{
						Schedules: []MirrorSchedule{
							{MirrorName: "arch-sync1", NextSchedule: time.Now().Add(time.Minute * 10)},
							{MirrorName: "arch-sync2", NextSchedule: time.Now().Add(time.Minute * 7)},
						},
					}

					url := fmt.Sprintf("%s/workers/%s/schedules", baseURL, status.Worker)
					resp, err := PostJSON(url, msg, nil)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
				})

				Convey("Update size of an invalid mirror", func(ctx C) {
					msg := struct {
						Name string `json:"name"`
						Size string `json:"size"`
					}{"Invalid mirror", "5GB"}

					url := fmt.Sprintf("%s/workers/%s/jobs/%s/size", baseURL, status.Worker, status.Name)
					resp, err := PostJSON(url, msg, nil)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
				})

				// what if status changed to failed
				status.Status = Failed
				time.Sleep(3 * time.Second)
				resp, err = PostJSON(fmt.Sprintf("%s/workers/%s/jobs/%s", baseURL, status.Worker, status.Name), status, nil)
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				So(resp.StatusCode, ShouldEqual, http.StatusOK)

				Convey("What if syncing job failed", func(ctx C) {
					var ms []MirrorStatus
					resp, err := GetJSON(baseURL+"/workers/test_worker1/jobs", &ms, nil)

					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					// err = json.NewDecoder(resp.Body).Decode(&mirrorStatusList)
					m := ms[0]
					So(m.Name, ShouldEqual, status.Name)
					So(m.Worker, ShouldEqual, status.Worker)
					So(m.Status, ShouldEqual, status.Status)
					So(m.Upstream, ShouldEqual, status.Upstream)
					So(m.Size, ShouldEqual, status.Size)
					So(m.IsMaster, ShouldEqual, status.IsMaster)
					So(time.Since(m.LastUpdate), ShouldBeGreaterThan, 3*time.Second)
					So(time.Since(m.LastStarted), ShouldBeGreaterThan, 3*time.Second)
					So(time.Since(m.LastEnded), ShouldBeLessThan, 1*time.Second)
				})
			})

			Convey("update mirror status of an inexisted worker", func(ctx C) {
				invalidWorker := "test_worker2"
				status := MirrorStatus{
					Name:        "arch-sync2",
					Worker:      invalidWorker,
					IsMaster:    true,
					Status:      Success,
					LastUpdate:  time.Now(),
					LastStarted: time.Now(),
					LastEnded:   time.Now(),
					Upstream:    "mirrors.tuna.tsinghua.edu.cn",
					Size:        "4GB",
				}
				resp, err := PostJSON(fmt.Sprintf("%s/workers/%s/jobs/%s",
					baseURL, status.Worker, status.Name), status, nil)
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				defer resp.Body.Close()
				var msg map[string]string
				err = json.NewDecoder(resp.Body).Decode(&msg)
				So(err, ShouldBeNil)
				So(msg[_errorKey], ShouldEqual, "invalid workerID "+invalidWorker)
			})
			Convey("update schedule of an non-existent worker", func(ctx C) {
				invalidWorker := "test_worker2"
				sch := MirrorSchedules{
					Schedules: []MirrorSchedule{
						{MirrorName: "arch-sync1", NextSchedule: time.Now().Add(time.Minute * 10)},
						{MirrorName: "arch-sync2", NextSchedule: time.Now().Add(time.Minute * 7)},
					},
				}
				resp, err := PostJSON(fmt.Sprintf("%s/workers/%s/schedules",
					baseURL, invalidWorker), sch, nil)
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				defer resp.Body.Close()
				var msg map[string]string
				err = json.NewDecoder(resp.Body).Decode(&msg)
				So(err, ShouldBeNil)
				So(msg[_errorKey], ShouldEqual, "invalid workerID "+invalidWorker)
			})
			Convey("handle client command", func(ctx C) {
				cmdChan := make(chan WorkerCmd, 1)
				workerServer := makeMockWorkerServer(cmdChan)
				workerPort := rand.Intn(10000) + 30000
				bindAddress := fmt.Sprintf("127.0.0.1:%d", workerPort)
				workerBaseURL := fmt.Sprintf("http://%s", bindAddress)
				w := WorkerStatus{
					ID:  "test_worker_cmd",
					URL: workerBaseURL + "/cmd",
				}
				resp, err := PostJSON(baseURL+"/workers", w, nil)
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusOK)

				go func() {
					// run the mock worker server
					workerServer.Run(bindAddress)
				}()
				time.Sleep(50 * time.Millisecond)
				// verify the worker mock server is running
				workerResp, err := http.Get(workerBaseURL + "/ping")
				So(err, ShouldBeNil)
				defer workerResp.Body.Close()
				So(workerResp.StatusCode, ShouldEqual, http.StatusOK)

				Convey("when client send wrong cmd", func(ctx C) {
					clientCmd := ClientCmd{
						Cmd:      CmdStart,
						MirrorID: "ubuntu-sync",
						WorkerID: "not_exist_worker",
					}
					resp, err := PostJSON(baseURL+"/cmd", clientCmd, nil)
					So(err, ShouldBeNil)
					defer resp.Body.Close()
					So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				})

				Convey("when client send correct cmd", func(ctx C) {
					clientCmd := ClientCmd{
						Cmd:      CmdStart,
						MirrorID: "ubuntu-sync",
						WorkerID: w.ID,
					}

					resp, err := PostJSON(baseURL+"/cmd", clientCmd, nil)
					So(err, ShouldBeNil)
					defer resp.Body.Close()
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					time.Sleep(50 * time.Microsecond)
					select {
					case cmd := <-cmdChan:
						ctx.So(cmd.Cmd, ShouldEqual, clientCmd.Cmd)
						ctx.So(cmd.MirrorID, ShouldEqual, clientCmd.MirrorID)
					default:
						ctx.So(0, ShouldEqual, 1)
					}
				})
			})
		})
	})
}

type mockDBAdapter struct {
	workerStore map[string]WorkerStatus
	statusStore map[string]MirrorStatus
	workerLock  sync.RWMutex
	statusLock  sync.RWMutex
}

func (b *mockDBAdapter) Init() error {
	return nil
}

func (b *mockDBAdapter) ListWorkers() ([]WorkerStatus, error) {
	b.workerLock.RLock()
	workers := make([]WorkerStatus, len(b.workerStore))
	idx := 0
	for _, w := range b.workerStore {
		workers[idx] = w
		idx++
	}
	b.workerLock.RUnlock()
	return workers, nil
}

func (b *mockDBAdapter) GetWorker(workerID string) (WorkerStatus, error) {
	b.workerLock.RLock()
	defer b.workerLock.RUnlock()
	w, ok := b.workerStore[workerID]

	if !ok {
		return WorkerStatus{}, fmt.Errorf("invalid workerId")
	}
	return w, nil
}

func (b *mockDBAdapter) DeleteWorker(workerID string) error {
	b.workerLock.Lock()
	delete(b.workerStore, workerID)
	b.workerLock.Unlock()
	return nil
}

func (b *mockDBAdapter) CreateWorker(w WorkerStatus) (WorkerStatus, error) {
	// _, ok := b.workerStore[w.ID]
	// if ok {
	// 	return workerStatus{}, fmt.Errorf("duplicate worker name")
	// }
	b.workerLock.Lock()
	b.workerStore[w.ID] = w
	b.workerLock.Unlock()
	return w, nil
}

func (b *mockDBAdapter) RefreshWorker(workerID string) (w WorkerStatus, err error) {
	w, err = b.GetWorker(workerID)
	if err == nil {
		w.LastOnline = time.Now()
		w, err = b.CreateWorker(w)
	}
	return w, err
}

func (b *mockDBAdapter) GetMirrorStatus(workerID, mirrorID string) (MirrorStatus, error) {
	id := mirrorID + "/" + workerID
	b.statusLock.RLock()
	status, ok := b.statusStore[id]
	b.statusLock.RUnlock()
	if !ok {
		return MirrorStatus{}, fmt.Errorf("no mirror %s exists in worker %s", mirrorID, workerID)
	}
	return status, nil
}

func (b *mockDBAdapter) UpdateMirrorStatus(workerID, mirrorID string, status MirrorStatus) (MirrorStatus, error) {
	// if _, ok := b.workerStore[workerID]; !ok {
	// 	// unregistered worker
	// 	return MirrorStatus{}, fmt.Errorf("invalid workerID %s", workerID)
	// }

	id := mirrorID + "/" + workerID
	b.statusLock.Lock()
	b.statusStore[id] = status
	b.statusLock.Unlock()
	return status, nil
}

func (b *mockDBAdapter) ListMirrorStatus(workerID string) ([]MirrorStatus, error) {
	var mirrorStatusList []MirrorStatus
	// simulating a database fail
	if workerID == _magicBadWorkerID {
		return []MirrorStatus{}, fmt.Errorf("database fail")
	}
	b.statusLock.RLock()
	for k, v := range b.statusStore {
		if wID := strings.Split(k, "/")[1]; wID == workerID {
			mirrorStatusList = append(mirrorStatusList, v)
		}
	}
	b.statusLock.RUnlock()
	return mirrorStatusList, nil
}

func (b *mockDBAdapter) ListAllMirrorStatus() ([]MirrorStatus, error) {
	var mirrorStatusList []MirrorStatus
	b.statusLock.RLock()
	for _, v := range b.statusStore {
		mirrorStatusList = append(mirrorStatusList, v)
	}
	b.statusLock.RUnlock()
	return mirrorStatusList, nil
}

func (b *mockDBAdapter) Close() error {
	return nil
}

func (b *mockDBAdapter) FlushDisabledJobs() error {
	return nil
}

func makeMockWorkerServer(cmdChan chan WorkerCmd) *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{_infoKey: "pong"})
	})
	r.POST("/cmd", func(c *gin.Context) {
		var cmd WorkerCmd
		c.BindJSON(&cmd)
		cmdChan <- cmd
	})

	return r
}
