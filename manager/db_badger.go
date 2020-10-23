package manager

import (
	"github.com/dgraph-io/badger/v2"
)

// implement kv interface backed by badger
type badgerAdapter struct {
	db *badger.DB
}

func (b *badgerAdapter) InitBucket(bucket string) (err error) {
	// no-op
	return
}

func (b *badgerAdapter) Get(bucket string, key string) (v []byte, err error) {
	b.db.View(func(tx *badger.Txn) error {
		var item *badger.Item
		item, err = tx.Get([]byte(bucket + key))
		if item != nil {
			v, err = item.ValueCopy(nil)
		}
		return nil
	})
	return
}

func (b *badgerAdapter) GetAll(bucket string) (m map[string][]byte, err error) {
	b.db.View(func(tx *badger.Txn) error {
		it := tx.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(bucket)
		m = make(map[string][]byte)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := string(item.Key())
			actualKey := k[len(bucket):]

			var v []byte
			v, err = item.ValueCopy(nil)
			m[actualKey] = v
		}
		return nil
	})
	return
}

func (b *badgerAdapter) Put(bucket string, key string, value []byte) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		err := tx.Set([]byte(bucket+key), value)
		return err
	})
	return err
}

func (b *badgerAdapter) Delete(bucket string, key string) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		err := tx.Delete([]byte(bucket + key))
		return err
	})
	return err
}

func (b *badgerAdapter) Close() error {
	return b.db.Close()
}
