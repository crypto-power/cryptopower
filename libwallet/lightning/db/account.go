package db

import "go.etcd.io/bbolt"

// SaveAccount saves an account information to the database
func (db *DB) SaveAccount(account []byte) error {
	return db.saveItem([]byte(accountBucket), []byte("account"), account)
}

// FetchAccount fetches the cached account info from the database
func (db *DB) FetchAccount() ([]byte, error) {
	return db.fetchItem([]byte(accountBucket), []byte("account"))
}

// EnableAccount sets the account state to either enabled or disabled
func (db *DB) EnableAccount(enabled bool) error {
	var enabledByte byte
	if enabled {
		enabledByte = 1
	}
	return db.saveItem([]byte(accountBucket), []byte("enabled"), []byte{enabledByte})
}

// AccountEnabled returns the account state
func (db *DB) AccountEnabled() (bool, error) {
	bytes, err := db.fetchItem([]byte(accountBucket), []byte("enabled"))
	if err != nil {
		return false, err
	}
	return bytes == nil || bytes[0] == 1, nil
}

// AddZeroConfHash saves a zero conf hash to track.
func (db *DB) AddZeroConfHash(hash []byte, payreq []byte) error {
	return db.saveItem([]byte(zeroConfInvoicesBucket), hash, payreq)
}

// FetchZeroConfInvoice fetches a zero conf invoice.
func (db *DB) FetchZeroConfInvoice(hash []byte) ([]byte, error) {
	return db.fetchItem([]byte(zeroConfInvoicesBucket), hash)
}

// RemoveZeroConfHash removes a zero conf hash from tracking.
func (db *DB) RemoveZeroConfHash(hash []byte) error {
	return db.deleteItem([]byte(zeroConfInvoicesBucket), hash)
}

// FetchZeroConfHashes fetches all zero conf hashes to track
func (db *DB) FetchZeroConfHashes() ([][]byte, error) {
	var hashes [][]byte
	err := db.View((func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(zeroConfInvoicesBucket))
		b.ForEach(func(k, v []byte) error {
			hashes = append(hashes, k)
			return nil
		})
		return nil
	}))
	return hashes, err
}
