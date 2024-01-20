package dcrdex

import (
	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/client/orderbook"
	"decred.org/dcrdex/dex"
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
	WalletIDForAsset(assetID uint32) (*int, error)
	AddWallet(assetID uint32, settings map[string]string, appPW, walletPW []byte) error
	PostBond(form *core.PostBondForm) (*core.PostBondResult, error)
	NotificationFeed() *core.NoteFeed
	Exchanges() map[string]*core.Exchange
	Exchange(host string) (*core.Exchange, error)
	SyncBook(dex string, base, quote uint32) (*orderbook.OrderBook, core.BookFeed, error)
	Orders(filter *core.OrderFilter) ([]*core.Order, error)
	TradeAsync(pw []byte, form *core.TradeForm) (*core.InFlightOrder, error)
	WalletState(assetID uint32) *core.WalletState
	WalletSettings(assetID uint32) (map[string]string, error)
	MaxBuy(host string, base, quote uint32, rate uint64) (*core.MaxOrderEstimate, error)
	MaxSell(host string, base, quote uint32) (*core.MaxOrderEstimate, error)
	Cancel(oid dex.Bytes) error
}
