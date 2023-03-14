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

// SetAPIFeeRate validates the string input its a number before sending it upstream.
// It returns the string convert to int amount.
func (w *WalletMapping) SetAPIFeeRate(feerate string) (int64, error) {
	switch asset := w.Asset.(type) {
	case *btc.Asset:
		rate, err := strconv.ParseInt(feerate, 10, 64)
		if err != nil {
			return 0, w.invalidParameter(feerate, "tx fee rate")
		}
		err = asset.SetUserFeeRate(asset.ToAmount(rate))
		return rate, err
	default:
		return 0, w.invalidWallet()
	}
}

func (w *WalletMapping) GetAPIFeeRate() ([]btc.FeeEstimate, error) {
	switch asset := w.Asset.(type) {
	case *btc.Asset:
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
