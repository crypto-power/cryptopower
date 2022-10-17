package btc

import (
	"github.com/btcsuite/btcwallet/waddrmgr"
)

const (
	TestnetHDPath = "m / 44' / 1' / " // TODO: confirm if this is the correct HD path for btc
	MainnetHDPath = "m / 44' / 0' / " // TODO: confirm if this is the correct HD path for btc
)

func (asset *BTCAsset) GetScope() waddrmgr.KeyScope {
	// Construct the key scope that will be used within the waddrmgr to
	// create an HD chain for deriving all of our required keys. A different
	// scope is used for each specific coin type.
	return waddrmgr.KeyScopeBIP0084
}
