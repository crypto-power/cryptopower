package btc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"github.com/btcsuite/btcutil"
)

// BTCAmount implements the Asset amount interface for the BTC asset
type BTCAmount btcutil.Amount

// ToCoin returns the float64 version of the BTC formatted asset amount.
func (a BTCAmount) ToCoin() float64 {
	return btcutil.Amount(a).ToBTC()
}

// String returns the string version of the BTC formatted asset amount.
func (a BTCAmount) String() string {
	return btcutil.Amount(a).String()
}

// MulF64 multiplys the BTCAmount with the provided float64 value.
func (a BTCAmount) MulF64(f float64) sharedW.AssetAmount {
	return BTCAmount(btcutil.Amount(a).MulF64(f))
}

// ToInt return the original unformatted amount BTCs
func (a BTCAmount) ToInt() int64 {
	return int64(btcutil.Amount(a))
}

type ListUnspentResult struct {
	TxID          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Account       string  `json:"account"`
	ScriptPubKey  string  `json:"scriptPubKey"`
	RedeemScript  string  `json:"redeemScript,omitempty"`
	Amount        float64 `json:"amount"`
	Confirmations int64   `json:"confirmations"`
	Spendable     bool    `json:"spendable"`
}
