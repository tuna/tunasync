package manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
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
	Convey("HTTP server should work", t, func(ctx C) {
		InitLogger(true, true, false)
		s := GetTUNASyncManager(&Config{Debug: false})
		So(s, ShouldNotBeNil)
		s.setDBAdapter(&mockDBAdapter{
			workerStore: map[string]WorkerStatus{
				_magicBadWorkerID: WorkerStatus{
					ID: _magicBadWorkerID,
				}},
			statusStore: make(map[string]MirrorStatus),
		})
		port := rand.Intn(10000) + 20000
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		go func() {
			s.engine.Run(fmt.Sprintf("127.0.0.1:%d", port))
		}()
		time.Sleep(50 * time.Microsecond)
		resp, err := http.Get(baseURL + "/ping")
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		So(resp.Header.Get("Content-Type"), ShouldEqual, "application/json; charset=utf-8")
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
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

			Convey("flush  disabled jobs", func(ctx C) {
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
				defer resp.Body.Close()
				So(err, ShouldBeNil)
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
					So(time.Now().Sub(m.LastUpdate), ShouldBeLessThan, 1*time.Second)
					So(time.Now().Sub(m.LastEnded), ShouldBeLessThan, 1*time.Second)

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
					So(time.Now().Sub(m.LastUpdate.Time), ShouldBeLessThan, 1*time.Second)
					So(time.Now().Sub(m.LastEnded.Time), ShouldBeLessThan, 1*time.Second)

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
						So(time.Now().Sub(m.LastUpdate), ShouldBeLessThan, 1*time.Second)
						So(time.Now().Sub(m.LastEnded), ShouldBeLessThan, 1*time.Second)
					})
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
				defer resp.Body.Close()
				So(err, ShouldBeNil)
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
					So(time.Now().Sub(m.LastUpdate), ShouldBeGreaterThan, 3*time.Second)
					So(time.Now().Sub(m.LastEnded), ShouldBeLessThan, 1*time.Second)
				})
			})

			Convey("update mirror status of an inexisted worker", func(ctx C) {
				invalidWorker := "test_worker2"
				status := MirrorStatus{
					Name:       "arch-sync2",
					Worker:     invalidWorker,
					IsMaster:   true,
					Status:     Success,
					LastUpdate: time.Now(),
					LastEnded:  time.Now(),
					Upstream:   "mirrors.tuna.tsinghua.edu.cn",
					Size:       "4GB",
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
				time.Sleep(50 * time.Microsecond)
				// verify the worker mock server is running
				workerResp, err := http.Get(workerBaseURL + "/ping")
				defer workerResp.Body.Close()
				So(err, ShouldBeNil)
				So(workerResp.StatusCode, ShouldEqual, http.StatusOK)

				Convey("when client send wrong cmd", func(ctx C) {
					clientCmd := ClientCmd{
						Cmd:      CmdStart,
						MirrorID: "ubuntu-sync",
						WorkerID: "not_exist_worker",
					}
					resp, err := PostJSON(baseURL+"/cmd", clientCmd, nil)
					defer resp.Body.Close()
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				})

				Convey("when client send correct cmd", func(ctx C) {
					clientCmd := ClientCmd{
						Cmd:      CmdStart,
						MirrorID: "ubuntu-sync",
						WorkerID: w.ID,
					}

					resp, err := PostJSON(baseURL+"/cmd", clientCmd, nil)
					defer resp.Body.Close()

					So(err, ShouldBeNil)
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
}

func (b *mockDBAdapter) Init() error {
	return nil
}

func (b *mockDBAdapter) ListWorkers() ([]WorkerStatus, error) {
	workers := make([]WorkerStatus, len(b.workerStore))
	idx := 0
	for _, w := range b.workerStore {
		workers[idx] = w
		idx++
	}
	return workers, nil
}

func (b *mockDBAdapter) GetWorker(workerID string) (WorkerStatus, error) {
	w, ok := b.workerStore[workerID]
	if !ok {
		return WorkerStatus{}, fmt.Errorf("invalid workerId")
	}
	return w, nil
}

func (b *mockDBAdapter) DeleteWorker(workerID string) error {
	delete(b.workerStore, workerID)
	return nil
}

func (b *mockDBAdapter) CreateWorker(w WorkerStatus) (WorkerStatus, error) {
	// _, ok := b.workerStore[w.ID]
	// if ok {
	// 	return workerStatus{}, fmt.Errorf("duplicate worker name")
	// }
	b.workerStore[w.ID] = w
	return w, nil
}

func (b *mockDBAdapter) GetMirrorStatus(workerID, mirrorID string) (MirrorStatus, error) {
	id := mirrorID + "/" + workerID
	status, ok := b.statusStore[id]
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
	b.statusStore[id] = status
	return status, nil
}

func (b *mockDBAdapter) ListMirrorStatus(workerID string) ([]MirrorStatus, error) {
	var mirrorStatusList []MirrorStatus
	// simulating a database fail
	if workerID == _magicBadWorkerID {
		return []MirrorStatus{}, fmt.Errorf("database fail")
	}
	for k, v := range b.statusStore {
		if wID := strings.Split(k, "/")[1]; wID == workerID {
			mirrorStatusList = append(mirrorStatusList, v)
		}
	}
	return mirrorStatusList, nil
}

func (b *mockDBAdapter) ListAllMirrorStatus() ([]MirrorStatus, error) {
	var mirrorStatusList []MirrorStatus
	for _, v := range b.statusStore {
		mirrorStatusList = append(mirrorStatusList, v)
	}
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
