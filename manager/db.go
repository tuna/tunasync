package manager

import "github.com/boltdb/bolt"

type dbAdapter interface {
	GetWorker(workerID string)
	UpdateMirrorStatus(workerID, mirrorID string, status mirrorStatus)
	GetMirrorStatus(workerID, mirrorID string)
	GetMirrorStatusList(workerID string)
	Close()
}

type boltAdapter struct {
	db     *bolt.DB
	dbFile string
}

func (b *boltAdapter) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
