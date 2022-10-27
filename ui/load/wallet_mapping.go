package load

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
)

// WalletMapping helps to call a function quickly no matter what currency it is,
// it is used for separate functions without an interface for general use
type WalletMapping struct {
	sharedW.Asset
}

func NewWalletMapping(asset sharedW.Asset) *WalletMapping {
	return &WalletMapping{
		Asset: asset,
	}
}
