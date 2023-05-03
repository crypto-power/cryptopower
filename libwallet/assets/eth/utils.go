package eth

import (
	"strconv"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
)

// Wei is the smallest unit of payment accepted on ethereum.
// 1 ether = 1,000,000,000 Gwei (1e9).
// 1 ether = 1,000,000,000,000,000,000 wei (1e18).
const ethTowei = 1e18

// Amount implement the sharedW AssetAmount interface within ethereum.
type Amount int64

// ToCoin returns an asset formatted amount in float64.
func (a Amount) ToCoin() float64 {
	if a == 0 {
		return 0
	}
	return float64(a) / ethTowei
}

// String returns an asset formatted amount in string.
func (a Amount) String() string {
	strVal := strconv.FormatFloat(a.ToCoin(), 'f', 0, 64)
	return strVal + " ETH"
}

// MulF64 multiplies an Amount by a floating point value.
func (a Amount) MulF64(f float64) sharedW.AssetAmount {
	return Amount(int64(a.ToCoin() * f))
}

// ToInt() returns the complete int64 value without formatting.
func (a Amount) ToInt() int64 {
	return int64(a)
}
