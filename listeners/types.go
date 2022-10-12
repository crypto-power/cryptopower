package listeners

import "gitlab.com/raedah/cryptopower/libwallet/wallets/wallet"

type TxNotifType int

const (
	// Transaction notification types
	NewTransaction TxNotifType = iota // 0 = New transaction.
	BlockAttached                     // 1 = block attached.
	TxConfirmed                       // 2 = Transaction confirmed.
)

// TxNotification models transaction notifications.
type TxNotification struct {
	Type        TxNotifType
	Transaction *wallet.Transaction
	WalletID    int
	BlockHeight int32
	Hash        string
}
