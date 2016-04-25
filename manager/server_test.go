package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

const (
	_magicBadWorkerID = "magic_bad_worker_id"
)

func postJSON(url string, obj interface{}) (*http.Response, error) {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(obj)
	return http.Post(url, "application/json; charset=utf-8", b)
}

func TestHTTPServer(t *testing.T) {
	Convey("HTTP server should work", t, func() {
		InitLogger(true, true, false)
		s := makeHTTPServer(false)
		So(s, ShouldNotBeNil)
		s.setDBAdapter(&mockDBAdapter{
			workerStore: map[string]worker{
				_magicBadWorkerID: worker{
					ID: _magicBadWorkerID,
				}},
			statusStore: make(map[string]mirrorStatus),
		})
		port := rand.Intn(10000) + 20000
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		go func() {
			s.Run(fmt.Sprintf("127.0.0.1:%d", port))
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

		Convey("when database fail", func() {
			resp, err := http.Get(fmt.Sprintf("%s/workers/%s/jobs", baseURL, _magicBadWorkerID))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
			defer resp.Body.Close()
			var msg map[string]string
			err = json.NewDecoder(resp.Body).Decode(&msg)
			So(err, ShouldBeNil)
			So(msg[_errorKey], ShouldEqual, fmt.Sprintf("failed to list jobs of worker %s: %s", _magicBadWorkerID, "database fail"))
		})

		Convey("when register a worker", func() {
			w := worker{
				ID: "test_worker1",
			}
			resp, err := postJSON(baseURL+"/workers", w)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)

			Convey("list all workers", func() {
				So(err, ShouldBeNil)
				resp, err := http.Get(baseURL + "/workers")
				So(err, ShouldBeNil)
				defer resp.Body.Close()
				var actualResponseObj []WorkerInfoMsg
				err = json.NewDecoder(resp.Body).Decode(&actualResponseObj)
				So(err, ShouldBeNil)
				So(len(actualResponseObj), ShouldEqual, 2)
			})

			Convey("update mirror status of a existed worker", func() {
				status := mirrorStatus{
					Name:       "arch-sync1",
					Worker:     "test_worker1",
					IsMaster:   true,
					Status:     Success,
					LastUpdate: time.Now(),
					Upstream:   "mirrors.tuna.tsinghua.edu.cn",
					Size:       "3GB",
				}
				resp, err := postJSON(fmt.Sprintf("%s/workers/%s/jobs/%s", baseURL, status.Worker, status.Name), status)
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusOK)

				Convey("list mirror status of an existed worker", func() {

					expectedResponse, err := json.Marshal([]mirrorStatus{status})
					So(err, ShouldBeNil)
					resp, err := http.Get(baseURL + "/workers/test_worker1/jobs")
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					// err = json.NewDecoder(resp.Body).Decode(&mirrorStatusList)
					body, err := ioutil.ReadAll(resp.Body)
					defer resp.Body.Close()
					So(err, ShouldBeNil)
					So(strings.TrimSpace(string(body)), ShouldEqual, string(expectedResponse))
				})

				Convey("list all job status of all workers", func() {
					expectedResponse, err := json.Marshal([]mirrorStatus{status})
					So(err, ShouldBeNil)
					resp, err := http.Get(baseURL + "/jobs")
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					body, err := ioutil.ReadAll(resp.Body)
					defer resp.Body.Close()
					So(err, ShouldBeNil)
					So(strings.TrimSpace(string(body)), ShouldEqual, string(expectedResponse))

				})
			})

			Convey("update mirror status of an inexisted worker", func() {
				invalidWorker := "test_worker2"
				status := mirrorStatus{
					Name:       "arch-sync2",
					Worker:     invalidWorker,
					IsMaster:   true,
					Status:     Success,
					LastUpdate: time.Now(),
					Upstream:   "mirrors.tuna.tsinghua.edu.cn",
					Size:       "4GB",
				}
				resp, err := postJSON(fmt.Sprintf("%s/workers/%s/jobs/%s",
					baseURL, status.Worker, status.Name), status)
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				defer resp.Body.Close()
				var msg map[string]string
				err = json.NewDecoder(resp.Body).Decode(&msg)
				So(err, ShouldBeNil)
				So(msg[_errorKey], ShouldEqual, "invalid workerID "+invalidWorker)
			})
		})
	})
}

type mockDBAdapter struct {
	workerStore map[string]worker
	statusStore map[string]mirrorStatus
}

func (b *mockDBAdapter) Init() error {
	return nil
}

func (b *mockDBAdapter) ListWorkers() ([]worker, error) {
	workers := make([]worker, len(b.workerStore))
	idx := 0
	for _, w := range b.workerStore {
		workers[idx] = w
		idx++
	}
	return workers, nil
}

func (b *mockDBAdapter) GetWorker(workerID string) (worker, error) {
	w, ok := b.workerStore[workerID]
	if !ok {
		return worker{}, fmt.Errorf("invalid workerId")
	}
	return w, nil
}

func (b *mockDBAdapter) CreateWorker(w worker) (worker, error) {
	// _, ok := b.workerStore[w.ID]
	// if ok {
	// 	return worker{}, fmt.Errorf("duplicate worker name")
	// }
	b.workerStore[w.ID] = w
	return w, nil
}

func (b *mockDBAdapter) GetMirrorStatus(workerID, mirrorID string) (mirrorStatus, error) {
	id := mirrorID + "/" + workerID
	status, ok := b.statusStore[id]
	if !ok {
		return mirrorStatus{}, fmt.Errorf("no mirror %s exists in worker %s", mirrorID, workerID)
	}
	return status, nil
}

func (b *mockDBAdapter) UpdateMirrorStatus(workerID, mirrorID string, status mirrorStatus) (mirrorStatus, error) {
	// if _, ok := b.workerStore[workerID]; !ok {
	// 	// unregistered worker
	// 	return mirrorStatus{}, fmt.Errorf("invalid workerID %s", workerID)
	// }

	id := mirrorID + "/" + workerID
	b.statusStore[id] = status
	return status, nil
}

func (b *mockDBAdapter) ListMirrorStatus(workerID string) ([]mirrorStatus, error) {
	var mirrorStatusList []mirrorStatus
	// simulating a database fail
	if workerID == _magicBadWorkerID {
		return []mirrorStatus{}, fmt.Errorf("database fail")
	}
	for k, v := range b.statusStore {
		if wID := strings.Split(k, "/")[1]; wID == workerID {
			mirrorStatusList = append(mirrorStatusList, v)
		}
	}
	return mirrorStatusList, nil
}

func (b *mockDBAdapter) ListAllMirrorStatus() ([]mirrorStatus, error) {
	var mirrorStatusList []mirrorStatus
	for _, v := range b.statusStore {
		mirrorStatusList = append(mirrorStatusList, v)
	}
	return mirrorStatusList, nil
}

func (b *mockDBAdapter) Close() error {
	return nil
}
