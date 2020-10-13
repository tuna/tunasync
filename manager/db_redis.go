package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	. "github.com/tuna/tunasync/internal"
)

type redisAdapter struct {
	db *redis.Client
}

var ctx = context.Background()

func (b *redisAdapter) Init() (err error) {
	return nil
}

func (b *redisAdapter) ListWorkers() (ws []WorkerStatus, err error) {
	var val map[string]string
	val, err = b.db.HGetAll(ctx, _workerBucketKey).Result()
	if err == nil {
		var w WorkerStatus
		for _, v := range val {
			jsonErr := json.Unmarshal([]byte(v), &w)
			if jsonErr != nil {
				err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
				continue
			}
			ws = append(ws, w)
		}
	}
	return
}

func (b *redisAdapter) GetWorker(workerID string) (w WorkerStatus, err error) {
	var val string
	val, err = b.db.HGet(ctx, _workerBucketKey, workerID).Result()
	if err == nil {
		err = json.Unmarshal([]byte(val), &w)
	} else {
		err = fmt.Errorf("invalid workerID %s", workerID)
	}
	return
}

func (b *redisAdapter) DeleteWorker(workerID string) (err error) {
	_, err = b.db.HDel(ctx, _workerBucketKey, workerID).Result()
	if err != nil {
		err = fmt.Errorf("invalid workerID %s", workerID)
	}
	return
}

func (b *redisAdapter) CreateWorker(w WorkerStatus) (WorkerStatus, error) {
	var v []byte
	v, err := json.Marshal(w)
	if err == nil {
		_, err = b.db.HSet(ctx, _workerBucketKey, w.ID, string(v)).Result()
	}
	return w, err
}

func (b *redisAdapter) RefreshWorker(workerID string) (w WorkerStatus, err error) {
	w, err = b.GetWorker(workerID)
	if err == nil {
		w.LastOnline = time.Now()
		w, err = b.CreateWorker(w)
	}
	return w, err
}

func (b *redisAdapter) UpdateMirrorStatus(workerID, mirrorID string, status MirrorStatus) (MirrorStatus, error) {
	id := mirrorID + "/" + workerID
	v, err := json.Marshal(status)
	if err == nil {
		_, err = b.db.HSet(ctx, _statusBucketKey, id, string(v)).Result()
	}
	return status, err
}

func (b *redisAdapter) GetMirrorStatus(workerID, mirrorID string) (m MirrorStatus, err error) {
	id := mirrorID + "/" + workerID
	var val string
	val, err = b.db.HGet(ctx, _statusBucketKey, id).Result()
	if err == nil {
		err = json.Unmarshal([]byte(val), &m)
	} else {
		err = fmt.Errorf("no mirror '%s' exists in worker '%s'", mirrorID, workerID)
	}
	return
}

func (b *redisAdapter) ListMirrorStatus(workerID string) (ms []MirrorStatus, err error) {
	var val map[string]string
	val, err = b.db.HGetAll(ctx, _statusBucketKey).Result()
	if err == nil {
		var m MirrorStatus
		for k, v := range val {
			if wID := strings.Split(string(k), "/")[1]; wID == workerID {
				jsonErr := json.Unmarshal([]byte(v), &m)
				if jsonErr != nil {
					err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
					continue
				}
				ms = append(ms, m)
			}
		}
	}
	return
}

func (b *redisAdapter) ListAllMirrorStatus() (ms []MirrorStatus, err error) {
	var val map[string]string
	val, err = b.db.HGetAll(ctx, _statusBucketKey).Result()
	if err == nil {
		var m MirrorStatus
		for _, v := range val {
			jsonErr := json.Unmarshal([]byte(v), &m)
			if jsonErr != nil {
				err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
				continue
			}
			ms = append(ms, m)
		}
	}
	return
}

func (b *redisAdapter) FlushDisabledJobs() (err error) {
	var val map[string]string
	val, err = b.db.HGetAll(ctx, _statusBucketKey).Result()
	if err == nil {
		var m MirrorStatus
		for k, v := range val {
			jsonErr := json.Unmarshal([]byte(v), &m)
			if jsonErr != nil {
				err = fmt.Errorf("%s; %s", err.Error(), jsonErr)
				continue
			}
			if m.Status == Disabled || len(m.Name) == 0 {
				_, err = b.db.HDel(ctx, _statusBucketKey, k).Result()
			}
		}
	}
	return
}

func (b *redisAdapter) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
