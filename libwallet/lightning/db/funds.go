package db

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

//SwapAddressInfo contains all the infromation regarding
//a submarine swap address.
type SwapAddressInfo struct {
	LspID            string
	PaymentRequest   string
	Address          string
	CreatedTimestamp int64

	//client side data
	PaymentHash []byte
	Preimage    []byte
	PrivateKey  []byte
	PublicKey   []byte

	//tracked data
	ConfirmedTransactionIds []string
	ConfirmedAmount         int64
	InvoicedAmount          int64
	PaidAmount              int64
	LockHeight              uint32
	FundingTxID             string

	//address script
	Script []byte

	ErrorMessage    string
	SwapErrorReason int32
	EnteredMempool  bool

	//refund
	LastRefundTxID string
	NonBlocking    bool
}

// Confirmed returns true if the transaction has confirmed in the past.
func (s *SwapAddressInfo) Confirmed() bool {
	return s.LockHeight > 0
}

/**
Swap addresses
**/
func (db *DB) FetchAllSwapAddresses() ([]*SwapAddressInfo, error) {
	return db.FetchSwapAddresses(func(addr *SwapAddressInfo) bool {
		return true
	})
}

func (db *DB) FetchSwapAddresses(filterFunc func(addr *SwapAddressInfo) bool) ([]*SwapAddressInfo, error) {
	var addresses []*SwapAddressInfo
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(addressesBucket))
		b.ForEach(func(k, v []byte) error {
			var address SwapAddressInfo
			err := json.Unmarshal(v, &address)
			if err != nil {
				return err
			}
			if filterFunc(&address) {
				addresses = append(addresses, &address)
			}
			return nil
		})
		return nil
	})
	return addresses, err
}

func (db *DB) SaveSwapAddressInfo(address *SwapAddressInfo) error {
	return db.Update(func(tx *bolt.Tx) error {
		byHashBucket := tx.Bucket([]byte(swapAddressesByHashBucket))
		addressesBucket := tx.Bucket([]byte(addressesBucket))
		bytes, err := serializeSwapAddressInfo(address)
		if err != nil {
			return err
		}
		if err = byHashBucket.Put(address.PaymentHash, []byte(address.Address)); err != nil {
			return err
		}
		if err = addressesBucket.Put([]byte(address.Address), bytes); err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) UpdateSwapAddressByPaymentHash(pHash []byte, updateFunc func(*SwapAddressInfo) error) (bool, error) {

	address, err := db.fetchItem([]byte(swapAddressesByHashBucket), pHash)
	if err != nil {
		return false, err
	}
	if address == nil {
		return false, nil
	}
	return db.UpdateSwapAddress(string(address), updateFunc)
}

func (db *DB) UpdateSwapAddress(address string, updateFunc func(*SwapAddressInfo) error) (bool, error) {
	var found bool
	err := db.Update(func(tx *bolt.Tx) error {
		addressesBucket := tx.Bucket([]byte(addressesBucket))
		addressInfoBytes := addressesBucket.Get([]byte(address))
		if addressInfoBytes == nil {
			return nil
		}
		found = true
		swapAddress, err := deserializeSwapAddressInfo(addressInfoBytes)
		if err != nil {
			return err
		}
		if err = updateFunc(swapAddress); err != nil {
			return err
		}

		if err != nil {
			return err
		}

		swapAddressData, err := serializeSwapAddressInfo(swapAddress)
		if err != nil {
			return err
		}

		if err = addressesBucket.Put([]byte(swapAddress.Address), swapAddressData); err != nil {
			return err
		}
		return nil
	})
	return found, err
}

func (db *DB) AddRedeemablePaymentHash(hash string) error {
	return db.Update(func(tx *bolt.Tx) error {
		redeemableHashesB := tx.Bucket([]byte(redeemableHashesBucket))
		return redeemableHashesB.Put([]byte(hash), []byte{})
	})
}

func (db *DB) FetchRedeemablePaymentHashes() ([]string, error) {
	var hashes []string
	err := db.View(func(tx *bolt.Tx) error {
		redeemableHashesB := tx.Bucket([]byte(redeemableHashesBucket))
		return redeemableHashesB.ForEach(func(k, v []byte) error {
			hashes = append(hashes, string(k))
			return nil
		})
	})
	return hashes, err
}

func (db *DB) UpdateRedeemTxForPayment(hash string, txID string) error {
	db.log.Infof("updateRedeemTxForPayment hash = %v, txid=%v", hash, txID)
	return db.Update(func(tx *bolt.Tx) error {
		paymentB := tx.Bucket([]byte(paymentsBucket))
		hashB := tx.Bucket([]byte(paymentsHashBucket))
		paymentIndex := hashB.Get([]byte(hash))
		if paymentIndex == nil {
			return fmt.Errorf("payment doesn't exist for hash %v", hash)
		}
		paymentBuf := paymentB.Get(paymentIndex)
		payment, err := deserializePaymentInfo(paymentBuf)
		if err != nil {
			return err
		}
		payment.RedeemTxID = txID
		paymentBuf, err = serializePaymentInfo(payment)
		if err != nil {
			return err
		}
		err = paymentB.Put(paymentIndex, paymentBuf)
		if err != nil {
			return err
		}

		redeemableHashesB := tx.Bucket([]byte(redeemableHashesBucket))
		return redeemableHashesB.Delete([]byte(hash))
	})
}

// IsInvoiceHashPaid returns true if the payReqHash was paid by this client.
func (db *DB) IsInvoiceHashPaid(payReqHash string) (bool, error) {
	var paid bool
	err := db.View(func(tx *bolt.Tx) error {
		hashB := tx.Bucket([]byte(paymentsHashBucket))
		paymentIndex := hashB.Get([]byte(payReqHash))
		paid = paymentIndex != nil
		return nil
	})
	return paid, err
}

func serializeSwapAddressInfo(s *SwapAddressInfo) ([]byte, error) {
	return json.Marshal(s)
}

func deserializeSwapAddressInfo(addressBytes []byte) (*SwapAddressInfo, error) {
	var addressInfo SwapAddressInfo
	err := json.Unmarshal(addressBytes, &addressInfo)
	return &addressInfo, err
}
