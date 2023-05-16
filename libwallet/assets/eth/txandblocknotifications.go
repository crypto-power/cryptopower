package eth

import (
	"errors"
	"sync/atomic"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

// AddSyncProgressListener registers a sync progress listener to the asset.
func (asset *Asset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if _, ok := asset.syncData.syncProgressListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	return nil
}

// RemoveSyncProgressListener unregisters a sync progress listener from the asset.
func (asset *Asset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	delete(asset.syncData.syncProgressListeners, uniqueIdentifier)
}

func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
	asset.blocksRescanProgressListener = blocksRescanProgressListener
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
func (asset *Asset) publishBlockAttached() {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	blockHeight := int32(asset.backend.SyncProgress().CurrentBlock)
	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnBlockAttached(asset.ID, blockHeight)
	}

	// publish the fetch complete UI update.
	if asset.IsSynced() && atomic.CompareAndSwapUint32(&asset.syncData.syncEnded, stop, start) {
		asset.publishHeadersFetchComplete()
	}
}

func (asset *Asset) publishHeadersFetchComplete() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	asset.handleSyncUIUpdate()

	asset.syncData.synced = true
	asset.syncData.syncing = false
}

func (asset *Asset) handleSyncUIUpdate() {
	for _, listener := range asset.syncData.syncProgressListeners {
		listener.OnSyncCompleted()
	}
}
