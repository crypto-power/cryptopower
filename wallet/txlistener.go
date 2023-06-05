package wallet

import sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"

// NewBlock is sent when a block is attached to the assetsManager.
type NewBlock struct {
	WalletID int
	Height   int32
}

// TxConfirmed is sent when a transaction is confirmed.
type TxConfirmed struct {
	WalletID int
	Height   int32
	Hash     string
}

type NewTransaction struct {
	Transaction *sharedW.Transaction
}
