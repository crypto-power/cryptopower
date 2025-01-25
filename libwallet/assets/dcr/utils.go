package dcr

import (
	"fmt"
	"math"
	"time"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/dcrutil/v4"
)

const (
	// fetchPercentage is used to increase the initial estimate gotten during cfilters stage
	fetchPercentage = 0.38

	// Use 10% of estimated total headers fetch time to estimate rescan time
	rescanPercentage = 0.1

	// Use 80% of estimated total headers fetch time to estimate address discovery time
	discoveryPercentage = 0.8

	TestnetHDPath       = "m / 44' / 1' / "
	LegacyTestnetHDPath = "m / 44’ / 11’ / "
	MainnetHDPath       = "m / 44' / 42' / "
	LegacyMainnetHDPath = "m / 44’ / 20’ / "

	// GenesisTimestampMainnet represents the genesis timestamp for the DCR mainnet.
	GenesisTimestampMainnet = 1454954400
	// GenesisTimestampTestnet represents the genesis timestamp for the DCR testnet.
	GenesisTimestampTestnet = 1533513600
	// TargetTimePerBlockMainnet represents the target time per block in seconds for DCR mainnet.
	TargetTimePerBlockMainnet = 300
	// TargetTimePerBlockTestnet represents the target time per block in seconds for DCR testnet.
	TargetTimePerBlockTestnet = 120
)

// Returns a DCR amount that implements the asset amount interface.
func (asset *Asset) ToAmount(v int64) sharedW.AssetAmount {
	return Amount(dcrutil.Amount(v))
}

func AmountAtom(f float64) int64 {
	amount, err := dcrutil.NewAmount(f)
	if err != nil {
		log.Error(err)
		return -1
	}
	return int64(amount)
}

func calculateTotalTimeRemaining(timeRemainingInSeconds time.Duration) string {
	minutes := timeRemainingInSeconds / 60
	if minutes > 0 {
		return fmt.Sprintf("%d min", minutes)
	}
	return fmt.Sprintf("%d sec", timeRemainingInSeconds)
}

func secondsToDuration(secs float64) time.Duration {
	return time.Duration(secs) * time.Second
}

func roundUp(n float64) int32 {
	return int32(math.Round(n))
}

// GetGenesisTimestamp returns the genesis timestamp for the provided network.
func GetGenesisTimestamp(network utils.NetworkType) int64 {
	switch network {
	case utils.Mainnet:
		return GenesisTimestampMainnet
	case utils.Testnet:
		return GenesisTimestampTestnet
	}
	return 0
}

// GetTargetTimePerBlock returns the target time per block for the provided network.
func GetTargetTimePerBlock(network utils.NetworkType) int64 {
	switch network {
	case utils.Mainnet:
		return TargetTimePerBlockMainnet
	case utils.Testnet:
		return TargetTimePerBlockTestnet
	}
	return 0
}
