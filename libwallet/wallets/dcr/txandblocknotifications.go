package dcr

import (
	"encoding/json"

	"decred.org/dcrwallet/v2/errors"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	mainW "gitlab.com/raedah/cryptopower/libwallet/wallets/wallet"
)

func (wallet *Wallet) listenForTransactions() {
	go func() {

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

					overwritten, err := wallet.walletDataDB.SaveOrUpdate(&mainW.Transaction{}, tempTransaction)
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
							wallet.mempoolTransactionNotification(string(result))
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

						_, err = wallet.walletDataDB.SaveOrUpdate(&mainW.Transaction{}, tempTransaction)
						if err != nil {
							log.Errorf("[%d] Incoming block replace tx error :%v", wallet.ID, err)
							return
						}
						wallet.publishTransactionConfirmed(transaction.Hash.String(), int32(block.Header.Height))
					}

					wallet.publishBlockAttached(int32(block.Header.Height))
				}

				if len(v.AttachedBlocks) > 0 {
					wallet.checkWalletMixers()
				}

			case <-wallet.syncData.syncCanceled:
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
func (wallet *Wallet) AddTxAndBlockNotificationListener(txAndBlockNotificationListener mainW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	wallet.notificationListenersMu.Lock()
	defer wallet.notificationListenersMu.Unlock()

	_, ok := wallet.txAndBlockNotificationListeners[uniqueIdentifier]
	if ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	if async {
		wallet.txAndBlockNotificationListeners[uniqueIdentifier] = &asyncTxAndBlockNotificationListener{
			l: txAndBlockNotificationListener,
		}
	} else {
		wallet.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	}

	return nil
}

func (wallet *Wallet) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	wallet.notificationListenersMu.Lock()
	defer wallet.notificationListenersMu.Unlock()

	delete(wallet.txAndBlockNotificationListeners, uniqueIdentifier)
}

func (wallet *Wallet) checkWalletMixers() {
	if wallet.IsAccountMixerActive() {
		unmixedAccount := wallet.ReadInt32ConfigValueForKey(mainW.AccountMixerUnmixedAccount, -1)
		hasMixableOutput, err := wallet.accountHasMixableOutput(unmixedAccount)
		if err != nil {
			log.Errorf("Error checking for mixable outputs: %v", err)
		}

		if !hasMixableOutput {
			log.Infof("[%d] unmixed account does not have a mixable output, stopping account mixer", wallet.ID)
			err = wallet.StopAccountMixer()
			if err != nil {
				log.Errorf("Error stopping account mixer: %v", err)
			}
		}
	}
}

func (wallet *Wallet) mempoolTransactionNotification(transaction string) {
	wallet.notificationListenersMu.RLock()
	defer wallet.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range wallet.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransaction(transaction)
	}
}

func (wallet *Wallet) publishTransactionConfirmed(transactionHash string, blockHeight int32) {
	wallet.notificationListenersMu.RLock()
	defer wallet.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range wallet.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnTransactionConfirmed(wallet.ID, transactionHash, blockHeight)
	}
}

func (wallet *Wallet) publishBlockAttached(blockHeight int32) {
	wallet.notificationListenersMu.RLock()
	defer wallet.notificationListenersMu.RUnlock()

	for _, txAndBlockNotifcationListener := range wallet.txAndBlockNotificationListeners {
		txAndBlockNotifcationListener.OnBlockAttached(wallet.ID, blockHeight)
	}
}
