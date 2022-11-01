package load

import (
	"fmt"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
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

func (wallt *WalletMapping) MixedAccountNumber() int32 {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		return asset.MixedAccountNumber()
	default:
		return -1
	}
}

func (wallt *WalletMapping) Broadcast(passphrase string) error {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		_, err := asset.Broadcast(passphrase)
		return err
	case *btc.BTCAsset:
		err := asset.Broadcast(passphrase, "")
		return err
	default:
		return fmt.Errorf("wallet not supported")
	}
}
