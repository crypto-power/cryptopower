package libwallet

import (
	"encoding/json"

	"decred.org/dcrwallet/v2/errors"
)

func (mw *MultiWallet) listenForTransactions(walletID int) {
	go func() {

		wallet := mw.wallets[walletID]
		n := wallet.Internal().NtfnServer.TransactionNotifications()

		for {
			select {
			case v := <-n.C:
				if v == nil {
					return
				}
				for _, transaction := range v.UnminedTransactions {
					tempTransaction, err := wallet.decodeTransactionWithTxSummary(&transaction, nil)
					if err != nil {
						log.Errorf("[%d] Error ntfn parse tx: %v", wallet.ID, err)
						return
					}

					overwritten, err := wallet.walletDataDB.SaveOrUpdate(&Transaction{}, tempTransaction)
					if err != nil {
						log.Errorf("[%d] New Tx save err: %v", wallet.ID, err)
						return
					}

					if !overwritten {
						log.Infof("[%d] New Transaction %s", wallet.ID, tempTransaction.Hash)

						result, err := json.Marshal(tempTransaction)
						if err != nil {
							log.Error(err)
						} else {
							mw.mempoolTransactionNotification(string(result))
						}
					}
				}

				for _, block := range v.AttachedBlocks {
					blockHash := block.Header.BlockHash()
					for _, transaction := range block.Transactions {
						tempTransaction, err := wallet.decodeTransactionWithTxSummary(&transaction, &blockHash)
						if err != nil {
							log.Errorf("[%d] Error ntfn parse tx: %v", wallet.ID, err)
							return
						}

						_, err = wallet.walletDataDB.SaveOrUpdate(&Transaction{}, tempTransaction)
						if err != nil {
							log.Errorf("[%d] Incoming block replace tx error :%v", wallet.ID, err)
							return
						}
						mw.publishTransactionConfirmed(wallet.ID, transaction.Hash.String(), int32(block.Header.Height))
					}

					mw.publishBlockAttached(wallet.ID, int32(block.Header.Height))
				}

				if len(v.AttachedBlocks) > 0 {
					mw.checkWalletMixers()
				}

			case <-mw.syncData.syncCanceled:
				n.Done()
			}
		}
	}()
}

// AddTxAndBlockNotificationListener registers a set of functions to be invoked
// when a transaction or block update is processed by the wallet. If async is
// true, the provided callback methods will be called from separate goroutines,
// allowing notification senders to continue their operation without waiting
// for the listener to complete processing the notification. This asyncrhonous
// handling is especially important for cases where the wallet process that
// sends the notification temporarily prevents access to other wallet features
// until all notification handlers finish processing the notification. If a
// notification handler were to try to access such features, it would result
// in a deadlock.
func (mw *MultiWallet) AddTxAndBlockNotificationListener(txAndBlockNotificationListener TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	mw.notificationListenersMu.Lock()
	defer mw.notificationListenersMu.Unlock()

	_, ok := mw.txAndBlockNotificationListeners[uniqueIdentifier]
	if ok {
		return errors.New(ErrListenerAlreadyExist)
	}

	if async {
		mw.txAndBlockNotificationListeners[uniqueIdentifier] = &asyncTxAndBlockNotificationListener{
			l: txAndBlockNotificationListener,
		}
	} else {
		mw.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	}

	return nil
}

func (mw *MultiWallet) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	mw.notificationListenersMu.Lock()
	defer mw.notificationListenersMu.Unlock()

	delete(mw.txAndBlockNotificationListeners, uniqueIdentifier)
}

func (mw *MultiWallet) checkWalletMixers() {
	for _, wallet := range mw.wallets {
		if wallet.IsAccountMixerActive() {
			unmixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)
			hasMixableOutput, err := wallet.accountHasMixableOutput(unmixedAccount)
			if err != nil {
				log.Errorf("Error checking for mixable outputs: %v", err)
			}

			if !hasMixableOutput {
				log.Infof("[%d] unmixed account does not have a mixable output, stopping account mixer", wallet.ID)
				err = mw.StopAccountMixer(wallet.ID)
				if err != nil {
					log.Errorf("Error stopping account mixer: %v", err)
				}
			}
		}
	}
}

func (mw *MultiWallet) mempoolTransactionNotification(transaction string) {
	mw.notificationListenersMu.RLock()
	defer mw.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range mw.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransaction(transaction)
	}
}

func (mw *MultiWallet) publishTransactionConfirmed(walletID int, transactionHash string, blockHeight int32) {
	mw.notificationListenersMu.RLock()
	defer mw.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range mw.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransactionConfirmed(walletID, transactionHash, blockHeight)
	}
}

func (mw *MultiWallet) publishBlockAttached(walletID int, blockHeight int32) {
	mw.notificationListenersMu.RLock()
	defer mw.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range mw.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnBlockAttached(walletID, blockHeight)
	}
}
