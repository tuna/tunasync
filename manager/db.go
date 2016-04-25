package manager

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
)

type dbAdapter interface {
	Init() error
	ListWorkers() ([]workerStatus, error)
	GetWorker(workerID string) (workerStatus, error)
	CreateWorker(w workerStatus) (workerStatus, error)
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
		err = db.Init()
		return &db, err
	}
	// unsupported db-type
	return nil, fmt.Errorf("unsupported db-type: %s", dbType)
}

const (
	_workerBucketKey = "workers"
	_statusBucketKey = "mirror_status"
)

type boltAdapter struct {
	db     *bolt.DB
	dbFile string
}

func (b *boltAdapter) Init() (err error) {
	return b.db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(_workerBucketKey))
		if err != nil {
			return fmt.Errorf("create bucket %s error: %s", _workerBucketKey, err.Error())
		}
		_, err = tx.CreateBucketIfNotExists([]byte(_statusBucketKey))
		if err != nil {
			return fmt.Errorf("create bucket %s error: %s", _statusBucketKey, err.Error())
		}
		return nil
	})
}

func (b *boltAdapter) ListWorkers() (ws []workerStatus, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_workerBucketKey))
		c := bucket.Cursor()
		var w workerStatus
		for k, v := c.First(); k != nil; k, v = c.Next() {
			jsonErr := json.Unmarshal(v, &w)
			if jsonErr != nil {
				err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
				continue
			}
			ws = append(ws, w)
		}
		return err
	})
	return
}

func (b *boltAdapter) GetWorker(workerID string) (w workerStatus, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_workerBucketKey))
		v := bucket.Get([]byte(workerID))
		if v == nil {
			return fmt.Errorf("invalid workerID %s", workerID)
		}
		err := json.Unmarshal(v, &w)
		return err
	})
	return
}

func (b *boltAdapter) CreateWorker(w workerStatus) (workerStatus, error) {
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_workerBucketKey))
		v, err := json.Marshal(w)
		if err != nil {
			return err
		}
		err = bucket.Put([]byte(w.ID), v)
		return err
	})
	return w, err
}

func (b *boltAdapter) UpdateMirrorStatus(workerID, mirrorID string, status mirrorStatus) (mirrorStatus, error) {
	id := mirrorID + "/" + workerID
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_statusBucketKey))
		v, err := json.Marshal(status)
		err = bucket.Put([]byte(id), v)
		return err
	})
	return status, err
}

func (b *boltAdapter) GetMirrorStatus(workerID, mirrorID string) (m mirrorStatus, err error) {
	id := mirrorID + "/" + workerID
	err = b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_statusBucketKey))
		v := bucket.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("no mirror %s exists in worker %s", mirrorID, workerID)
		}
		err := json.Unmarshal(v, &m)
		return err
	})
	return
}

func (b *boltAdapter) ListMirrorStatus(workerID string) (ms []mirrorStatus, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_statusBucketKey))
		c := bucket.Cursor()
		var m mirrorStatus
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if wID := strings.Split(string(k), "/")[1]; wID == workerID {
				jsonErr := json.Unmarshal(v, &m)
				if jsonErr != nil {
					err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
					continue
				}
				ms = append(ms, m)
			}
		}
		return err
	})
	return
}

func (b *boltAdapter) ListAllMirrorStatus() (ms []mirrorStatus, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(_statusBucketKey))
		c := bucket.Cursor()
		var m mirrorStatus
		for k, v := c.First(); k != nil; k, v = c.Next() {
			jsonErr := json.Unmarshal(v, &m)
			if jsonErr != nil {
				err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
				continue
			}
			ms = append(ms, m)
		}
		return err
	})
	return
}

func (b *boltAdapter) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
