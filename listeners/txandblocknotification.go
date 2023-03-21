package listeners

import (
	"encoding/json"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
)

// TxAndBlockNotificationListener satisfies libwallet
// TxAndBlockNotificationListener interface contract.
type TxAndBlockNotificationListener struct {
	txAndBlockNotifChan chan TxNotification

	// Because of the asynchronous use of txAndBlockNotifChan chan, sometimes
	// txAndBlockNotifChan could be closed when the send goroutine is still running
	// notifChanClosed should help to identify when txAndBlockNotifChan was closed
	// thereby preventing the send via a closed channnel.
	notifChanClosed chan struct{}
	// Waitgroup keeps track of all the write operations
	wg sync.WaitGroup
	// Mutex prevent calling Wait() and add() methods simultaneuosly that could
	// result into a race condition.
	wgMu sync.Mutex
}

func NewTxAndBlockNotificationListener() *TxAndBlockNotificationListener {
	return &TxAndBlockNotificationListener{
		txAndBlockNotifChan: make(chan TxNotification, 4),
		notifChanClosed:     make(chan struct{}, 1),
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

// TxAndBlockNotifChan returns a read-only channel.
func (txAndBlk *TxAndBlockNotificationListener) TxAndBlockNotifChan() <-chan TxNotification {
	return txAndBlk.txAndBlockNotifChan
}

func (txAndBlk *TxAndBlockNotificationListener) UpdateNotification(signal TxNotification) {
	txAndBlk.wgMu.Lock()
	txAndBlk.wg.Add(1)
	txAndBlk.wgMu.Unlock()

	defer txAndBlk.wg.Done()

	// Since select randomly chooses which case to execute, If TxAndBlockNotifChan
	// channel is closed further execution is stopped.
	select {
	case <-txAndBlk.notifChanClosed:
		// txAndBlk.txAndBlockNotifChan already closed, exit the function now.
		return
	default:
		// default is choosen after the pseudo random case selected from all available
		// cases fails to proceed with execution.
		// channel not yet closed
	}

	// Second select can proceed to write to the channel if its open.
	select {
	case txAndBlk.txAndBlockNotifChan <- signal:
	default:
	}
}

func (txAndBlk *TxAndBlockNotificationListener) CloseTxAndBlockChan() {
	close(txAndBlk.notifChanClosed)

	// Drain all unread channel's contents.
	go func() {
		for range txAndBlk.txAndBlockNotifChan {
		}
	}()

	// Wait until all pending writes succeed.
	txAndBlk.wgMu.Lock()
	txAndBlk.wg.Wait()
	txAndBlk.wgMu.Unlock()

	// now close the main notifications channel.
	close(txAndBlk.txAndBlockNotifChan)
}
