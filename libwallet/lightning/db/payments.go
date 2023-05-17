package db

import (
	"encoding/json"
	"errors"

	bolt "go.etcd.io/bbolt"
)

// PaymentType is the type of payment
type PaymentType byte
type ChannelCloseStatus byte

const (
	//SentPayment is type for sent payments
	SentPayment = PaymentType(0)

	//ReceivedPayment is type for received payments
	ReceivedPayment = PaymentType(1)

	//DepositPayment is type for payment got from add funds
	DepositPayment = PaymentType(2)

	//WithdrawalPayment is type for payment got from remove funds
	WithdrawalPayment = PaymentType(3)

	//ClosedChannelPayment is type for payment got from a closed channel
	ClosedChannelPayment = PaymentType(4)

	WaitingClose   = ChannelCloseStatus(0)
	PendingClose   = ChannelCloseStatus(1)
	ConfirmedClose = ChannelCloseStatus(2)
)

/*
PaymentInfo is the structure that holds the data for a payment in breez
*/
type PaymentInfo struct {
	Type                       PaymentType
	Amount                     int64
	Fee                        int64
	CreationTimestamp          int64
	Description                string
	PayeeName                  string
	PayeeImageURL              string
	PayerName                  string
	PayerImageURL              string
	TransferRequest            bool
	PaymentHash                string
	RedeemTxID                 string
	Destination                string
	PendingExpirationHeight    uint32
	PendingExpirationTimestamp int64
	PendingFull                bool
	Preimage                   string
	IsKeySend                  bool
	GroupKey                   string
	GroupName                  string

	//For closed channels
	ClosedChannelPoint      string
	ClosedChannelStatus     ChannelCloseStatus
	ClosedChannelTxID       string
	ClosedChannelRemoteTxID string
	ClosedChannelSweepTxID  string
}

/*
AddAccountPayment adds a payment to the database
*/
func (db *DB) AddAccountPayment(accPayment *PaymentInfo, receivedIndex uint64, sentTime uint64) (bool, error) {
	db.log.Infof("addAccountPayment hash = %v", accPayment.PaymentHash)
	var paymentExists bool
	err := db.Update(func(tx *bolt.Tx) error {
		var err error
		paymentExists, err = db.hasAccountPayment(accPayment, tx)
		if err != nil {
			return err
		}
		if paymentExists {
			return nil
		}
		_, err = db.addPayment(accPayment, tx, 0)
		if err != nil {
			return err
		}
		b := tx.Bucket([]byte(paymentsBucket))

		syncInfoBucket := b.Bucket([]byte(paymentsSyncInfoBucket))

		//if we have a newer item, update the last payment timestamp
		lastPaymentTime := uint64(0)
		if lastPaymentTimeBuf := syncInfoBucket.Get([]byte("lastSentPaymentTime")); lastPaymentTimeBuf != nil {
			lastPaymentTime = btoi(lastPaymentTimeBuf)
		}
		if lastPaymentTime < sentTime {
			if err := syncInfoBucket.Put([]byte("lastSentPaymentTime"), itob(sentTime)); err != nil {
				return err
			}
		}

		lastInvoiceSettledIndex := uint64(0)
		if lastInvoiceSettledIndexBuf := syncInfoBucket.Get([]byte("lastSettledIndex")); lastInvoiceSettledIndexBuf != nil {
			lastInvoiceSettledIndex = btoi(lastInvoiceSettledIndexBuf)
		}
		if lastInvoiceSettledIndex < receivedIndex {
			if err := syncInfoBucket.Put([]byte("lastSettledIndex"), itob(receivedIndex)); err != nil {
				return err
			}
		}
		return nil
	})
	return paymentExists, err
}

// AddChannelClosedPayment adds a payment that reflects channel close to the db.
func (db *DB) AddChannelClosedPayment(accPayment *PaymentInfo) error {
	db.log.Infof("AddChannelClosedPayment chanel point = %v", accPayment.ClosedChannelPoint)
	return db.Update(func(tx *bolt.Tx) error {
		chanIDKey := []byte(accPayment.ClosedChannelPoint)
		existingPayment, paymentID, err := db.fetchClosedChannelPayment(tx, chanIDKey)
		if err != nil {
			return err
		}

		// the payment reflects this channel is already in db.
		if existingPayment != nil {
			if existingPayment.ClosedChannelStatus == ConfirmedClose ||
				existingPayment.ClosedChannelStatus > accPayment.ClosedChannelStatus {
				db.log.Infof("skipping closed channel payment %v", accPayment.ClosedChannelPoint)
				return nil
			}
			if accPayment.ClosedChannelStatus != ConfirmedClose {
				accPayment.CreationTimestamp = existingPayment.CreationTimestamp
			}
		}

		db.log.Infof("adding not existing closed channel payment %v, sweep: ",
			accPayment.ClosedChannelPoint, accPayment.ClosedChannelSweepTxID)

		id, err := db.addPayment(accPayment, tx, paymentID)
		if err != nil {
			db.log.Infof("failed to add closed channel payment %v", err)
			return err
		}
		db.log.Info("Closed channel payment added successfully")
		pb := tx.Bucket([]byte(paymentsBucket))
		pb.Delete(chanIDKey[:])
		b := tx.Bucket([]byte(closedChannelsBucket))
		return b.Put(chanIDKey[:], itob(id))
	})
}

