package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/breez/breez/data"
	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"go.etcd.io/bbolt"
)

var (
	unconfirmedClaimTransactionKey = []byte("unconfirmed_claim_transaction")
	unspendLockupTransactionKey    = []byte("unspend_lockup_transaction")
)

// SaveReverseSwap saves the reverse swap information
func (db *DB) SaveReverseSwap(rs *data.ReverseSwap) (string, error) {
	p, err := hex.DecodeString(rs.Preimage)
	if err != nil {
		return "", fmt.Errorf("hex.DecodeString(%v): %w", rs.Preimage, err)
	}
	data, err := proto.Marshal(rs)
	if err != nil {
		return "", fmt.Errorf("proto.Marshal(%#v): %w", rs, err)
	}
	h := sha256.Sum256(p)
	return hex.EncodeToString(h[:]), db.Update(func(tx *bbolt.Tx) error {
		rsb := tx.Bucket([]byte(reverseSwapBucket))
		return rsb.Put(h[:], data)
	})
}

// FetchReverseSwap fetches the reverse swap information
func (db *DB) FetchReverseSwap(hash string) (*data.ReverseSwap, error) {
	h, err := hex.DecodeString(hash)
	if err != nil {
		return nil, err
	}
	var b []byte
	err = db.View(func(tx *bbolt.Tx) error {
		rsb := tx.Bucket([]byte(reverseSwapBucket))
		b = rsb.Get(h)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	var rs data.ReverseSwap
	err = proto.Unmarshal(b, &rs)
	if err != nil {
		return nil, fmt.Errorf("proto.Unmarshal(%x): %w", b, err)
	}
	return &rs, nil
}

// SaveUnconfirmedClaimTransaction saves the unconfirmed claim transaction
// set confRequest to nil when the transaction is confirmed
func (db *DB) SaveUnconfirmedClaimTransaction(confRequest *chainrpc.ConfRequest) error {
	if confRequest == nil {
		return db.Update(func(tx *bbolt.Tx) error {
			rsb := tx.Bucket([]byte(reverseSwapBucket))
			return rsb.Delete(unconfirmedClaimTransactionKey)
		})
	}
	data, err := proto.Marshal(confRequest)
	if err != nil {
		return fmt.Errorf("proto.Marshal(%#v): %w", confRequest, err)
	}
	return db.Update(func(tx *bbolt.Tx) error {
		rsb := tx.Bucket([]byte(reverseSwapBucket))
		return rsb.Put(unconfirmedClaimTransactionKey, data)
	})
}

// FetchUnconfirmedClaimTransaction returns the current unconfirmed claim
// transaction, or nil if there is no unconfimed claim transaction.
func (db *DB) FetchUnconfirmedClaimTransaction() (*chainrpc.ConfRequest, error) {
	var b []byte
	err := db.View(func(tx *bbolt.Tx) error {
		rsb := tx.Bucket([]byte(reverseSwapBucket))
		b = rsb.Get(unconfirmedClaimTransactionKey)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	var confRequest chainrpc.ConfRequest
	err = proto.Unmarshal(b, &confRequest)
	if err != nil {
		return nil, fmt.Errorf("proto.Unmarshal(%x): %w", b, err)
	}
	return &confRequest, nil
}

// SaveUnspendLockupInformation saves the unconfirmed claim transaction
// set unspendLockupTransaction to nil when the transaction is confirmed
func (db *DB) SaveUnspendLockupInformation(unspendLockupTransaction *data.UnspendLockupInformation) error {
	if unspendLockupTransaction == nil {
		db.log.Infof("Resetting unconfirmed reverse swap")
		return db.Update(func(tx *bbolt.Tx) error {
			rsb := tx.Bucket([]byte(reverseSwapBucket))
			return rsb.Delete(unspendLockupTransactionKey)
		})
	}
	data, err := proto.Marshal(unspendLockupTransaction)
	if err != nil {
		return fmt.Errorf("proto.Marshal(%#v): %w", unspendLockupTransaction, err)
	}
	return db.Update(func(tx *bbolt.Tx) error {
		rsb := tx.Bucket([]byte(reverseSwapBucket))
		return rsb.Put(unspendLockupTransactionKey, data)
	})
}

// FetchUnspendLockupInformation returns the current unconfirmed claim
// transaction, or nil if there is no unconfimed claim transaction.
func (db *DB) FetchUnspendLockupInformation() (*data.UnspendLockupInformation, error) {
	var b []byte
	err := db.View(func(tx *bbolt.Tx) error {
		rsb := tx.Bucket([]byte(reverseSwapBucket))
		b = rsb.Get(unspendLockupTransactionKey)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	var unspendLockupTransaction data.UnspendLockupInformation
	err = proto.Unmarshal(b, &unspendLockupTransaction)
	if err != nil {
		return nil, fmt.Errorf("proto.Unmarshal(%x): %w", b, err)
	}
	return &unspendLockupTransaction, nil
}
