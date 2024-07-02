package btc

import (
	"decred.org/dcrwallet/v4/errors"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

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
func (asset *Asset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener *sharedW.TxAndBlockNotificationListener,
	uniqueIdentifier string,
) error {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	if _, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.txAndBlockNotificationListeners[uniqueIdentifier] = &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(walletID int, transaction *sharedW.Transaction) {
			if txAndBlockNotificationListener.OnTransaction != nil {
				go txAndBlockNotificationListener.OnTransaction(walletID, transaction)
			}
		},
		OnBlockAttached: func(walletID int, blockHeight int32) {
			if txAndBlockNotificationListener.OnBlockAttached != nil {
				go txAndBlockNotificationListener.OnBlockAttached(walletID, blockHeight)
			}
		},
		OnTransactionConfirmed: func(walletID int, hash string, blockHeight int32) {
			if txAndBlockNotificationListener.OnTransactionConfirmed != nil {
				txAndBlockNotificationListener.OnTransactionConfirmed(walletID, hash, blockHeight)
			}
		},
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
func (asset *Asset) mempoolTransactionNotification(transaction *sharedW.Transaction) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotificationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotificationListener.OnTransaction(asset.ID, transaction)
	}
}

// publishTransactionConfirmed publishes all the relevant tx identified in a filtered
// block. A valid list of addresses associated with the current block need to
// be provided.
func (asset *Asset) publishTransactionConfirmed(txHash string, blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotificationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotificationListener.OnTransactionConfirmed(asset.ID, txHash, blockHeight)
	}
}

// publishBlockAttached once the initial sync is complete all the new blocks received
// are published through this method.
func (asset *Asset) publishBlockAttached(blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotificationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotificationListener.OnBlockAttached(asset.ID, blockHeight)
	}
}
