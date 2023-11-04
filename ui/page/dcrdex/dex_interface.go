package dcrdex

import (
	"decred.org/dcrdex/client/core"
)

type clientCore interface {
	Ready() <-chan struct{}
	WaitForShutdown() <-chan struct{}
	IsDEXPasswordSet() bool
	SetDEXPassword(pw, seed []byte) error
	Login(pw []byte) error
	DiscoverAccount(dexAddr string, pass []byte, certI any) (*core.Exchange, bool, error)
	BondsFeeBuffer(assetID uint32) uint64
	PostBond(form *core.PostBondForm) (*core.PostBondResult, error)
	NotificationFeed() <-chan core.Notification
}
