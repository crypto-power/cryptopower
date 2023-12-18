package dcrdex

import (
	"decred.org/dcrdex/client/core"
)

type dexClient interface {
	Ready() <-chan struct{}
	WaitForShutdown() <-chan struct{}
	IsDEXPasswordSet() bool
	InitWithPassword(pw, seed []byte) error
	Login(pw []byte) error
	GetDEXConfig(dexAddr string, certI any) (*core.Exchange, error)
	BondsFeeBuffer(assetID uint32) uint64
	HasWallet(assetID int32) bool
	AddWallet(assetID uint32, settings map[string]string, appPW, walletPW []byte) error
	PostBond(form *core.PostBondForm) (*core.PostBondResult, error)
	NotificationFeed() <-chan core.Notification
}
