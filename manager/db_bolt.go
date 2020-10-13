package manager

import (
	"fmt"

	"github.com/boltdb/bolt"
)

type boltAdapter struct {
	db     *bolt.DB
	dbFile string
}

func (b *boltAdapter) InitBucket(bucket string) (err error) {
	return b.db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return fmt.Errorf("create bucket %s error: %s", _workerBucketKey, err.Error())
		}
		return nil
	})
}

func (b *boltAdapter) Get(bucket string, key string) (v []byte, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		v = bucket.Get([]byte(key))
		return nil
	})
	return
}

func (b *boltAdapter) GetAll(bucket string) (m map[string][]byte, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		c := bucket.Cursor()
		m = make(map[string][]byte)
		for k, v := c.First(); k != nil; k, v = c.Next() {
			m[string(k)] = v
		}
		return nil
	})
	return
}

func (b *boltAdapter) Put(bucket string, key string, value []byte) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		err := bucket.Put([]byte(key), value)
		return err
	})
	return err
}

func (b *boltAdapter) Delete(bucket string, key string) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		err := bucket.Delete([]byte(key))
		return err
	})
	return err
}

func (b *boltAdapter) Close() error {
	return b.db.Close()
}
