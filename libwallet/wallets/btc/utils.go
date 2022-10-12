package btc

import (
	"github.com/btcsuite/btcwallet/waddrmgr"
)

const (
	DefaultRequiredConfirmations = 6
)

func (wallet *Wallet) RequiredConfirmations() int32 {
	return DefaultRequiredConfirmations
}

func (wallet *Wallet) GetScope() waddrmgr.KeyScope {
	return waddrmgr.KeyScope{}
}
