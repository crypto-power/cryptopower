package db

import bolt "go.etcd.io/bbolt"

// FetchLNURLAuthKey fetches the bip32 master key for lnurl auth.
func (db *DB) FetchLNURLAuthKey(createNew func() ([]byte, error)) ([]byte, error) {
	var key []byte
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(lnurlAuthBucket))
		key = b.Get([]byte("key"))
		if key == nil {
			newKey, err := createNew()
			if err != nil {
				return err
			}
			if err = b.Put([]byte("key"), newKey); err != nil {
				return err
			}
			key = newKey
		}
		return nil
	})
	return key, err
}
