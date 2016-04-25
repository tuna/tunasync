package manager

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

func TestBoltAdapter(t *testing.T) {
	Convey("boltAdapter should work", t, func() {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)

		dbType, dbFile := "bolt", filepath.Join(tmpDir, "bolt.db")
		boltDB, err := makeDBAdapter(dbType, dbFile)
		So(err, ShouldBeNil)

		defer func() {
			// close boltDB
			err := boltDB.Close()
			So(err, ShouldBeNil)
		}()

		testWorkerIDs := []string{"test_worker1", "test_worker2"}
		Convey("create worker", func() {
			for _, id := range testWorkerIDs {
				w := workerStatus{
					ID:         id,
					Token:      "token_" + id,
					LastOnline: time.Now(),
				}
				w, err = boltDB.CreateWorker(w)
				So(err, ShouldBeNil)
			}

			Convey("get exists worker", func() {
				_, err := boltDB.GetWorker(testWorkerIDs[0])
				So(err, ShouldBeNil)
			})

			Convey("list exist worker", func() {
				ws, err := boltDB.ListWorkers()
				So(err, ShouldBeNil)
				So(len(ws), ShouldEqual, 2)
			})

			Convey("get inexist worker", func() {
				_, err := boltDB.GetWorker("invalid workerID")
				So(err, ShouldNotBeNil)
			})
		})

		Convey("update mirror status", func() {
			status1 := mirrorStatus{
				Name:       "arch-sync1",
				Worker:     testWorkerIDs[0],
				IsMaster:   true,
				Status:     Success,
				LastUpdate: time.Now(),
				Upstream:   "mirrors.tuna.tsinghua.edu.cn",
				Size:       "3GB",
			}
			status2 := mirrorStatus{
				Name:       "arch-sync2",
				Worker:     testWorkerIDs[1],
				IsMaster:   true,
				Status:     Success,
				LastUpdate: time.Now(),
				Upstream:   "mirrors.tuna.tsinghua.edu.cn",
				Size:       "4GB",
			}

			_, err := boltDB.UpdateMirrorStatus(status1.Worker, status1.Name, status1)
			_, err = boltDB.UpdateMirrorStatus(status2.Worker, status2.Name, status2)
			So(err, ShouldBeNil)

			Convey("get mirror status", func() {
				m, err := boltDB.GetMirrorStatus(testWorkerIDs[0], status1.Name)
				So(err, ShouldBeNil)
				expectedJSON, err := json.Marshal(status1)
				So(err, ShouldBeNil)
				actualJSON, err := json.Marshal(m)
				So(err, ShouldBeNil)
				So(string(actualJSON), ShouldEqual, string(expectedJSON))
			})

			Convey("list mirror status", func() {
				ms, err := boltDB.ListMirrorStatus(testWorkerIDs[0])
				So(err, ShouldBeNil)
				expectedJSON, err := json.Marshal([]mirrorStatus{status1})
				So(err, ShouldBeNil)
				actualJSON, err := json.Marshal(ms)
				So(err, ShouldBeNil)
				So(string(actualJSON), ShouldEqual, string(expectedJSON))
			})

			Convey("list all mirror status", func() {
				ms, err := boltDB.ListAllMirrorStatus()
				So(err, ShouldBeNil)
				expectedJSON, err := json.Marshal([]mirrorStatus{status1, status2})
				So(err, ShouldBeNil)
				actualJSON, err := json.Marshal(ms)
				So(err, ShouldBeNil)
				So(string(actualJSON), ShouldEqual, string(expectedJSON))
			})

		})

	})
}
