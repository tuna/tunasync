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

	. "github.com/smartystreets/goconvey/convey"
)

type mockDBAdapter struct {
	workerStore map[string]worker
	statusStore map[string]mirrorStatus
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
		return worker{}, fmt.Errorf("inexist workerId")
	}
	return w, nil
}

func (b *mockDBAdapter) CreateWorker(w worker) (worker, error) {
	_, ok := b.workerStore[w.id]
	if ok {
		return worker{}, fmt.Errorf("duplicate worker name")
	}
	b.workerStore[w.id] = w
	return w, nil
}

func (b *mockDBAdapter) GetMirrorStatus(workerID, mirrorID string) (mirrorStatus, error) {
	// TODO: need to check worker exist first
	id := workerID + "/" + mirrorID
	status, ok := b.statusStore[id]
	if !ok {
		return mirrorStatus{}, fmt.Errorf("no mirror %s exists in worker %s", mirrorID, workerID)
	}
	return status, nil
}

func (b *mockDBAdapter) UpdateMirrorStatus(workerID, mirrorID string, status mirrorStatus) (mirrorStatus, error) {
	id := workerID + "/" + mirrorID
	b.statusStore[id] = status
	return status, nil
}

func (b *mockDBAdapter) ListMirrorStatus(workerID string) ([]mirrorStatus, error) {
	var mirrorStatusList []mirrorStatus
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

func TestHTTPServer(t *testing.T) {
	Convey("HTTP server should work", t, func() {
		s := makeHTTPServer(false)
		So(s, ShouldNotBeNil)
		port := rand.Intn(10000) + 20000
		go func() {
			s.Run(fmt.Sprintf("127.0.0.1:%d", port))
		}()
		time.Sleep(50 * time.Microsecond)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", port))
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		So(resp.Header.Get("Content-Type"), ShouldEqual, "application/json; charset=utf-8")
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		var p map[string]string
		err = json.Unmarshal(body, &p)
		So(err, ShouldBeNil)
		So(p["msg"], ShouldEqual, "pong")
	})

}
