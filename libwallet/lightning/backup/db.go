package backup

import (
	"encoding/binary"

	bolt "go.etcd.io/bbolt"
)

const (
	backupBucket = "backup"
)

type backupDB struct {
	db *bolt.DB
}

func openDB(path string) (*backupDB, error) {
	database, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = database.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(backupBucket))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &backupDB{db: database}, nil
}

func (d *backupDB) close() error {
	return d.db.Close()
}

var (
	markIDKey        = []byte("lastBackupMarkID")
	useEncryptionKey = []byte("useEncryption")
)

// AddBackupRequest is used to mark a need for a backup before actually executing it.
// This will allow to recover in case of backup failure.
func (d *backupDB) addBackupRequest() (err error) {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(backupBucket))
		backupSeq, err := b.NextSequence()
		if err != nil {
			return err
		}
		return b.Put(markIDKey, itob(backupSeq))
	})
}

// LastBackupRequest returns the pending backup id if exists, zero if doesn't.
func (d *backupDB) lastBackupRequest() (uint64, error) {
	var pendingRequest []byte
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(backupBucket))
		pendingRequest = b.Get(markIDKey)
		return nil
	})
	if err != nil || pendingRequest == nil {
		return 0, err
	}
	return btoi(pendingRequest), nil
}

// MarkBackupRequestCompleted is used to signal that the backup has completed and that the
// corresponding requestID can be removed.
func (d *backupDB) markBackupRequestCompleted(requestID uint64) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(backupBucket))
		pendingRequest := b.Get(markIDKey)
		if pendingRequest == nil || requestID != btoi(pendingRequest) {
			return nil
		}
		return b.Delete(markIDKey)
	})
}

func (d *backupDB) setUseEncryption(use bool) error {
	var encrypt byte
	if use {
		encrypt = 1
	}
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(backupBucket))
		return b.Put(useEncryptionKey, []byte{encrypt})
	})
}

func (d *backupDB) useEncryption() (bool, error) {
	var useEncryption bool
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(backupBucket))
		useEncryptionBytes := b.Get(markIDKey)
		useEncryption = len(useEncryptionBytes) == 1 && useEncryptionBytes[0] == 1
		return nil
	})
	if err != nil {
		return false, err
	}

	return useEncryption, nil
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func btoi(bytes []byte) uint64 {
	return binary.BigEndian.Uint64(bytes)
}
