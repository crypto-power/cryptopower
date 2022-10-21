package walletdata

import (
	"io"

	"github.com/btcsuite/btcwallet/walletdb"
	"go.etcd.io/bbolt"
)

// Enforce db implements the walletdb.Db interface.
var _ walletdb.DB = (*DB)(nil)

func (db *DB) beginTx(writable bool) (*transaction, error) {
	boltTx, err := db.walletDataDB.Bolt.Begin(writable)
	if err != nil {
		return nil, err
	}
	return &transaction{boltTx: boltTx}, nil
}

// BeginReadTx opens a database read transaction.
func (db *DB) BeginReadTx() (walletdb.ReadTx, error) {
	return db.beginTx(false)
}

// BeginReadWriteTx opens a database read+write transaction.
func (db *DB) BeginReadWriteTx() (walletdb.ReadWriteTx, error) {
	return db.beginTx(true)
}

// Copy writes a copy of the database to the provided writer.  This
// call will start a read-only transaction to perform all operations.
func (db *DB) Copy(w io.Writer) error {
	return db.walletDataDB.Bolt.View(func(tx *bbolt.Tx) error {
		return tx.Copy(w)
	})
}

// Close cleanly shuts down the database and syncs all data.
func (db *DB) Close() error {
	return db.walletDataDB.Bolt.Close()
}

// PrintStats returns all collected stats pretty printed into a string.
func (db *DB) PrintStats() string {
	return "---"
}

// View opens a database read transaction and executes the function f
// with the transaction passed as a parameter. After f exits, the
// transaction is rolled back. If f errors, its error is returned, not a
// rollback error (if any occur). The passed reset function is called
// before the start of the transaction and can be used to reset
// intermediate state. As callers may expect retries of the f closure
// (depending on the database backend used), the reset function will be
// called before each retry respectively.
//
// NOTE: For new code, this method should be used directly instead of
// the package level View() function.
func (db *DB) View(f func(tx walletdb.ReadTx) error, reset func()) error {
	// We don't do any retries with bolt so we just initially call the reset
	// function once.
	reset()

	tx, err := db.BeginReadTx()
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	rollbackErr := tx.Rollback()
	if err != nil {
		return err
	}

	if rollbackErr != nil {
		return rollbackErr
	}
	return nil
}

// Update opens a database read/write transaction and executes the
// function f with the transaction passed as a parameter. After f exits,
// if f did not error, the transaction is committed. Otherwise, if f did
// error, the transaction is rolled back. If the rollback fails, the
// original error returned by f is still returned. If the commit fails,
// the commit error is returned. As callers may expect retries of the f
// closure (depending on the database backend used), the reset function
// will be called before each retry respectively.
//
// NOTE: For new code, this method should be used directly instead of
// the package level Update() function.
func (db *DB) Update(f func(tx walletdb.ReadWriteTx) error, reset func()) error {
	// We don't do any retries with bolt so we just initially call the reset
	// function once.
	reset()

	tx, err := db.BeginReadWriteTx()
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	if err != nil {
		// Want to return the original error, not a rollback error if
		// any occur.
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

type transaction struct {
	boltTx *bbolt.Tx
}

var _ walletdb.ReadWriteTx = (*transaction)(nil)

// ReadBucket opens the root bucket for read only access.  If the bucket
// described by the key does not exist, nil is returned.
func (tx *transaction) ReadBucket(key []byte) walletdb.ReadBucket {
	return tx.ReadWriteBucket(key)
}

// ForEachBucket will iterate through all top level buckets.
func (tx *transaction) ForEachBucket(fn func(key []byte) error) error {
	return tx.boltTx.ForEach(
		func(name []byte, _ *bbolt.Bucket) error {
			return fn(name)
		},
	)
}

// Rollback closes the transaction, discarding changes (if any) if the
// database was modified by a write transaction.
func (tx *transaction) Rollback() error {
	return tx.boltTx.Rollback()
}

// ReadWriteBucket opens the root bucket for read/write access.  If the
// bucket described by the key does not exist, nil is returned.
func (tx *transaction) ReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	boltBucket := tx.boltTx.Bucket(key)
	if boltBucket == nil {
		return nil
	}
	b := &bucket{upstream: boltBucket}
	// fmt.Println("NEsted Bucket ", nil == b.NestedReadBucket(key))
	return b
}

// CreateTopLevelBucket creates the top level bucket for a key if it
// does not exist.  The newly-created bucket it returned.
func (tx *transaction) CreateTopLevelBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := tx.boltTx.CreateBucketIfNotExists(key)
	if err != nil {
		return nil, err
	}
	return &bucket{upstream: boltBucket}, nil
}

// DeleteTopLevelBucket deletes the top level bucket for a key.  This
// errors if the bucket can not be found or the key keys a single value
// instead of a bucket.
func (tx *transaction) DeleteTopLevelBucket(key []byte) error {
	err := tx.boltTx.DeleteBucket(key)
	if err != nil {
		return err
	}
	return nil
}

// Commit commits all changes that have been on the transaction's root
// buckets and all of their sub-buckets to persistent storage.
func (tx *transaction) Commit() error {
	return tx.boltTx.Commit()
}

// OnCommit takes a function closure that will be executed when the
// transaction successfully gets committed.
func (tx *transaction) OnCommit(f func()) {
	tx.boltTx.OnCommit(f)
}

type bucket struct {
	upstream *bbolt.Bucket
}

// Verify that bucket implements walletdb.ReadWriteBucket interface.
var _ walletdb.ReadWriteBucket = (*bucket)(nil)

// NestedReadBucket retrieves a nested bucket with the given key.
// Returns nil if the bucket does not exist.
func (b *bucket) NestedReadBucket(key []byte) walletdb.ReadBucket {
	return b.NestedReadWriteBucket(key)
}

