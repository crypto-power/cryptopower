package badgerdb

import (
	"bytes"

	"decred.org/dcrwallet/v2/errors"
	"github.com/dgraph-io/badger"
)

const (
	// Maximum length of a key, in bytes.
	maxKeySize = 65378

	// Holds an identifier for a bucket
	metaBucket = 5
)

// Bucket is an internal type used to represent a collection of key/value pairs
// and implements the walletdb Bucket interfaces.
type Bucket struct {
	prefix        []byte
	buckets       []*Bucket
	txn           *badger.Txn
	dbTransaction *transaction
}

// Cursor represents a cursor over key/value pairs and nested buckets of a
// bucket.
//
// Note that open cursors are not tracked on bucket changes and any
// modifications to the bucket, with the exception of cursor.Delete, invalidate
// the cursor. After invalidation, the cursor must be repositioned, or the keys
// and values returned may be unpredictable.
type Cursor struct {
	iterator        *badger.Iterator
	reverseIterator *badger.Iterator
	txn             *badger.Txn
	prefix          []byte
	ck              []byte
	dbTransaction   *transaction
}

func newBucket(tx *badger.Txn, badgerKey []byte, dbTx *transaction) (*Bucket, error) {
	prefix := make([]byte, len(badgerKey))
	copy(prefix, badgerKey)
	item, err := tx.Get(prefix)
	if err != nil {
		//Not Found
		if err == badger.ErrKeyNotFound {
			entry := badger.NewEntry(prefix, insertPrefixLength([]byte{}, len(prefix))).WithMeta(metaBucket)
			err = tx.SetEntry(entry)
			if err != nil {
				return nil, convertErr(err)
			}
			return &Bucket{txn: tx, prefix: prefix, dbTransaction: dbTx}, nil
		}
		return nil, convertErr(err)
	}
	if item.UserMeta() != metaBucket {
		errors.E(errors.Invalid, "key is not associated with a bucket")
	}
	return &Bucket{txn: tx, prefix: prefix, dbTransaction: dbTx}, nil
}

func insertPrefixLength(val []byte, length int) []byte {
	result := make([]byte, 0)
	prefixBytes := byte(length)
	result = append(result, prefixBytes)
	result = append(result, val...)
	return result
}

func addPrefix(prefix []byte, key []byte) ([]byte, error) {
	if len(key) > maxKeySize {
		return nil, errors.E(errors.Invalid, "key too long")
	}
	return append(prefix, key...), nil
}

// SetTx changes the transaction for bucket and sub buckets
func (b *Bucket) setTx(tx *badger.Txn) {
	b.txn = tx
	for _, bkt := range b.buckets {
		bkt.setTx(tx)
	}
}

func (b *Bucket) iterator() *badger.Iterator {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 100
	it := b.txn.NewIterator(opts)
	return it
}

func (b *Bucket) badgerCursor() *Cursor {
	reverseOptions := badger.DefaultIteratorOptions
	//Key-only iteration for faster search. Value gets fetched when item.Value() is called.
	reverseOptions.PrefetchValues = false
	reverseOptions.Reverse = true
	txn := b.dbTransaction.db.NewTransaction(false)
	reverseIterator := txn.NewIterator(reverseOptions)
	cursor := &Cursor{iterator: b.iterator(), reverseIterator: reverseIterator, txn: b.txn, prefix: b.prefix, dbTransaction: b.dbTransaction}
	return cursor
}

// Bucket returns a nested bucket which is created from the passed key
func (b *Bucket) bucket(key []byte, errorIfExists bool) (*Bucket, error) {
	if len(key) == 0 {
		//Empty Key
		return nil, errors.E(errors.Invalid, "key is empty")
	}
	keyPrefix, err := addPrefix(b.prefix, key)
	if err != nil {
		return nil, err
	}
	copiedKey := make([]byte, len(keyPrefix))
	copy(copiedKey, keyPrefix)
	item, err := b.txn.Get(copiedKey)
	if err != nil {
		//Key Not Found
		entry := badger.NewEntry(copiedKey, insertPrefixLength([]byte{}, len(b.prefix))).WithMeta(metaBucket)
		err = b.txn.SetEntry(entry)
		if err != nil {
			return nil, convertErr(err)
		}
		bucket := &Bucket{txn: b.txn, prefix: copiedKey, dbTransaction: b.dbTransaction}
		b.buckets = append(b.buckets, bucket)
		return bucket, nil
	}

	if item.UserMeta() == metaBucket {
		if errorIfExists {
			return nil, errors.E(errors.Exist, "bucket already exists")
		}

		bucket := &Bucket{txn: b.txn, prefix: copiedKey, dbTransaction: b.dbTransaction}
		b.buckets = append(b.buckets, bucket)
		return bucket, nil
	}

	return nil, errors.E(errors.Invalid, "key is not associated with a bucket")
}

