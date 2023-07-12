package btc

import (
	"encoding/json"
	"sync/atomic"

	"decred.org/dcrwallet/v3/errors"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

func (asset *Asset) listenForTransactions() {
	if !atomic.CompareAndSwapUint32(&asset.syncData.txlistening, stop, start) {
		// sync listening in progress already.
		return
	}

	log.Infof("Subscribing wallet (%s) for transaction notifications", asset.GetWalletName())
	notify := asset.Internal().BTC.NtfnServer.TransactionNotifications()

notificationsLoop:
	for {
		select {
		case n, ok := <-notify.C:
			if !ok {
				break notificationsLoop
			}

			txToCache := make([]sharedW.Transaction, len(n.UnminedTransactions))

			// handle txs hitting the mempool.
			for i, tx := range n.UnminedTransactions {
				log.Debugf("(%v) Incoming unmined tx with hash (%v)",
					asset.GetWalletName(), tx.Hash.String())

				// decodeTxs
				txToCache[i] = asset.decodeTransactionWithTxSummary(sharedW.UnminedTxHeight, tx)

				result, err := json.Marshal(txToCache[i])
				if err != nil {
					log.Error(err)
				} else {
					// publish mempool tx.
					asset.mempoolTransactionNotification(string(result))
				}
			}

			if len(n.UnminedTransactions) > 0 {
				// Since the tx cache receives a fresh update only when a new
				// block is detected, update cache with the newly received mempool tx(s).
				asset.txs.mu.Lock()
				asset.txs.unminedTxs = append(txToCache, asset.txs.unminedTxs...)
				asset.txs.mu.Unlock()
			}

			// Handle Historical, Connected blocks and newly mined Txs.
			for _, block := range n.AttachedBlocks {
				// When syncing historical data no tx are available.
				// Txs are reported only when chain is synced and newly mined tx
				// we discovered in the latest block.
				for _, tx := range block.Transactions {
					log.Debugf("(%v) Incoming mined tx with hash=%v block=%v",
						asset.GetWalletName(), tx.Hash, block.Height)

					// Publish the confirmed tx notification.
					asset.publishTransactionConfirmed(tx.Hash.String(), block.Height)
				}

				asset.publishBlockAttached(block.Height)
			}

		case <-asset.syncCtx.Done():
			notify.Done()
			break notificationsLoop
		}
	}

	// Signal that handleNotifications can be safely started next time its needed.
	atomic.StoreUint32(&asset.syncData.syncstarted, stop)
	// when done allow timer reset.
	atomic.SwapUint32(&asset.syncData.txlistening, stop)
}

// AddTxAndBlockNotificationListener registers a set of functions to be invoked
// when a transaction or block update is processed by the asset. If async is
// true, the provided callback methods will be called from separate goroutines,
// allowing notification senders to continue their operation without waiting
// for the listener to complete processing the notification. This asyncrhonous
// handling is especially important for cases where the wallet process that
// sends the notification temporarily prevents access to other wallet features
// until all notification handlers finish processing the notification. If a
// notification handler were to try to access such features, it would result
// in a deadlock.
func (asset *Asset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener,
	async bool, uniqueIdentifier string,
) error {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	if _, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	if async {
		asset.txAndBlockNotificationListeners[uniqueIdentifier] = &sharedW.AsyncTxAndBlockNotificationListener{
			TxAndBlockNotificationListener: txAndBlockNotificationListener,
		}
	} else {
		asset.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	}
	return nil
}

// RemoveTxAndBlockNotificationListener removes a previously registered
// transaction and block notification listener.
func (asset *Asset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	delete(asset.txAndBlockNotificationListeners, uniqueIdentifier)
}

// mempoolTransactionNotification publishes the txs that hit the mempool for the first time.
func (asset *Asset) mempoolTransactionNotification(transaction string) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransaction(transaction)
	}
}

// publishTransactionConfirmed publishes all the relevant tx identified in a filtered
// block. A valid list of addresses associated with the current block need to
// be provided.
func (asset *Asset) publishTransactionConfirmed(txHash string, blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransactionConfirmed(asset.ID, txHash, blockHeight)
	}
}

// publishBlockAttached once the initial sync is complete all the new blocks recieved
// are published through this method.
func (asset *Asset) publishBlockAttached(blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnBlockAttached(asset.ID, blockHeight)
	}
}
