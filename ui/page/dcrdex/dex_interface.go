package dcrdex

import (
	"decred.org/dcrdex/client/core"
)

type dexClient interface {
	Ready() <-chan struct{}
	WaitForShutdown() <-chan struct{}
	IsDEXPasswordSet() bool
	IsLoggedIn() bool
	InitWithPassword(pw, seed []byte) error
	Login(pw []byte) error
	GetDEXConfig(dexAddr string, certI any) (*core.Exchange, error)
	BondsFeeBuffer(assetID uint32) uint64
	HasWallet(assetID int32) bool
	AddWallet(assetID uint32, settings map[string]string, appPW, walletPW []byte) error
	PostBond(form *core.PostBondForm) (*core.PostBondResult, error)
	NotificationFeed() *core.NoteFeed
	Exchanges() map[string]*core.Exchange
	Exchange(host string) (*core.Exchange, error)
	Book(dex string, base, quote uint32) (*core.OrderBook, error)
	Orders(filter *core.OrderFilter) ([]*core.Order, error)
	TradeAsync(pw []byte, form *core.TradeForm) (*core.InFlightOrder, error)
	WalletState(assetID uint32) *core.WalletState
	PreOrder(*core.TradeForm) (*core.OrderEstimate, error)
}
