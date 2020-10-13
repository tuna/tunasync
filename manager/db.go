package manager

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"

	. "github.com/tuna/tunasync/internal"
)

type dbAdapter interface {
	Init() error
	ListWorkers() ([]WorkerStatus, error)
	GetWorker(workerID string) (WorkerStatus, error)
	DeleteWorker(workerID string) error
	CreateWorker(w WorkerStatus) (WorkerStatus, error)
	RefreshWorker(workerID string) (WorkerStatus, error)
	UpdateMirrorStatus(workerID, mirrorID string, status MirrorStatus) (MirrorStatus, error)
	GetMirrorStatus(workerID, mirrorID string) (MirrorStatus, error)
	ListMirrorStatus(workerID string) ([]MirrorStatus, error)
	ListAllMirrorStatus() ([]MirrorStatus, error)
	FlushDisabledJobs() error
	Close() error
}

func makeDBAdapter(dbType string, dbFile string) (dbAdapter, error) {
	if dbType == "bolt" {
		innerDB, err := bolt.Open(dbFile, 0600, &bolt.Options{
			Timeout: 5 * time.Second,
		})
		if err != nil {
			return nil, err
		}
		db := boltAdapter{
			db:     innerDB,
			dbFile: dbFile,
		}
		err = db.Init()
		return &db, err
	}
	// unsupported db-type
	return nil, fmt.Errorf("unsupported db-type: %s", dbType)
}
