package doubleratchet

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	encryptedSessionsBucket = "encryptedSessions"
	sessionKeysBucket       = "sessionKeys"
	sessionContextBucket    = "sessionContext"
	sessionUserInfoBucket   = "sessionUserInfo"
)

var db *bolt.DB

func openDB(dbPath string) error {
	var err error
	db, err = bolt.Open(dbPath, 0600, nil)
	if err != nil {
		fmt.Printf("Failed to open database %v", err)
		return err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		var err error
		_, err = tx.CreateBucketIfNotExists([]byte(sessionKeysBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(sessionContextBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(sessionUserInfoBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(encryptedSessionsBucket))
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

func closeDB() error {
	return db.Close()
}

func destroyDB() error {
	return os.Remove(db.Path())
}

func saveEncryptedSession(sessionID []byte, sessionData []byte) error {
	return saveItem([]byte(encryptedSessionsBucket), sessionID, sessionData)
}

func fetchEncryptedSession(sessionID []byte) []byte {
	return fetchItem([]byte(encryptedSessionsBucket), []byte(sessionID))
}

func saveEncryptedMessageKey(sessionID []byte, pubKey []byte, msgNum uint, msgKey []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		keysBucket := tx.Bucket([]byte(sessionKeysBucket))
		pubKeyBucket, err := keysBucket.CreateBucketIfNotExists(pubKey)
		if err != nil {
			return err
		}
		return pubKeyBucket.Put(itob(uint64(msgNum)), msgKey)
	})
}

func fetchEncryptedMessageKey(pubKey []byte, msgNum uint) ([]byte, error) {
	var msgKey []byte
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionKeysBucket))
		pubKeyBucket := bucket.Bucket(pubKey)
		if pubKeyBucket == nil {
			return nil
		}
		msgKey = pubKeyBucket.Get(itob(uint64(msgNum)))
		return nil
	})
	return msgKey, err
}

func countMessageKeys(pubKey []byte) (uint, error) {
	var count uint
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionKeysBucket))
		pubKeyBucket := bucket.Bucket(pubKey)
		if pubKeyBucket == nil {
			return nil
		}
		count = uint(pubKeyBucket.Stats().KeyN)
		return nil
	})
	return count, err
}

func allMessageKeys() (map[[32]byte]map[uint][32]byte, error) {
	var all map[[32]byte]map[uint][32]byte
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sessionKeysBucket))
		return bucket.ForEach(func(k, v []byte) error {
			if v == nil {
				var key [32]byte
				copy(key[:], k)
				all[key] = make(map[uint][32]byte)
				pubKeyBucket := bucket.Bucket(k)
				pubKeyBucket.ForEach(func(mk, mv []byte) error {
					var msgKey [32]byte
					copy(msgKey[:], k)
					all[key][uint(btoi(mk))] = msgKey
					return nil
				})
			}
			return nil
		})
	})
	return all, err
}

func createSessionContext(sessionID []byte, initiated bool, expiry uint64) error {
	contextBytes := itob(expiry)
	initiatedByte := byte(0)
	if initiated {
		initiatedByte = byte(1)
	}
	contextBytes = append(contextBytes, initiatedByte)
	return saveItem([]byte(sessionContextBucket), sessionID, contextBytes)
}

func fetchSessionContext(sessionID []byte) (initiated bool, expiry uint64) {
	sessionContext := fetchItem([]byte(sessionContextBucket), sessionID)
	if sessionContext == nil {
		return false, 0
	}
	expiry = btoi(sessionContext[:8])
	initiated = sessionContext[8] == 1
	return
}

func deleteExpiredSessions() error {
	currentTime := uint64(time.Now().Unix())
	return db.Update(func(tx *bolt.Tx) error {
		sessionCtxBucket := tx.Bucket([]byte(sessionContextBucket))
		sessionInfbucket := tx.Bucket([]byte(sessionUserInfoBucket))
		sessionsBucket := tx.Bucket([]byte(encryptedSessionsBucket))
		return sessionCtxBucket.ForEach(func(key []byte, val []byte) error {
			if key == nil {
				return nil
			}
			expiry := btoi(val[:8])
			if currentTime >= expiry {
				if err := sessionCtxBucket.Delete(key); err != nil {
					return err
				}
				if err := sessionInfbucket.Delete(key); err != nil {
					return err
				}
				if err := sessionsBucket.Delete(key); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func setSessionInfo(sessionID, info []byte) error {
	return saveItem([]byte(sessionUserInfoBucket), []byte(sessionID), []byte(info))
}

func fetchSessionInfo(sessionID []byte) []byte {
	return fetchItem([]byte(sessionUserInfoBucket), sessionID)
}

func saveItem(bucket []byte, key []byte, value []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put(key, value)
	})
}

func fetchItem(bucket []byte, key []byte) []byte {
	var value []byte
	_ = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		value = b.Get(key)
		return nil
	})
	return value
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func btoi(bytes []byte) uint64 {
	return binary.BigEndian.Uint64(bytes)
}
