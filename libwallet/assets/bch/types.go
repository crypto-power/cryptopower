package bch

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/gcash/bchutil"
)

// Amount implements the Asset amount interface for the BCH asset
type Amount bchutil.Amount

// ToCoin returns the float64 version of the BCH formatted asset amount.
func (a Amount) ToCoin() float64 {
	return bchutil.Amount(a).ToBCH()
}

// String returns the string version of the BCH formatted asset amount.
func (a Amount) String() string {
	return bchutil.Amount(a).String()
}

// MulF64 multiplys the Amount with the provided float64 value.
func (a Amount) MulF64(f float64) sharedW.AssetAmount {
	return Amount(bchutil.Amount(a).MulF64(f))
}

// ToInt return the original unformatted amount LTCs
func (a Amount) ToInt() int64 {
	return int64(bchutil.Amount(a))
}
