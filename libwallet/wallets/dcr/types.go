package dcr

import "gitlab.com/raedah/cryptopower/libwallet/wallets/wallet"

/** begin tx-related types */

// asyncTxAndBlockNotificationListener is a TxAndBlockNotificationListener that
// triggers notifcation callbacks asynchronously.
type asyncTxAndBlockNotificationListener struct {
	l wallet.TxAndBlockNotificationListener
}

// OnTransaction satisfies the TxAndBlockNotificationListener interface and
// starts a goroutine to actually handle the notification using the embedded
// listener.
func (asyncTxBlockListener *asyncTxAndBlockNotificationListener) OnTransaction(transaction string) {
	go asyncTxBlockListener.l.OnTransaction(transaction)
}

// OnBlockAttached satisfies the TxAndBlockNotificationListener interface and
// starts a goroutine to actually handle the notification using the embedded
// listener.
func (asyncTxBlockListener *asyncTxAndBlockNotificationListener) OnBlockAttached(walletID int, blockHeight int32) {
	go asyncTxBlockListener.l.OnBlockAttached(walletID, blockHeight)
}

// OnTransactionConfirmed satisfies the TxAndBlockNotificationListener interface
// and starts a goroutine to actually handle the notification using the embedded
// listener.
func (asyncTxBlockListener *asyncTxAndBlockNotificationListener) OnTransactionConfirmed(walletID int, hash string, blockHeight int32) {
	go asyncTxBlockListener.l.OnTransactionConfirmed(walletID, hash, blockHeight)
}
