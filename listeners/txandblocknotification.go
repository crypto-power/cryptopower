package listeners

import (
	"encoding/json"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
)

// TxAndBlockNotificationListener satisfies libwallet
// TxAndBlockNotificationListener interface contract.
type TxAndBlockNotificationListener struct {
	TxAndBlockNotifChan chan TxNotification

	// Because of the asynchronous use of TxAndBlockNotifChan chan, sometimes
	// TxAndBlockNotifChan could be closed when the send goroutine is still running
	// NotifChanClosed should help to identify when TxAndBlockNotifChan was closed
	// thereby prevent the send via a closed channnel.
	NotifChanClosed chan struct{}
}

func NewTxAndBlockNotificationListener() *TxAndBlockNotificationListener {
	return &TxAndBlockNotificationListener{
		TxAndBlockNotifChan: make(chan TxNotification, 4),
		NotifChanClosed:     make(chan struct{}, 1),
	}
}

func (txAndBlk *TxAndBlockNotificationListener) OnTransaction(transaction string) {
	var tx sharedW.Transaction
	err := json.Unmarshal([]byte(transaction), &tx)
	if err != nil {
		log.Errorf("Error unmarshalling transaction: %v", err)
		return
	}

	update := TxNotification{
		Type:        NewTransaction,
		Transaction: &tx,
	}
	txAndBlk.UpdateNotification(update)
}

func (txAndBlk *TxAndBlockNotificationListener) OnBlockAttached(walletID int, blockHeight int32) {
	txAndBlk.UpdateNotification(TxNotification{
		Type:        BlockAttached,
		WalletID:    walletID,
		BlockHeight: blockHeight,
	})
}

func (txAndBlk *TxAndBlockNotificationListener) OnTransactionConfirmed(walletID int, hash string, blockHeight int32) {
	txAndBlk.UpdateNotification(TxNotification{
		Type:        TxConfirmed,
		WalletID:    walletID,
		BlockHeight: blockHeight,
		Hash:        hash,
	})
}

func (txAndBlk *TxAndBlockNotificationListener) UpdateNotification(signal TxNotification) {
	// Since select randomly chooses which case to execute, If TxAndBlockNotifChan
	// channel is closed further execution is stopped.
	select {
	case <-txAndBlk.NotifChanClosed:
		// txAndBlk.TxAndBlockNotifChan already closed, exit the function now.
		return
	case <-time.After(time.Second * 2):
		// channel not yet closed
	}

	// Second select can proceed to write to the channel if its open.
	select {
	case txAndBlk.TxAndBlockNotifChan <- signal:
	default:
	}
}
