package btc

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcwallet/chain"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/lightninglabs/neutrino"
)

// Amount implements the Asset amount interface for the BTC asset
type Amount btcutil.Amount

// ToCoin returns the float64 version of the BTC formatted asset amount.
func (a Amount) ToCoin() float64 {
	return btcutil.Amount(a).ToBTC()
}

// String returns the string version of the BTC formatted asset amount.
func (a Amount) String() string {
	return btcutil.Amount(a).String()
}

// MulF64 multiplys the Amount with the provided float64 value.
func (a Amount) MulF64(f float64) sharedW.AssetAmount {
	return Amount(btcutil.Amount(a).MulF64(f))
}

// ToInt return the original unformatted amount BTCs
func (a Amount) ToInt() int64 {
	return int64(btcutil.Amount(a))
}

// ExtraNeutrinoChainService includes the extra method declarations not available in
// https://github.com/btcsuite/btcwallet/blob/db3a4a2543bdc7ebc1e992b2f7fd01e4abd257a3/chain/chainservice.go#L16-L38
// but are required for use in this context.
type ExtraNeutrinoChainService interface {
	chain.NeutrinoChainService

	ConnectedCount() int32
	Peers() []*neutrino.ServerPeer
}