// ForEach invokes the passed function with every key/value pair in
// the bucket.  This includes nested buckets, in which case the value
// is nil, but it does not include the key/value pairs within those
// nested buckets.
//
// NOTE: The values returned by this function are only valid during a
// transaction.  Attempting to access them after a transaction has ended
// results in undefined behavior.  This constraint prevents additional
// data copies and allows support for memory-mapped database
// implementations.
func (b *bucket) ForEach(fn func(k, v []byte) error) error {
	return b.upstream.ForEach(fn)
}

// Get returns the value for the given key.  Returns nil if the key does
// not exist in this bucket (or nested buckets).
//
// NOTE: The value returned by this function is only valid during a
// transaction.  Attempting to access it after a transaction has ended
// results in undefined behavior.  This constraint prevents additional
// data copies and allows support for memory-mapped database
// implementations.
func (b *bucket) Get(key []byte) []byte {
	return b.upstream.Get(key)
}

// NestedReadWriteBucket retrieves a nested bucket with the given key.
// Returns nil if the bucket does not exist.
func (b *bucket) NestedReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	boltBucket := b.upstream.Bucket(key)
	// Don't return a non-nil interface to a nil pointer.
	if boltBucket == nil {
		return nil
	}
	return &bucket{upstream: boltBucket}
}

// CreateBucket creates and returns a new nested bucket with the given
// key.  Returns ErrBucketExists if the bucket already exists,
// ErrBucketNameRequired if the key is empty, or ErrIncompatibleValue
// if the key value is otherwise invalid for the particular database
// implementation.  Other errors are possible depending on the
// implementation.
func (b *bucket) CreateBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := b.upstream.CreateBucket(key)
	if err != nil {
		return nil, err
	}
	return &bucket{upstream: boltBucket}, nil
}

// CreateBucketIfNotExists creates and returns a new nested bucket with
// the given key if it does not already exist.  Returns
// ErrBucketNameRequired if the key is empty or ErrIncompatibleValue
// if the key value is otherwise invalid for the particular database
// backend.  Other errors are possible depending on the implementation.
func (b *bucket) CreateBucketIfNotExists(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := b.upstream.CreateBucketIfNotExists(key)
	if err != nil {
		return nil, err
	}
	return &bucket{upstream: boltBucket}, nil
}

// DeleteNestedBucket removes a nested bucket with the given key.
// Returns ErrTxNotWritable if attempted against a read-only transaction
// and ErrBucketNotFound if the specified bucket does not exist.
func (b *bucket) DeleteNestedBucket(key []byte) error {
	return b.upstream.DeleteBucket(key)
}

// Put saves the specified key/value pair to the bucket.  Keys that do
// not already exist are added and keys that already exist are
// overwritten.  Returns ErrTxNotWritable if attempted against a
// read-only transaction.
func (b *bucket) Put(key, value []byte) error {
	return b.upstream.Put(key, value)
}

// Delete removes the specified key from the bucket.  Deleting a key
// that does not exist does not return an error.  Returns
// ErrTxNotWritable if attempted against a read-only transaction.
func (b *bucket) Delete(key []byte) error {
	return b.upstream.Delete(key)
}

func (b *bucket) ReadCursor() walletdb.ReadCursor {
	return b.ReadWriteCursor()
}

// ReadWriteCursor returns a new cursor, allowing for iteration over the
// bucket's key/value pairs and nested buckets in forward or backward
// order.
func (b *bucket) ReadWriteCursor() walletdb.ReadWriteCursor {
	return b.upstream.Cursor()
}

// Tx returns the bucket's transaction.
func (b *bucket) Tx() walletdb.ReadWriteTx {
	return &transaction{
		b.upstream.Tx(),
	}
}

// NextSequence returns an autoincrementing integer for the bucket.
func (b *bucket) NextSequence() (uint64, error) {
	return b.upstream.NextSequence()
}

// SetSequence updates the sequence number for the bucket.
func (b *bucket) SetSequence(v uint64) error {
	return b.upstream.SetSequence(v)
}

// Sequence returns the current integer for the bucket without
// incrementing it.
func (b *bucket) Sequence() uint64 {
	return b.upstream.Sequence()
}

// type cursor bbolt.Cursor

// // First positions the cursor at the first key/value pair and returns
// // the pair.
// func (c *cursor) First() (key, value []byte) {
// 	return (*bbolt.Cursor)(c).First()
// }

// // Last positions the cursor at the last key/value pair and returns the
// // pair.
// func (c *cursor) Last() (key, value []byte) {
// 	return (*bbolt.Cursor)(c).Last()
// }

// // Next moves the cursor one key/value pair forward and returns the new
// // pair.
// func (c *cursor) Next() (key, value []byte) {
// 	return (*bbolt.Cursor)(c).Prev()
// }

// // Prev moves the cursor one key/value pair backward and returns the new
// // pair.
// func (c *cursor) Prev() (key, value []byte) {
// 	return (*bbolt.Cursor)(c).Prev()
// }

// // Seek positions the cursor at the passed seek key.  If the key does
// // not exist, the cursor is moved to the next key after seek.  Returns
// // the new pair.
// func (c *cursor) Seek(seek []byte) (key, value []byte) {
// 	return (*bbolt.Cursor)(c).Seek(seek)
// }

// // Delete removes the current key/value pair the cursor is at without
// // invalidating the cursor.  Returns ErrIncompatibleValue if attempted
// // when the cursor points to a nested bucket.
// func (c *cursor) Delete() error {
// 	return (*bbolt.Cursor)(c).Delete()
// }
