package walletdata

import (
	"io"

	"github.com/btcsuite/btcwallet/walletdb"
)

// Enforce db implements the walletdb.Db interface.
var _ walletdb.DB = (*DB)(nil)

// BeginReadTx opens a database read transaction.
func (db *DB) BeginReadTx() (walletdb.ReadTx, error) {
	return db, nil
}

// BeginReadWriteTx opens a database read+write transaction.
func (db *DB) BeginReadWriteTx() (walletdb.ReadWriteTx, error) {
	return db, nil
}

// Copy writes a copy of the database to the provided writer.  This
// call will start a read-only transaction to perform all operations.
func (db *DB) Copy(w io.Writer) error {
	return nil
}

// Close cleanly shuts down the database and syncs all data.
func (db *DB) Close() error {
	return db.close()
}

// PrintStats returns all collected stats pretty printed into a string.
func (db *DB) PrintStats() string {
	return ""
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
	// db.Fin
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
	return nil
}

type transaction struct{}

// ReadBucket opens the root bucket for read only access.  If the bucket
// described by the key does not exist, nil is returned.
func (db *DB) ReadBucket(key []byte) walletdb.ReadBucket {
	return nil
}

// ForEachBucket will iterate through all top level buckets.
func (db *DB) ForEachBucket(func(key []byte) error) error {
	return nil
}

// Rollback closes the transaction, discarding changes (if any) if the
// database was modified by a write transaction.
func (db *DB) Rollback() error {
	return nil
}

// ReadWriteBucket opens the root bucket for read/write access.  If the
// bucket described by the key does not exist, nil is returned.
func (db *DB) ReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	return nil
}

// CreateTopLevelBucket creates the top level bucket for a key if it
// does not exist.  The newly-created bucket it returned.
func (db *DB) CreateTopLevelBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	return nil, nil
}

// DeleteTopLevelBucket deletes the top level bucket for a key.  This
// errors if the bucket can not be found or the key keys a single value
// instead of a bucket.
func (db *DB) DeleteTopLevelBucket(key []byte) error {
	return nil
}

// Commit commits all changes that have been on the transaction's root
// buckets and all of their sub-buckets to persistent storage.
func (db *DB) Commit() error {
	return nil
}

// OnCommit takes a function closure that will be executed when the
// transaction successfully gets committed.
func (db *DB) OnCommit(func()) {
}
