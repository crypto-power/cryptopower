package ltc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
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
func (asset *Asset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener,
	async bool, uniqueIdentifier string) error {
	return utils.ErrLTCMethodNotImplemented("AddTxAndBlockNotificationListener")
}

// RemoveTxAndBlockNotificationListener removes a previously registered
// transaction and block notification listener.
func (asset *Asset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {

}
