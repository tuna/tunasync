package manager

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// implement kv interface backed by redis
type redisAdapter struct {
	db *redis.Client
}

var ctx = context.Background()

func (b *redisAdapter) InitBucket(bucket string) (err error) {
	// no-op
	return
}

func (b *redisAdapter) Get(bucket string, key string) (v []byte, err error) {
	var val string
	val, err = b.db.HGet(ctx, bucket, key).Result()
	if err == nil {
		v = []byte(val)
	}
	return
}

func (b *redisAdapter) GetAll(bucket string) (m map[string][]byte, err error) {
	var val map[string]string
	val, err = b.db.HGetAll(ctx, bucket).Result()
	if err == nil && val != nil {
		m = make(map[string][]byte)
		for k, v := range val {
			m[k] = []byte(v)
		}
	}
	return
}

func (b *redisAdapter) Put(bucket string, key string, value []byte) error {
	_, err := b.db.HSet(ctx, bucket, key, string(value)).Result()
	return err
}

func (b *redisAdapter) Delete(bucket string, key string) error {
	_, err := b.db.HDel(ctx, bucket, key).Result()
	return err
}

func (b *redisAdapter) Close() error {
	return b.db.Close()
}
