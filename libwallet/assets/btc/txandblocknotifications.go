package btc

import (
	"encoding/json"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
)

func (asset *BTCAsset) listenForTransactions() {
	go func() {
		log.Infof("Subscribing wallet (%s) for transaction notifications", asset.GetWalletName())
		n := asset.Internal().BTC.NtfnServer.TransactionNotifications()

		for {
			select {
			case v := <-n.C:
				if v == nil {
					return
				}

				for _, transaction := range v.UnminedTransactions {
					log.Debugf("Incoming unmined transaction with hash (%v)", transaction.Hash)

					tempTransaction := asset.decodeTransactionWithTxSummary(-1, transaction)

					overwritten, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, &tempTransaction)
					if err != nil {
						log.Errorf("[%d] New Tx save err: %v", asset.ID, err)
						return
					}

					if !overwritten {
						log.Infof("[%d] New Transaction %s", asset.ID, tempTransaction.Hash)

						result, err := json.Marshal(tempTransaction)
						if err != nil {
							log.Error(err)
						} else {
							asset.mempoolTransactionNotification(string(result))
						}
					}
				}

				for _, block := range v.AttachedBlocks {
					blockHeight := block.Height
					log.Infof("Incoming block with height (%d), and hash (%v)", blockHeight, block.Hash)

					for _, transaction := range block.Transactions {
						log.Debugf("Incoming mined transaction with hash (%v)", transaction.Hash)

						tempTransaction := asset.decodeTransactionWithTxSummary(blockHeight, transaction)

						_, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, &tempTransaction)
						if err != nil {
							log.Errorf("[%d] Incoming block replace tx error :%v", asset.ID, err)
							return
						}
						asset.publishTransactionConfirmed(transaction.Hash.String(), int32(block.Height))
					}

					asset.publishBlockAttached(int32(block.Height))
				}

			case <-asset.syncData.syncCanceled:
				n.Done()
			}
		}
	}()
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
