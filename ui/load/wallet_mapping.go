package load

import (
	"fmt"

	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
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

func (wallt *WalletMapping) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		return asset.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, async, uniqueIdentifier)
	case *btc.BTCAsset:
		return fmt.Errorf("btc wallet does not support this function")
	default:
		return fmt.Errorf("wallet not supported")
	}
}

func (wallt *WalletMapping) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) error {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		asset.RemoveTxAndBlockNotificationListener(uniqueIdentifier)
		return nil
	case *btc.BTCAsset:
		return fmt.Errorf("btc wallet does not support this function")
	default:
		return fmt.Errorf("wallet not supported")
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
