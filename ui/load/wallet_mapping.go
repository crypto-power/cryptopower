package load

import (
	"fmt"
	"strconv"

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

func (w *WalletMapping) MixedAccountNumber() int32 {
	switch asset := w.Asset.(type) {
	case *dcr.DCRAsset:
		return asset.MixedAccountNumber()
	default:
		return -1
	}
}

func (w *WalletMapping) Broadcast(passphrase string) error {
	switch asset := w.Asset.(type) {
	case *dcr.DCRAsset:
		_, err := asset.Broadcast(passphrase)
		return err
	case *btc.BTCAsset:
		err := asset.Broadcast(passphrase, "")
		return err
	default:
		return w.invalidWallet()
	}
}

func (w *WalletMapping) SetAPIFeeRate(feerate string) error {
	switch asset := w.Asset.(type) {
	case *btc.BTCAsset:
		rate, err := strconv.ParseInt(feerate, 10, 64)
		if err != nil {
			return w.invalidParameter(feerate, "tx fee rate")
		}
		return asset.SetUserFeeRate(asset.ToAmount(rate))
	default:
		return w.invalidWallet()
	}
}

func (w *WalletMapping) GetAPIFeeRate() ([]btc.FeeEstimate, error) {
	switch asset := w.Asset.(type) {
	case *btc.BTCAsset:
		return asset.GetAPIFeeEstimateRate()
	default:
		return nil, w.invalidWallet()
	}
}

func (w *WalletMapping) invalidWallet() error {
	return fmt.Errorf("(%v) wallet not supported", w.Asset.GetAssetType())
}

func (w *WalletMapping) invalidParameter(v interface{}, vType string) error {
	return fmt.Errorf("(%v) not valid %v", v, vType)
}
