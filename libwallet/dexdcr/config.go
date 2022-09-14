// Copyright (c) 2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dexdcr

import (
	"decred.org/dcrdex/client/asset"
)

const (
	// defaultFee is the default value for the fallbackfee.
	defaultFee = 20
	// defaultFeeRateLimit is the default value for the feeratelimit.
	defaultFeeRateLimit = 100
	// defaultRedeemConfTarget is the default redeem transaction confirmation
	// target in blocks used by estimatesmartfee to get the optimal fee for a
	// redeem transaction.
	defaultRedeemConfTarget = 1
)

// DefaultConfigOpts are the general, non-rpc-specific configuration options
// defined in the decred.org/dcrdex/client/asset/dcr package.
var DefaultConfigOpts = []*asset.ConfigOption{
	{
		Key:         "account",
		DisplayName: "Account Name",
		Description: "dcrwallet account name",
	},
	{
		Key:         "fallbackfee",
		DisplayName: "Fallback fee rate",
		Description: "The fee rate to use for fee payment and withdrawals when " +
			"estimatesmartfee is not available. Units: DCR/kB",
		DefaultValue: defaultFee * 1000 / 1e8,
	},
	{
		Key:         "feeratelimit",
		DisplayName: "Highest acceptable fee rate",
		Description: "This is the highest network fee rate you are willing to " +
			"pay on swap transactions. If feeratelimit is lower than a market's " +
			"maxfeerate, you will not be able to trade on that market with this " +
			"wallet.  Units: DCR/kB",
		DefaultValue: defaultFeeRateLimit * 1000 / 1e8,
	},
	{
		Key:         "redeemconftarget",
		DisplayName: "Redeem confirmation target",
		Description: "The target number of blocks for the redeem transaction " +
			"to get a confirmation. Used to set the transaction's fee rate." +
			" (default: 1 block)",
		DefaultValue: defaultRedeemConfTarget,
	},
	{
		Key:         "txsplit",
		DisplayName: "Pre-size funding inputs",
		Description: "When placing an order, create a \"split\" transaction to " +
			"fund the order without locking more of the wallet balance than " +
			"necessary. Otherwise, excess funds may be reserved to fund the order " +
			"until the first swap contract is broadcast during match settlement, or " +
			"the order is canceled. This an extra transaction for which network " +
			"mining fees are paid.  Used only for standing-type orders, e.g. " +
			"limit orders without immediate time-in-force.",
		IsBoolean: true,
	},
}
