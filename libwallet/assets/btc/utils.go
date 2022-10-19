package btc

import (
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/waddrmgr"
)

const (
	maxAmountSatoshi = btcutil.MaxSatoshi // MaxSatoshi is the maximum transaction amount allowed in satoshi.

	TestnetHDPath = "m / 44' / 1' / " // TODO: confirm if this is the correct HD path for btc
	MainnetHDPath = "m / 44' / 0' / " // TODO: confirm if this is the correct HD path for btc
)

func (asset *BTCAsset) GetScope() waddrmgr.KeyScope {
	// Construct the key scope that will be used within the waddrmgr to
	// create an HD chain for deriving all of our required keys. A different
	// scope is used for each specific coin type.
	return waddrmgr.KeyScopeBIP0084
}

func AmountBTC(amount int64) float64 {
	return btcutil.Amount(amount).ToBTC()
}

func AmountSatoshi(f float64) int64 {
	amount, err := btcutil.NewAmount(f)
	if err != nil {
		log.Error(err)
		return -1
	}
	return int64(amount)
}
