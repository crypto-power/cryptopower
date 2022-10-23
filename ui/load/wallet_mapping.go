package load

import (
	"fmt"

	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
)

type WalletMapping struct {
	sharedW.Asset
}

func NewWalletMapping(asset sharedW.Asset) *WalletMapping {
	return &WalletMapping{
		Asset: asset,
	}
}

func (wallt *WalletMapping) IsDCR() bool {
	switch wallt.Asset.(type) {
	case *dcr.DCRAsset:
		return true
	default:
		return false
	}
}

func (wallt *WalletMapping) IsBTC() bool {
	switch wallt.Asset.(type) {
	case *btc.BTCAsset:
		return true
	default:
		return false
	}
}

func (wallt *WalletMapping) ID() int {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		return asset.ID
	case *btc.BTCAsset:
		return asset.ID
	default:
		return -1
	}
}

func (wallt *WalletMapping) Name() string {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		return asset.Name
	case *btc.BTCAsset:
		return asset.Name
	default:
		return ""
	}
}

func (wallt *WalletMapping) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		return asset.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, async, uniqueIdentifier)
	case *btc.BTCAsset:
		return fmt.Errorf("btc wallet not support this function")
	default:
		return fmt.Errorf("none type of wallet")
	}
}

func (wallt *WalletMapping) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) error {
	switch asset := wallt.Asset.(type) {
	case *dcr.DCRAsset:
		asset.RemoveTxAndBlockNotificationListener(uniqueIdentifier)
		return nil
	case *btc.BTCAsset:
		return fmt.Errorf("btc wallet not support this function")
	default:
		return fmt.Errorf("none type of wallet")
	}
}
