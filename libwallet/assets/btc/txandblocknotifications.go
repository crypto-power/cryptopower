package btc

import (
	"encoding/json"
	"fmt"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
)

func (asset *BTCAsset) listenForTransactions() {
	go func() {

		n := asset.Internal().BTC.NtfnServer.TransactionNotifications()
		fmt.Println("[][][][] TransactionNotifications BTC", n)
		for {
			select {
			case v := <-n.C:
				if v == nil {
					fmt.Println("[][][][] V is nil", v)
					return
				}
				fmt.Println("[][][][] V", v)
				fmt.Println("[][][][] UNMINED transactionS", v.UnminedTransactions)

				for _, transaction := range v.UnminedTransactions {
					fmt.Println("[][][][] UNMINED transaction", transaction)

					tempTransaction := asset.decodeTransactionWithTxSummary(-1, transaction)

					overwritten, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, tempTransaction)
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
					fmt.Println("[][][][] AttachedBlocks BTC", block)

					blockHeight := block.Height
					fmt.Println("[][][][] MINED transactionS", block.Transactions)

					for _, transaction := range block.Transactions {
						fmt.Println("[][][][] MINED transaction", transaction)

						tempTransaction := asset.decodeTransactionWithTxSummary(blockHeight, transaction)

						_, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, tempTransaction)
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
