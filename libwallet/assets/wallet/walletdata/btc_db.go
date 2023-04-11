package walletdata

import (
	"io"

	"github.com/btcsuite/btcwallet/walletdb"
	"go.etcd.io/bbolt"
)

type BTCDB struct {
	Bolt *bbolt.DB
}

// Enforce db implements the walletdb.Db interface.
var _ walletdb.DB = (*BTCDB)(nil)

func (db *BTCDB) beginTx(writable bool) (*BTCTX, error) {
	boltTx, err := db.Bolt.Begin(writable)
	if err != nil {
		return nil, err
	}
	return &BTCTX{boltTx: boltTx}, nil
}

// BeginReadTx opens a database read transaction.
func (db *BTCDB) BeginReadTx() (walletdb.ReadTx, error) {
	return db.beginTx(false)
}

// BeginReadWriteTx opens a database read+write transaction.
func (db *BTCDB) BeginReadWriteTx() (walletdb.ReadWriteTx, error) {
	return db.beginTx(true)
}

// Copy writes a copy of the database to the provided writer.  This
// call will start a read-only transaction to perform all operations.
func (db *BTCDB) Copy(w io.Writer) error {
	return db.Bolt.View(func(tx *bbolt.Tx) error {
		return tx.Copy(w)
	})
}

// Close cleanly shuts down the database and syncs all data.
func (db *BTCDB) Close() error {
	return db.Bolt.Close()
}

// PrintStats returns all collected stats pretty printed into a string.
func (db *BTCDB) PrintStats() string {
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
func (db *BTCDB) View(f func(tx walletdb.ReadTx) error, reset func()) error {
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
func (db *BTCDB) Update(f func(tx walletdb.ReadWriteTx) error, reset func()) error {
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

type BTCTX struct {
	boltTx *bbolt.Tx
}

var _ walletdb.ReadWriteTx = (*BTCTX)(nil)

// ReadBucket opens the root bucket for read only access.  If the bucket
// described by the key does not exist, nil is returned.
func (tx *BTCTX) ReadBucket(key []byte) walletdb.ReadBucket {
	return tx.ReadWriteBucket(key)
}

// ForEachBucket will iterate through all top level buckets.
func (tx *BTCTX) ForEachBucket(fn func(key []byte) error) error {
	return tx.boltTx.ForEach(
		func(name []byte, _ *bbolt.Bucket) error {
			return fn(name)
		},
	)
}

// Rollback closes the transaction, discarding changes (if any) if the
// database was modified by a write transaction.
func (tx *BTCTX) Rollback() error {
	return tx.boltTx.Rollback()
}

// ReadWriteBucket opens the root bucket for read/write access.  If the
// bucket described by the key does not exist, nil is returned.
func (tx *BTCTX) ReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	boltBucket := tx.boltTx.Bucket(key)
	if boltBucket == nil {
		return nil
	}
	return &BTCBucket{boltBucket: boltBucket}
}

// CreateTopLevelBucket creates the top level bucket for a key if it
// does not exist.  The newly-created bucket it returned.
func (tx *BTCTX) CreateTopLevelBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := tx.boltTx.CreateBucketIfNotExists(key)
	if err != nil {
		return nil, err
	}
	return &BTCBucket{boltBucket: boltBucket}, nil
}

// DeleteTopLevelBucket deletes the top level bucket for a key.  This
// errors if the bucket can not be found or the key keys a single value
// instead of a bucket.
func (tx *BTCTX) DeleteTopLevelBucket(key []byte) error {
	return tx.boltTx.DeleteBucket(key)
}

// Commit commits all changes that have been on the transaction's root
// buckets and all of their sub-buckets to persistent storage.
func (tx *BTCTX) Commit() error {
	return tx.boltTx.Commit()
}

// OnCommit takes a function closure that will be executed when the
// transaction successfully gets committed.
func (tx *BTCTX) OnCommit(f func()) {
	tx.boltTx.OnCommit(f)
}

type BTCBucket struct {
	boltBucket *bbolt.Bucket
}

// Verify that bucket implements walletdb.ReadWriteBucket interface.
var _ walletdb.ReadWriteBucket = (*BTCBucket)(nil)

// NestedReadBucket retrieves a nested bucket with the given key.
// Returns nil if the bucket does not exist.
func (b *BTCBucket) NestedReadBucket(key []byte) walletdb.ReadBucket {
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
func (b *BTCBucket) ForEach(fn func(k, v []byte) error) error {
	return b.boltBucket.ForEach(fn)
}

// Get returns the value for the given key.  Returns nil if the key does
// not exist in this bucket (or nested buckets).
//
// NOTE: The value returned by this function is only valid during a
// transaction.  Attempting to access it after a transaction has ended
// results in undefined behavior.  This constraint prevents additional
// data copies and allows support for memory-mapped database
// implementations.
func (b *BTCBucket) Get(key []byte) []byte {
	return b.boltBucket.Get(key)
}

// NestedReadWriteBucket retrieves a nested bucket with the given key.
// Returns nil if the bucket does not exist.
func (b *BTCBucket) NestedReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	boltBucket := b.boltBucket.Bucket(key)
	// Don't return a non-nil interface to a nil pointer.
	if boltBucket == nil {
		return nil
	}
	return &BTCBucket{boltBucket: boltBucket}
}

// CreateBucket creates and returns a new nested bucket with the given
// key.  Returns ErrBucketExists if the bucket already exists,
// ErrBucketNameRequired if the key is empty, or ErrIncompatibleValue
// if the key value is otherwise invalid for the particular database
// implementation.  Other errors are possible depending on the
// implementation.
func (b *BTCBucket) CreateBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := b.boltBucket.CreateBucket(key)
	if err != nil {
		return nil, err
	}
	return &BTCBucket{boltBucket: boltBucket}, nil
}

