package manager

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "github.com/boltdb/bolt"
	"github.com/dgraph-io/badger/v2"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"

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

// interface for a kv database
type kvAdapter interface {
	InitBucket(bucket string) error
	Get(bucket string, key string) ([]byte, error)
	GetAll(bucket string) (map[string][]byte, error)
	Put(bucket string, key string, value []byte) error
	Delete(bucket string, key string) error
	Close() error
}

const (
	_workerBucketKey = "workers"
	_statusBucketKey = "mirror_status"
)

func makeDBAdapter(dbType string, dbFile string) (dbAdapter, error) {
	if dbType == "bolt" {
		innerDB, err := bolt.Open(dbFile, 0600, &bolt.Options{
			Timeout: 5 * time.Second,
		})
		if err != nil {
			return nil, err
		}
		db := boltAdapter{
			db: innerDB,
		}
		kv := kvDBAdapter{
			db: &db,
		}
		err = kv.Init()
		return &kv, err
	} else if dbType == "redis" {
		opt, err := redis.ParseURL(dbFile)
		if err != nil {
			return nil, fmt.Errorf("bad redis url: %s", err)
		}
		innerDB := redis.NewClient(opt)
		db := redisAdapter{
			db: innerDB,
		}
		kv := kvDBAdapter{
			db: &db,
		}
		err = kv.Init()
		return &kv, err
	} else if dbType == "badger" {
		innerDB, err := badger.Open(badger.DefaultOptions(dbFile))
		if err != nil {
			return nil, err
		}
		db := badgerAdapter{
			db: innerDB,
		}
		kv := kvDBAdapter{
			db: &db,
		}
		err = kv.Init()
		return &kv, err
	} else if dbType == "leveldb" {
		innerDB, err := leveldb.OpenFile(dbFile, nil)
		if err != nil {
			return nil, err
		}
		db := leveldbAdapter{
			db: innerDB,
		}
		kv := kvDBAdapter{
			db: &db,
		}
		err = kv.Init()
		return &kv, err
	}
	// unsupported db-type
	return nil, fmt.Errorf("unsupported db-type: %s", dbType)
}

// use the underlying kv database to store data
type kvDBAdapter struct {
	db kvAdapter
}

func (b *kvDBAdapter) Init() error {
	err := b.db.InitBucket(_workerBucketKey)
	if err != nil {
		return fmt.Errorf("create bucket %s error: %s", _workerBucketKey, err.Error())
	}
	err = b.db.InitBucket(_statusBucketKey)
	if err != nil {
		return fmt.Errorf("create bucket %s error: %s", _workerBucketKey, err.Error())
	}
	return err
}

func (b *kvDBAdapter) ListWorkers() (ws []WorkerStatus, err error) {
	var workers map[string][]byte
	workers, err = b.db.GetAll(_workerBucketKey)

	var w WorkerStatus
	for _, v := range workers {
		jsonErr := json.Unmarshal(v, &w)
		if jsonErr != nil {
			err = errors.Wrap(err, jsonErr.Error())
			continue
		}
		ws = append(ws, w)
	}
	return
}

func (b *kvDBAdapter) GetWorker(workerID string) (w WorkerStatus, err error) {
	var v []byte
	v, _ = b.db.Get(_workerBucketKey, workerID)
	if v == nil {
		err = fmt.Errorf("invalid workerID %s", workerID)
	} else {
		err = json.Unmarshal(v, &w)
	}
	return
}

func (b *kvDBAdapter) DeleteWorker(workerID string) error {
	v, _ := b.db.Get(_workerBucketKey, workerID)
	if v == nil {
		return fmt.Errorf("invalid workerID %s", workerID)
	}
	return b.db.Delete(_workerBucketKey, workerID)
}

func (b *kvDBAdapter) CreateWorker(w WorkerStatus) (WorkerStatus, error) {
	v, err := json.Marshal(w)
	if err == nil {
		err = b.db.Put(_workerBucketKey, w.ID, v)
	}
	return w, err
}

func (b *kvDBAdapter) RefreshWorker(workerID string) (w WorkerStatus, err error) {
	w, err = b.GetWorker(workerID)
	if err == nil {
		w.LastOnline = time.Now()
		w, err = b.CreateWorker(w)
	}
	return w, err
}

func (b *kvDBAdapter) UpdateMirrorStatus(workerID, mirrorID string, status MirrorStatus) (MirrorStatus, error) {
	id := mirrorID + "/" + workerID
	v, err := json.Marshal(status)
	if err == nil {
		err = b.db.Put(_statusBucketKey, id, v)
	}
	return status, err
}

func (b *kvDBAdapter) GetMirrorStatus(workerID, mirrorID string) (m MirrorStatus, err error) {
	id := mirrorID + "/" + workerID
	var v []byte
	v, err = b.db.Get(_statusBucketKey, id)
	if v == nil {
		err = fmt.Errorf("no mirror '%s' exists in worker '%s'", mirrorID, workerID)
	} else if err == nil {
		err = json.Unmarshal(v, &m)
	}
	return
}

func (b *kvDBAdapter) ListMirrorStatus(workerID string) (ms []MirrorStatus, err error) {
	var vals map[string][]byte
	vals, err = b.db.GetAll(_statusBucketKey)
	if err != nil {
		return
	}

	for k, v := range vals {
		if wID := strings.Split(k, "/")[1]; wID == workerID {
			var m MirrorStatus
			jsonErr := json.Unmarshal(v, &m)
			if jsonErr != nil {
				err = errors.Wrap(err, jsonErr.Error())
				continue
			}
			ms = append(ms, m)
		}
	}
	return
}

func (b *kvDBAdapter) ListAllMirrorStatus() (ms []MirrorStatus, err error) {
	var vals map[string][]byte
	vals, err = b.db.GetAll(_statusBucketKey)
	if err != nil {
		return
	}

	for _, v := range vals {
		var m MirrorStatus
		jsonErr := json.Unmarshal(v, &m)
		if jsonErr != nil {
			err = errors.Wrap(err, jsonErr.Error())
			continue
		}
		ms = append(ms, m)
	}
	return
}

func (b *kvDBAdapter) FlushDisabledJobs() (err error) {
	var vals map[string][]byte
	vals, err = b.db.GetAll(_statusBucketKey)
	if err != nil {
		return
	}

	for k, v := range vals {
		var m MirrorStatus
		jsonErr := json.Unmarshal(v, &m)
		if jsonErr != nil {
			err = errors.Wrap(err, jsonErr.Error())
			continue
		}
		if m.Status == Disabled || len(m.Name) == 0 {
			deleteErr := b.db.Delete(_statusBucketKey, k)
			if deleteErr != nil {
				err = errors.Wrap(err, deleteErr.Error())
			}
		}
	}
	return
}

func (b *kvDBAdapter) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
