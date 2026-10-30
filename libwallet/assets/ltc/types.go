package ltc

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/ltcsuite/ltcd/ltcutil"
)

// Amount implements the Asset amount interface for the LTC asset
type Amount ltcutil.Amount

// ToCoin returns the float64 version of the LTC formatted asset amount.
func (a Amount) ToCoin() float64 {
	return ltcutil.Amount(a).ToBTC()
}

// String returns the string version of the LTC formatted asset amount.
func (a Amount) String() string {
	return ltcutil.Amount(a).String()
}

// MulF64 multiplys the Amount with the provided float64 value.
func (a Amount) MulF64(f float64) sharedW.AssetAmount {
	return Amount(ltcutil.Amount(a).MulF64(f))
}

// ToInt return the original unformatted amount LTCs
func (a Amount) ToInt() int64 {
	return int64(ltcutil.Amount(a))
}

type CSPPConfig struct {
	// Mixing option activates the new version of the coins mixer which is a
	// replacement of the old client-server mechanism. Now peer to peer
	// mechanism is in place. Ref: https://github.com/decred/dcrwallet/pull/2351
	Mixing             bool
	MixedAccount       uint32
	MixedAccountBranch uint32
	TicketSplitAccount uint32
	ChangeAccount      uint32
}

type AccountMixerNotificationListener struct {
	OnAccountMixerStarted func(walletID int)
	OnAccountMixerEnded   func(walletID int)
}