func (db *DB) fetchClosedChannelPayment(tx *bolt.Tx, channelPoint []byte) (payment *PaymentInfo, index uint64, err error) {
	b := tx.Bucket([]byte(closedChannelsBucket))

	// the payment reflects this channel is already in db.
	closeChanRecID := b.Get(channelPoint[:])
	if closeChanRecID != nil {
		paymentsBucket := tx.Bucket([]byte(paymentsBucket))
		closeChanRec := paymentsBucket.Get(closeChanRecID)
		if closeChanRec != nil {
			payment, err = deserializePaymentInfo(closeChanRec)
			index = btoi(closeChanRecID)
		}
	}
	return
}

func (db *DB) hasAccountPayment(accPayment *PaymentInfo, tx *bolt.Tx) (bool, error) {
	if accPayment.PaymentHash == "" {
		return false, errors.New("account payment must have payment hash")
	}
	hashB := tx.Bucket([]byte(paymentsHashBucket))
	if oldID := hashB.Get([]byte(accPayment.PaymentHash)); oldID != nil {
		return true, nil
	}
	return false, nil
}

func (db *DB) addPayment(accPayment *PaymentInfo, tx *bolt.Tx, existingID uint64) (uint64, error) {
	paymentBuf, err := serializePaymentInfo(accPayment)
	if err != nil {
		return 0, err
	}

	b := tx.Bucket([]byte(paymentsBucket))

	id := existingID
	if id == 0 {
		hashB := tx.Bucket([]byte(paymentsHashBucket))
		if oldID := hashB.Get([]byte(accPayment.PaymentHash)); oldID != nil {
			id = btoi(oldID)
		}
		if id == 0 {
			nextSeq, err := b.NextSequence()
			if err != nil {
				return 0, err
			}
			id = nextSeq
		}
	}

	//write the payment value with the next sequence as key
	if err := b.Put(itob(id), paymentBuf); err != nil {
		return 0, err
	}

	if accPayment.PaymentHash != "" {
		hashB := tx.Bucket([]byte(paymentsHashBucket))
		if err := hashB.Put([]byte(accPayment.PaymentHash), itob(id)); err != nil {
			return 0, err
		}
	}

	return id, nil
}

/*
FetchAllAccountPayments fetches all payments in the database sorted by date
*/
func (db *DB) FetchAllAccountPayments() ([]*PaymentInfo, error) {
	var payments []*PaymentInfo
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(paymentsBucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				//nested bucket
				continue
			}
			payment, err := deserializePaymentInfo(v)
			if err != nil {
				return err
			}
			payments = append(payments, payment)
		}

		return nil
	})
	return payments, err
}

/*
FetchPaymentsSyncInfo returns the last payment time and last invoice settled index.
This is used for callers to understand when needs to be synchronized into the db
*/
func (db *DB) FetchPaymentsSyncInfo() (lastTime int64, lastSetteledIndex uint64) {
	lastPaymentTime := int64(0)
	lastInvoiceSettledIndex := uint64(0)
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(paymentsBucket))
		syncInfoBucket := b.Bucket([]byte(paymentsSyncInfoBucket))
		lastPaymentTimeBuf := syncInfoBucket.Get([]byte("lastSentPaymentTime"))
		if lastPaymentTimeBuf != nil {
			lastPaymentTime = int64(btoi(lastPaymentTimeBuf))
		}
		lastInvoiceSettledIndexBuf := syncInfoBucket.Get([]byte("lastSettledIndex"))
		if lastInvoiceSettledIndexBuf != nil {
			lastInvoiceSettledIndex = btoi(lastInvoiceSettledIndexBuf)
		}
		return nil
	})
	return lastPaymentTime, lastInvoiceSettledIndex
}

// SavePaymentRequest saves a payment request into the database
func (db *DB) SavePaymentRequest(payReqHash string, payReq []byte) error {
	return db.saveItem([]byte(incomingPayReqBucket), []byte(payReqHash), payReq)
}

// FetchPaymentRequest fetches a payment request by a payment hash
func (db *DB) FetchPaymentRequest(payReqHash string) ([]byte, error) {
	return db.fetchItem([]byte(incomingPayReqBucket), []byte(payReqHash))
}

// SaveTipMessage saves a tip message related to payment hash into the database
func (db *DB) SaveTipMessage(payReqHash string, message []byte) error {
	return db.saveItem([]byte(keysendTipMessagBucket), []byte(payReqHash), message)
}

// FetchTipMessage fetches a a tip message related to payment hash
func (db *DB) FetchTipMessage(payReqHash string) ([]byte, error) {
	return db.fetchItem([]byte(keysendTipMessagBucket), []byte(payReqHash))
}

// SavePaymentGroup saves a tip message related to payment hash into the database
func (db *DB) SavePaymentGroup(payReqHash string, groupKey, groupName []byte) error {
	if err := db.saveItem([]byte(paymentGroupBucket), []byte(payReqHash+"-key"), groupKey); err != nil {
		return err
	}
	if err := db.saveItem([]byte(paymentGroupBucket), []byte(payReqHash+"-name"), groupName); err != nil {
		return err
	}
	return nil
}

// FetchPaymentGroup fetches a a tip message related to payment hash
func (db *DB) FetchPaymentGroup(payReqHash string) ([]byte, []byte, error) {
	groupKey, err := db.fetchItem([]byte(paymentGroupBucket), []byte(payReqHash+"-key"))
	if err != nil {
		return nil, nil, err
	}
	groupName, err := db.fetchItem([]byte(paymentGroupBucket), []byte(payReqHash+"-name"))
	if err != nil {
		return nil, nil, err
	}
	return groupKey, groupName, nil
}

func serializePaymentInfo(s *PaymentInfo) ([]byte, error) {
	return json.Marshal(s)
}

func deserializePaymentInfo(paymentBytes []byte) (*PaymentInfo, error) {
	var payment PaymentInfo
	err := json.Unmarshal(paymentBytes, &payment)
	return &payment, err
}
