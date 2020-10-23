package manager

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// implement kv interface backed by leveldb
type leveldbAdapter struct {
	db *leveldb.DB
}

func (b *leveldbAdapter) InitBucket(bucket string) (err error) {
	// no-op
	return
}

func (b *leveldbAdapter) Get(bucket string, key string) (v []byte, err error) {
	v, err = b.db.Get([]byte(bucket+key), nil)
	return
}

func (b *leveldbAdapter) GetAll(bucket string) (m map[string][]byte, err error) {
	it := b.db.NewIterator(util.BytesPrefix([]byte(bucket)), nil)
	defer it.Release()
	m = make(map[string][]byte)
	for it.Next() {
		k := string(it.Key())
		actualKey := k[len(bucket):]
		// it.Value() changes on next iteration
		val := it.Value()
		v := make([]byte, len(val))
		copy(v, val)
		m[actualKey] = v
	}
	return
}

func (b *leveldbAdapter) Put(bucket string, key string, value []byte) error {
	err := b.db.Put([]byte(bucket+key), []byte(value), nil)
	return err
}

func (b *leveldbAdapter) Delete(bucket string, key string) error {
	err := b.db.Delete([]byte(bucket+key), nil)
	return err
}

func (b *leveldbAdapter) Close() error {
	return b.db.Close()
}
