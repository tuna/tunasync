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
				w := WorkerStatus{
					ID:         id,
					Token:      "token_" + id,
					LastOnline: time.Now(),
				}
				w, err = boltDB.CreateWorker(w)
				So(err, ShouldBeNil)
			}

			Convey("get existent worker", func() {
				_, err := boltDB.GetWorker(testWorkerIDs[0])
				So(err, ShouldBeNil)
			})

			Convey("list existent workers", func() {
				ws, err := boltDB.ListWorkers()
				So(err, ShouldBeNil)
				So(len(ws), ShouldEqual, 2)
			})

			Convey("get non-existent worker", func() {
				_, err := boltDB.GetWorker("invalid workerID")
				So(err, ShouldNotBeNil)
			})

			Convey("delete existent worker", func() {
				err := boltDB.DeleteWorker(testWorkerIDs[0])
				So(err, ShouldBeNil)
				_, err = boltDB.GetWorker(testWorkerIDs[0])
				So(err, ShouldNotBeNil)
				ws, err := boltDB.ListWorkers()
				So(err, ShouldBeNil)
				So(len(ws), ShouldEqual, 1)
			})

			Convey("delete non-existent worker", func() {
				err := boltDB.DeleteWorker("invalid workerID")
				So(err, ShouldNotBeNil)
				ws, err := boltDB.ListWorkers()
				So(err, ShouldBeNil)
				So(len(ws), ShouldEqual, 2)
			})
		})

		Convey("update mirror status", func() {
			status := []MirrorStatus{
				MirrorStatus{
					Name:       "arch-sync1",
					Worker:     testWorkerIDs[0],
					IsMaster:   true,
					Status:     Success,
					LastUpdate: time.Now(),
					LastEnded:  time.Now(),
					Upstream:   "mirrors.tuna.tsinghua.edu.cn",
					Size:       "3GB",
				},
				MirrorStatus{
					Name:       "arch-sync2",
					Worker:     testWorkerIDs[1],
					IsMaster:   true,
					Status:     Disabled,
					LastUpdate: time.Now().Add(-time.Hour),
					LastEnded:  time.Now(),
					Upstream:   "mirrors.tuna.tsinghua.edu.cn",
					Size:       "4GB",
				},
				MirrorStatus{
					Name:       "arch-sync3",
					Worker:     testWorkerIDs[1],
					IsMaster:   true,
					Status:     Success,
					LastUpdate: time.Now().Add(-time.Second),
					LastEnded:  time.Now(),
					Upstream:   "mirrors.tuna.tsinghua.edu.cn",
					Size:       "4GB",
				},
			}

			for _, s := range status {
				_, err := boltDB.UpdateMirrorStatus(s.Worker, s.Name, s)
				So(err, ShouldBeNil)

			}

			Convey("get mirror status", func() {
				m, err := boltDB.GetMirrorStatus(testWorkerIDs[0], status[0].Name)
				So(err, ShouldBeNil)
				expectedJSON, err := json.Marshal(status[0])
				So(err, ShouldBeNil)
				actualJSON, err := json.Marshal(m)
				So(err, ShouldBeNil)
				So(string(actualJSON), ShouldEqual, string(expectedJSON))
			})

			Convey("list mirror status", func() {
				ms, err := boltDB.ListMirrorStatus(testWorkerIDs[0])
				So(err, ShouldBeNil)
				expectedJSON, err := json.Marshal([]MirrorStatus{status[0]})
				So(err, ShouldBeNil)
				actualJSON, err := json.Marshal(ms)
				So(err, ShouldBeNil)
				So(string(actualJSON), ShouldEqual, string(expectedJSON))
			})

			Convey("list all mirror status", func() {
				ms, err := boltDB.ListAllMirrorStatus()
				So(err, ShouldBeNil)
				expectedJSON, err := json.Marshal(status)
				So(err, ShouldBeNil)
				actualJSON, err := json.Marshal(ms)
				So(err, ShouldBeNil)
				So(string(actualJSON), ShouldEqual, string(expectedJSON))
			})

			Convey("flush disabled jobs", func() {
				ms, err := boltDB.ListAllMirrorStatus()
				So(err, ShouldBeNil)
				So(len(ms), ShouldEqual, 3)
				err = boltDB.FlushDisabledJobs()
				So(err, ShouldBeNil)
				ms, err = boltDB.ListAllMirrorStatus()
				So(err, ShouldBeNil)
				So(len(ms), ShouldEqual, 2)
			})

		})

	})
}
