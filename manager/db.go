package manager

import (
	"fmt"
	"github.com/boltdb/bolt"
)

type dbAdapter interface {
	ListWorkers() ([]worker, error)
	GetWorker(workerID string) (worker, error)
	CreateWorker(w worker) (worker, error)
	UpdateMirrorStatus(workerID, mirrorID string, status mirrorStatus) (mirrorStatus, error)
	GetMirrorStatus(workerID, mirrorID string) (mirrorStatus, error)
	ListMirrorStatus(workerID string) ([]mirrorStatus, error)
	ListAllMirrorStatus() ([]mirrorStatus, error)
	Close() error
}

func makeDBAdapter(dbType string, dbFile string) (dbAdapter, error) {
	if dbType == "bolt" {
		innerDB, err := bolt.Open(dbFile, 0600, nil)
		if err != nil {
			return nil, err
		}
		db := boltAdapter{
			db:     innerDB,
			dbFile: dbFile,
		}
		return &db, nil
	}
	// unsupported db-type
	return nil, fmt.Errorf("unsupported db-type: %s", dbType)
}

type boltAdapter struct {
	db     *bolt.DB
	dbFile string
}

func (b *boltAdapter) ListWorkers() ([]worker, error) {
	return []worker{}, nil
}

func (b *boltAdapter) GetWorker(workerID string) (worker, error) {
	return worker{}, nil
}

func (b *boltAdapter) CreateWorker(w worker) (worker, error) {
	return worker{}, nil
}

func (b *boltAdapter) UpdateMirrorStatus(workerID, mirrorID string, status mirrorStatus) (mirrorStatus, error) {
	return mirrorStatus{}, nil
}

func (b *boltAdapter) GetMirrorStatus(workerID, mirrorID string) (mirrorStatus, error) {
	return mirrorStatus{}, nil
}

func (b *boltAdapter) ListMirrorStatus(workerID string) ([]mirrorStatus, error) {
	return []mirrorStatus{}, nil
}

func (b *boltAdapter) ListAllMirrorStatus() ([]mirrorStatus, error) {
	return []mirrorStatus{}, nil
}

func (b *boltAdapter) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
