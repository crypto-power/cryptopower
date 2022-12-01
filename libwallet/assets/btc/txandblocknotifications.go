package btc

import (
	"encoding/json"
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

	// when done allow listening restart.
	defer atomic.SwapUint32(&asset.syncData.txlistening, stop)

	log.Infof("Subscribing wallet (%s) for transaction notifications", asset.GetWalletName())
	n := asset.Internal().BTC.NtfnServer.TransactionNotifications()

	for {
		select {
		case v := <-n.C:
			if v == nil {
				return
			}

			// New transactions are only detected if they are mined while the chain
			// is running. When syncing historical data the attached blocks do not
			// contain txs. This means thus that until the chain is synced,
			// tx can't be handled here.
			if !asset.IsSynced() {
				continue
			}

			for _, transaction := range v.UnminedTransactions {
				log.Infof("Incoming unmined transaction with hash (%v)", transaction.Hash)

				tempTransaction := asset.decodeTransactionWithTxSummary(sharedW.UnminedTxHeight, transaction)
				overwritten, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, &tempTransaction)
				if err != nil {
					log.Errorf("[%s] New Tx save err: %v", asset.GetWalletName(), err)
					return
				}

				if !overwritten {
					log.Infof("[%s] New Transaction %s", asset.GetWalletName(), tempTransaction.Hash)
					result, err := json.Marshal(tempTransaction)
					if err != nil {
						log.Error(err)
					} else {
						asset.mempoolTransactionNotification(string(result))
					}
				}
			}

			for _, block := range v.AttachedBlocks {
				log.Infof("Incoming block with height (%d) and hash (%v)", block.Height, block.Hash)

				for _, transaction := range block.Transactions {
					log.Infof("Incoming mined transaction with hash (%v)", transaction.Hash)

					tempTransaction := asset.decodeTransactionWithTxSummary(block.Height, transaction)
					_, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, &tempTransaction)
					if err != nil {
						log.Errorf("[%s] Incoming block replace tx error :%v", asset.GetWalletName(), err)
						return
					}
					asset.publishTransactionConfirmed(transaction.Hash.String(), int32(block.Height))
				}
				asset.publishBlockAttached(int32(block.Height))
			}

		case <-asset.syncCtx.Done():
			n.Done()
			return
		}
	}
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
func (asset *BTCAsset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	_, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]
	if ok {
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

func (asset *BTCAsset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	delete(asset.txAndBlockNotificationListeners, uniqueIdentifier)
}

func (asset *BTCAsset) mempoolTransactionNotification(transaction string) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransaction(transaction)
	}
}

func (asset *BTCAsset) publishTransactionConfirmed(transactionHash string, blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransactionConfirmed(asset.ID, transactionHash, blockHeight)
	}
}

func (asset *BTCAsset) publishBlockAttached(blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range asset.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnBlockAttached(asset.ID, blockHeight)
	}
}