// CreateBucketIfNotExists creates and returns a new nested bucket with
// the given key if it does not already exist.  Returns
// ErrBucketNameRequired if the key is empty or ErrIncompatibleValue
// if the key value is otherwise invalid for the particular database
// backend.  Other errors are possible depending on the implementation.
func (b *BTCBucket) CreateBucketIfNotExists(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := b.boltBucket.CreateBucketIfNotExists(key)
	if err != nil {
		return nil, err
	}
	return &BTCBucket{boltBucket: boltBucket}, nil
}

// DeleteNestedBucket removes a nested bucket with the given key.
// Returns ErrTxNotWritable if attempted against a read-only transaction
// and ErrBucketNotFound if the specified bucket does not exist.
func (b *BTCBucket) DeleteNestedBucket(key []byte) error {
	return b.boltBucket.DeleteBucket(key)
}

// Put saves the specified key/value pair to the bucket.  Keys that do
// not already exist are added and keys that already exist are
// overwritten.  Returns ErrTxNotWritable if attempted against a
// read-only transaction.
func (b *BTCBucket) Put(key, value []byte) error {
	return b.boltBucket.Put(key, value)
}

// Delete removes the specified key from the bucket.  Deleting a key
// that does not exist does not return an error.  Returns
// ErrTxNotWritable if attempted against a read-only transaction.
func (b *BTCBucket) Delete(key []byte) error {
	return b.boltBucket.Delete(key)
}

func (b *BTCBucket) ReadCursor() walletdb.ReadCursor {
	return b.ReadWriteCursor()
}

// ReadWriteCursor returns a new cursor, allowing for iteration over the
// bucket's key/value pairs and nested buckets in forward or backward
// order.
func (b *BTCBucket) ReadWriteCursor() walletdb.ReadWriteCursor {
	return b.boltBucket.Cursor()
}

// Tx returns the bucket's transaction.
func (b *BTCBucket) Tx() walletdb.ReadWriteTx {
	return &BTCTX{
		b.boltBucket.Tx(),
	}
}

// NextSequence returns an autoincrementing integer for the bucket.
func (b *BTCBucket) NextSequence() (uint64, error) {
	return b.boltBucket.NextSequence()
}

// SetSequence updates the sequence number for the bucket.
func (b *BTCBucket) SetSequence(v uint64) error {
	return b.boltBucket.SetSequence(v)
}

// Sequence returns the current integer for the bucket without
// incrementing it.
func (b *BTCBucket) Sequence() uint64 {
	return b.boltBucket.Sequence()
}
