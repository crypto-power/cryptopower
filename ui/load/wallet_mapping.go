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
