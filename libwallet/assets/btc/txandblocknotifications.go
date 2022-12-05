package btc

import (
	"sync/atomic"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
)

func (asset *BTCAsset) listenForTransactions() {
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

			// handle txs hitting the mempool.
			for _, txhash := range n.UnminedTransactionHashes {
				log.Debugf("(%v) Incoming unmined tx with hash (%v)", txhash, asset.GetWalletName())

				// publish mempool tx.
				asset.mempoolTxNotification(txhash.String())
			}

			// Handle Historical, Connected blocks and newly mined Txs.
			for _, b := range n.AttachedBlocks {
				// When syncing historical data no tx are available.
				// Txs are reported only when chain is synced and newly mined tx
				// we discovered in the latest block.
				for _, tx := range b.Transactions {
					log.Debugf("(%v) Incoming mined tx with hash=%v block=%v",
						asset.GetWalletName(), tx.Hash, b.Height)

					// Publish the confirmed tx notification.
					asset.publishRelevantTx(tx.Hash.String(), b.Height)
				}
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
func (asset *BTCAsset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener,
	async bool, uniqueIdentifier string) error {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	if _, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	if async {
		asset.txAndBlockNotificationListeners[uniqueIdentifier] = &sharedW.AsyncTxAndBlockNotificationListener{
			TxAndBlockNotificationListener: txAndBlockNotificationListener,
		}
		return nil
	}

	asset.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	return nil
}

func (asset *BTCAsset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	delete(asset.txAndBlockNotificationListeners, uniqueIdentifier)
}

// mempoolTxNotification publishes the txs that hit the mempool for the first time.
func (asset *BTCAsset) mempoolTxNotification(transaction string) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransaction(transaction)
	}
}

// publishRelevantTx publishes all the relevant tx identified in a filtered
// block. A valid list of addresses associated with the current block need to
// be provided.
func (asset *BTCAsset) publishRelevantTx(txHash string, blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransactionConfirmed(asset.ID, txHash, blockHeight)
	}
}

// publishNewBlock once the initial sync is complete all the new blocks recieved
// are published through this method.
func (asset *BTCAsset) publishNewBlock(blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnBlockAttached(asset.ID, blockHeight)
	}
}
