package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

func SortMirrorStatus(status []MirrorStatus) {
	sort.Slice(status, func(l, r int) bool {
		return status[l].Name < status[r].Name
	})
}

func DBAdapterTest(db dbAdapter) {
	var err error
	testWorkerIDs := []string{"test_worker1", "test_worker2"}
	Convey("create worker", func() {
		for _, id := range testWorkerIDs {
			w := WorkerStatus{
				ID:           id,
				Token:        "token_" + id,
				LastOnline:   time.Now(),
				LastRegister: time.Now(),
			}
			_, err = db.CreateWorker(w)
			So(err, ShouldBeNil)
		}

		Convey("get existent worker", func() {
			_, err := db.GetWorker(testWorkerIDs[0])
			So(err, ShouldBeNil)
		})

		Convey("list existent workers", func() {
			ws, err := db.ListWorkers()
			So(err, ShouldBeNil)
			So(len(ws), ShouldEqual, 2)
		})

		Convey("get non-existent worker", func() {
			_, err := db.GetWorker("invalid workerID")
			So(err, ShouldNotBeNil)
		})

		Convey("delete existent worker", func() {
			err := db.DeleteWorker(testWorkerIDs[0])
			So(err, ShouldBeNil)
			_, err = db.GetWorker(testWorkerIDs[0])
			So(err, ShouldNotBeNil)
			ws, err := db.ListWorkers()
			So(err, ShouldBeNil)
			So(len(ws), ShouldEqual, 1)
		})

		Convey("delete non-existent worker", func() {
			err := db.DeleteWorker("invalid workerID")
			So(err, ShouldNotBeNil)
			ws, err := db.ListWorkers()
			So(err, ShouldBeNil)
			So(len(ws), ShouldEqual, 2)
		})
	})

	Convey("update mirror status", func() {
		status := []MirrorStatus{
			{
				Name:        "arch-sync1",
				Worker:      testWorkerIDs[0],
				IsMaster:    true,
				Status:      Success,
				LastUpdate:  time.Now(),
				LastStarted: time.Now().Add(-time.Minute),
				LastEnded:   time.Now(),
				Upstream:    "mirrors.tuna.tsinghua.edu.cn",
				Size:        "3GB",
			},
			{
				Name:        "arch-sync2",
				Worker:      testWorkerIDs[1],
				IsMaster:    true,
				Status:      Disabled,
				LastUpdate:  time.Now().Add(-time.Hour),
				LastStarted: time.Now().Add(-time.Minute),
				LastEnded:   time.Now(),
				Upstream:    "mirrors.tuna.tsinghua.edu.cn",
				Size:        "4GB",
			},
			{
				Name:        "arch-sync3",
				Worker:      testWorkerIDs[1],
				IsMaster:    true,
				Status:      Success,
				LastUpdate:  time.Now().Add(-time.Minute),
				LastStarted: time.Now().Add(-time.Second),
				LastEnded:   time.Now(),
				Upstream:    "mirrors.tuna.tsinghua.edu.cn",
				Size:        "4GB",
			},
		}
		SortMirrorStatus(status)

		for _, s := range status {
			_, err := db.UpdateMirrorStatus(s.Worker, s.Name, s)
			So(err, ShouldBeNil)

		}

		Convey("get mirror status", func() {
			m, err := db.GetMirrorStatus(testWorkerIDs[0], status[0].Name)
			So(err, ShouldBeNil)
			expectedJSON, err := json.Marshal(status[0])
			So(err, ShouldBeNil)
			actualJSON, err := json.Marshal(m)
			So(err, ShouldBeNil)
			So(string(actualJSON), ShouldEqual, string(expectedJSON))
		})

		Convey("list mirror status", func() {
			ms, err := db.ListMirrorStatus(testWorkerIDs[0])
			So(err, ShouldBeNil)
			expectedJSON, err := json.Marshal([]MirrorStatus{status[0]})
			So(err, ShouldBeNil)
			actualJSON, err := json.Marshal(ms)
			So(err, ShouldBeNil)
			So(string(actualJSON), ShouldEqual, string(expectedJSON))
		})

		Convey("list all mirror status", func() {
			ms, err := db.ListAllMirrorStatus()
			So(err, ShouldBeNil)
			SortMirrorStatus(ms)

			expectedJSON, err := json.Marshal(status)
			So(err, ShouldBeNil)
			actualJSON, err := json.Marshal(ms)
			So(err, ShouldBeNil)
			So(string(actualJSON), ShouldEqual, string(expectedJSON))
		})

		Convey("flush disabled jobs", func() {
			ms, err := db.ListAllMirrorStatus()
			So(err, ShouldBeNil)
			So(len(ms), ShouldEqual, 3)
			err = db.FlushDisabledJobs()
			So(err, ShouldBeNil)
			ms, err = db.ListAllMirrorStatus()
			So(err, ShouldBeNil)
			So(len(ms), ShouldEqual, 2)
		})

	})
}

func TestDBAdapter(t *testing.T) {
	Convey("boltAdapter should work", t, func() {
		tmpDir, err := os.MkdirTemp("", "tunasync")
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

		DBAdapterTest(boltDB)
	})

	Convey("redisAdapter should work", t, func() {
		mr, err := miniredis.Run()
		So(err, ShouldBeNil)

		addr := fmt.Sprintf("redis://%s", mr.Addr())
		redisDB, err := makeDBAdapter("redis", addr)
		So(err, ShouldBeNil)

		defer func() {
			// close redisDB
			err := redisDB.Close()
			So(err, ShouldBeNil)
			mr.Close()
		}()

		DBAdapterTest(redisDB)
	})

	Convey("badgerAdapter should work", t, func() {
		tmpDir, err := os.MkdirTemp("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)

		dbType, dbFile := "badger", filepath.Join(tmpDir, "badger.db")
		badgerDB, err := makeDBAdapter(dbType, dbFile)
		So(err, ShouldBeNil)

		defer func() {
			// close badgerDB
			err := badgerDB.Close()
			So(err, ShouldBeNil)
		}()

		DBAdapterTest(badgerDB)
	})

	Convey("leveldbAdapter should work", t, func() {
		tmpDir, err := os.MkdirTemp("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)

		dbType, dbFile := "leveldb", filepath.Join(tmpDir, "leveldb.db")
		leveldbDB, err := makeDBAdapter(dbType, dbFile)
		So(err, ShouldBeNil)

		defer func() {
			// close leveldbDB
			err := leveldbDB.Close()
			So(err, ShouldBeNil)
		}()

		DBAdapterTest(leveldbDB)
	})
}
