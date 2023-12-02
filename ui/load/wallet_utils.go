package load

import (
	"fmt"
	"strconv"

	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/assets/ltc"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
)

func MixedAccountNumber(w sharedW.Asset) int32 {
	switch asset := w.(type) {
	case *dcr.Asset:
		return asset.MixedAccountNumber()
	default:
		return -1
	}
}

// SetAPIFeeRate validates the string input its a number before sending it upstream.
// It returns the string convert to int amount.
func SetAPIFeeRate(w sharedW.Asset, feerate string) (int64, error) {
	var amount sharedW.AssetAmount
	var setUserFeeRate func(feeRatePerkvB sharedW.AssetAmount) error

	rate, err := strconv.ParseInt(feerate, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("(%v) not valid tx fee rate", feerate)
	}

	switch asset := w.(type) {
	case *btc.Asset:
		amount = asset.ToAmount(rate)
		setUserFeeRate = asset.SetUserFeeRate
	case *ltc.Asset:
		amount = asset.ToAmount(rate)
		setUserFeeRate = asset.SetUserFeeRate
	default:
		return 0, fmt.Errorf("(%v) wallet not supported", w.GetAssetType())
	}

	err = setUserFeeRate(amount)
	return rate, err
}

func GetAPIFeeRate(w sharedW.Asset) ([]sharedW.FeeEstimate, error) {
	switch asset := w.(type) {
	case *btc.Asset:
		return asset.GetAPIFeeEstimateRate()
	case *ltc.Asset:
		return asset.GetAPIFeeEstimateRate()
	default:
		return nil, fmt.Errorf("(%v) wallet not supported", w.GetAssetType())
	}
}