// DropBucket deletes a bucket and all it's data
// from the database. Return nil if bucket does
// not exist, transaction is not writable or given
// key does not point to a bucket
func (b *Bucket) dropBucket(key []byte) error {
	if !b.dbTransaction.writable {
		return errors.E(errors.Invalid, "cannot delete nested bucket in a read-only transaction")
	}

	prefix, err := addPrefix(b.prefix, key)
	if err != nil {
		return err
	}

	item, err := b.txn.Get(prefix)
	if err != nil {
		return convertErr(err)
	}

	if item.UserMeta() != metaBucket {
		return errors.E(errors.Invalid, "key is not associated with a bucket")
	}

	iteratorTxn := b.dbTransaction.db.NewTransaction(true)
	it := iteratorTxn.NewIterator(badger.DefaultIteratorOptions)

	it.Rewind()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		if bytes.Equal(item.Key(), prefix) {
			continue
		}

		v, err := item.ValueCopy(nil)
		if err != nil {
			return convertErr(err)
		}

		prefixLength := int(v[0])
		if bytes.Equal(item.Key()[:prefixLength], prefix) {
		retryDelete:
			err = b.txn.Delete(item.KeyCopy(nil))
			if err != nil {
				if err == badger.ErrTxnTooBig {
					err = b.txn.Commit()
					if err != nil {
						return err
					}
					*b.txn = *b.dbTransaction.db.NewTransaction(true)
					goto retryDelete
				}
				return err
			}
		}
	}
	it.Close()
	iteratorTxn.Discard()

	err = b.txn.Commit()
	if err != nil {
		return convertErr(err)
	}

	*b.txn = *b.dbTransaction.db.NewTransaction(true)

	err = b.txn.Delete(item.Key()[:])
	if err != nil {
		return convertErr(err)
	}

	return nil
}

func (b *Bucket) get(key []byte) []byte {
	if len(key) == 0 {
		return nil
	}
	k, err := addPrefix(b.prefix, key)
	if err != nil {
		return nil
	}
	item, err := b.txn.Get(k)
	if err != nil {
		//Not found
		return nil
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil
	}
	return val[1:]
}

func (b *Bucket) put(key []byte, value []byte) error {
	if len(key) == 0 {
		return errors.E(errors.Invalid, "key is empty")
	} else if len(key) > maxKeySize {
		return errors.E(errors.Invalid, "key is too large")
	}
	copiedKey := make([]byte, len(key))
	copy(copiedKey, key[:])

	k, err := addPrefix(b.prefix, copiedKey)
	if err != nil {
		return err
	}
	err = b.txn.Set(k, insertPrefixLength(value[:], len(b.prefix)))

	return err
}

func (b *Bucket) delete(key []byte) error {
	if len(key) == 0 {
		return nil
	}

	k, err := addPrefix(b.prefix, key)
	if err != nil {
		return err
	}
	err = b.txn.Delete(k)
	if err == badger.ErrKeyNotFound {
		return nil
	}
	return err
}

func (b *Bucket) forEach(fn func(k, v []byte) error) error {
	txn := b.txn
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer func() {
		it.Close()
	}()
	prefix := b.prefix
	it.Rewind()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		k := item.Key()
		if bytes.Equal(item.Key(), prefix) {
			continue
		}

		v, err := item.ValueCopy(nil)
		if err != nil {
			return convertErr(err)
		}

		prefixLength := int(v[0])
		if bytes.Equal(item.Key()[:prefixLength], prefix) {
			if item.UserMeta() == metaBucket {
				if err := fn(k[prefixLength:], nil); err != nil {
					return err
				}
			} else {
				if err := fn(k[prefixLength:], v[1:]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
