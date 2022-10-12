package btc

import (
	"github.com/btcsuite/btcwallet/waddrmgr"
)

const (
	TestnetHDPath = "m / 44' / 1' / "  // TODO: confirm if this is the correct HD path for btc
	MainnetHDPath = "m / 44' / 42' / " // TODO: confirm if this is the correct HD path for btc

	DefaultRequiredConfirmations = 6
)

func (wallet *Wallet) RequiredConfirmations() int32 {
	return DefaultRequiredConfirmations
}

func (wallet *Wallet) GetScope() waddrmgr.KeyScope {
	return waddrmgr.KeyScope{}
}
