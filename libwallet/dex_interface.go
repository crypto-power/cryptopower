package libwallet

import (
	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/client/orderbook"
	"decred.org/dcrdex/dex"
)

type DexClient interface {
	Ready() <-chan struct{}
	WaitForShutdown() <-chan struct{}
	Shutdown()
	IsDEXPasswordSet() bool
	IsLoggedIn() bool
	InitWithPassword(pw, seed []byte) error
	Login(pw []byte) error
	Logout() error
	DBPath() string
	DiscoverAccount(dexAddr string, appPW []byte, certI any) (*core.Exchange, bool, error)
	GetDEXConfig(dexAddr string, certI any) (*core.Exchange, error)
	BondsFeeBuffer(assetID uint32) uint64
	HasWallet(assetID int32) bool
	AddWallet(assetID uint32, settings map[string]string, appPW, walletPW []byte) error
	PostBond(form *core.PostBondForm) (*core.PostBondResult, error)
	NotificationFeed() *core.NoteFeed
	Exchanges() map[string]*core.Exchange
	Exchange(host string) (*core.Exchange, error)
	ExportSeed(pw []byte) ([]byte, error)
	SyncBook(dex string, base, quote uint32) (*orderbook.OrderBook, core.BookFeed, error)
	Orders(filter *core.OrderFilter) ([]*core.Order, error)
	TradeAsync(pw []byte, form *core.TradeForm) (*core.InFlightOrder, error)
	WalletState(assetID uint32) *core.WalletState
	WalletSettings(assetID uint32) (map[string]string, error)
	WalletIDForAsset(assetID uint32) (*int, error)
	MaxBuy(host string, base, quote uint32, rate uint64) (*core.MaxOrderEstimate, error)
	MaxSell(host string, base, quote uint32) (*core.MaxOrderEstimate, error)
	Cancel(oid dex.Bytes) error
}
