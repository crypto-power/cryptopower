package dcr

import (
	"decred.org/dcrwallet/v4/errors"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

func (asset *Asset) listenForTransactions() {
	go func() {
		n := asset.Internal().DCR.NtfnServer.TransactionNotifications()

		for {
			select {
			case v := <-n.C:
				if v == nil {
					return
				}
				for _, transaction := range v.UnminedTransactions {
					tempTransaction, err := asset.decodeTransactionWithTxSummary(&transaction, nil)
					if err != nil {
						log.Errorf("[%d] Error ntfn parse tx: %v", asset.ID, err)
						return
					}

					overwritten, err := asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, tempTransaction)
					if err != nil {
						log.Errorf("[%d] New Tx save err: %v", asset.ID, err)
						return
					}

					if !overwritten {
						log.Infof("[%d] New Transaction %s", asset.ID, tempTransaction.Hash)
						asset.mempoolTransactionNotification(tempTransaction)
					}
				}

				for _, block := range v.AttachedBlocks {
					blockHash := block.Header.BlockHash()
					for _, transaction := range block.Transactions {
						tempTransaction, err := asset.decodeTransactionWithTxSummary(&transaction, &blockHash)
						if err != nil {
							log.Errorf("[%d] Error ntfn parse tx: %v", asset.ID, err)
							return
						}

						_, err = asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, tempTransaction)
						if err != nil {
							log.Errorf("[%d] Incoming block replace tx error :%v", asset.ID, err)
							return
						}
						asset.publishTransactionConfirmed(transaction.Hash.String(), int32(block.Header.Height))
					}

					asset.publishBlockAttached(int32(block.Header.Height))
				}

				if len(v.AttachedBlocks) > 0 {
					asset.checkWalletMixers()
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
func (asset *Asset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener *sharedW.TxAndBlockNotificationListener, uniqueIdentifier string) error {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	_, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]
	if ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	return nil
}

func (asset *Asset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	delete(asset.txAndBlockNotificationListeners, uniqueIdentifier)
}

func (asset *Asset) checkWalletMixers() {
	if asset.IsAccountMixerActive() {
		unmixedAccount := asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)
		hasMixableOutput := asset.accountHasMixableOutput(unmixedAccount)
		if !hasMixableOutput {
			log.Infof("[%d] unmixed account does not have a mixable output, stopping account mixer", asset.ID)
			err := asset.StopAccountMixer()
			if err != nil {
				log.Errorf("Error stopping account mixer: %v", err)
			}
		}
	}
}

func (asset *Asset) mempoolTransactionNotification(transaction *sharedW.Transaction) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotificationListener := range asset.txAndBlockNotificationListeners {
		if txAndBlockNotificationListener.OnTransaction != nil {
			go txAndBlockNotificationListener.OnTransaction(asset.ID, transaction)
		}
	}
}

func (asset *Asset) publishTransactionConfirmed(transactionHash string, blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotificationListener := range asset.txAndBlockNotificationListeners {
		if txAndBlockNotificationListener.OnTransactionConfirmed != nil {
			go txAndBlockNotificationListener.OnTransactionConfirmed(asset.ID, transactionHash, blockHeight)
		}
	}
}

func (asset *Asset) publishBlockAttached(blockHeight int32) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, txAndBlockNotificationListener := range asset.txAndBlockNotificationListeners {
		if txAndBlockNotificationListener.OnBlockAttached != nil {
			go txAndBlockNotificationListener.OnBlockAttached(asset.ID, blockHeight)
		}
	}
}

func (asset *Asset) IsNotificationListenerExist(uniqueIdentifier string) bool {
	_, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]
	return ok
}
