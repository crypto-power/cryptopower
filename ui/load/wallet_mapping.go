package load

import (
	"fmt"
	"strconv"

	"gitlab.com/cryptopower/cryptopower/libwallet/assets/btc"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/dcr"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/ltc"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
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
	var amount sharedW.AssetAmount
	var setUserFeeRate func(feeRatePerkvB sharedW.AssetAmount) error

	rate, err := strconv.ParseInt(feerate, 10, 64)
	if err != nil {
		return 0, w.invalidParameter(feerate, "tx fee rate")
	}

	switch asset := w.Asset.(type) {
	case *btc.Asset:
		amount = asset.ToAmount(rate)
		setUserFeeRate = asset.SetUserFeeRate
	case *ltc.Asset:
		amount = asset.ToAmount(rate)
		setUserFeeRate = asset.SetUserFeeRate
	default:
		return 0, w.invalidWallet()
	}

	err = setUserFeeRate(amount)
	return rate, err
}

func (w *WalletMapping) GetAPIFeeRate() ([]sharedW.FeeEstimate, error) {
	switch asset := w.Asset.(type) {
	case *btc.Asset:
		return asset.GetAPIFeeEstimateRate()
	case *ltc.Asset:
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
