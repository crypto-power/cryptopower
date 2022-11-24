package btc

import (
	"fmt"
	"strconv"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
)

const (
	MainnetAPIFeeRateURL = "https://blockstream.info/api/fee-estimates"
	TestnetAPIFeeRateURL = "https://blockstream.info/testnet/api/fee-estimates"
)

// Since the introduction of segwit account different size measument was
// introduced (Sat/VB). When sending a transaction from the legacy account,
// 1B (byte) = 1vB (virtual byte). When sending a transaction from segwit
// (legacy segwit, bech32, taproot), then 1B = 4vB.

// 1,000 sat/kvB = 1 sat/vB
// 1 sat/vB = 0.25 sat/wu
// 0.25 sat/wu = 250 sat/kwu
// 20 sat/vB = 5,000 sat/kwu

// 1vB = 0.0001 kvB
// 1 BTC = 10 ^ 8 Sats = 100,000,000 Sats.

var FallBackFeeRatePerkvB sharedW.AssetAmount = BTCAmount(1000) // Equals to 1 sat/vB.

// feeEstimateCache helps to cache the resolved fee rate until a new
// block is mined
type feeEstimateCache struct {
	// SetFeeRate defines the fee rate if set, the user wants to apply for
	// all his transactions.
	SetFeeRatePerKvB sharedW.AssetAmount
	// If not empty, they hold the fee rate queries from the API when the best
	// block was set at LastBestBlock.
	APIFeeRates []FeeEstimate
	// LastBestblock defines the last height where Max and Min fee rate were last
	// evaluated. This helps to keep the API calls to fetch the fee rate under
	// control.
	LastBestblock int32

	mu sync.RWMutex
}

type FeeEstimate struct {
	// Number of confrmed blocks that show the average fee rate below.
	ConfirmedBlocks int32
	// Feerate shows estimate fee rate in Sat/kvB.
	Feerate sharedW.AssetAmount
}

// fetchAPIFeeRate queries the API fee rate for the provided blocks.
func (asset *BTCAsset) fetchAPIFeeRate() ([]FeeEstimate, error) {
	var feerateURL string
	net := asset.NetType()
	switch net {
	case utils.Mainnet:
		feerateURL = MainnetAPIFeeRateURL
	case utils.Testnet:
		feerateURL = TestnetAPIFeeRateURL
	default:
		return nil, fmt.Errorf("%v network is not supported", net)
	}

	var resp = make(map[string]float64, 0)

	if _, _, err := utils.HttpGet(feerateURL, &resp); err != nil {
		return nil, fmt.Errorf("fetching API fee estimates failed: %v", err)
	}

	// if no data was returned, return an error.
	if len(resp) <= 0 {
		return nil, errors.New("API fee estimates not found")
	}

	var results = make([]FeeEstimate, 0, len(resp))

	// Fee rate returned is in Sat/vB units.
	for blocks, feerate := range resp {
		vals, err := strconv.ParseInt(blocks, 10, 64)
		if err != nil {
			// Invalid blocks confirmation found ignore it,
			continue
		}

		results = append(results, FeeEstimate{
			ConfirmedBlocks: int32(vals),
			// Fee rate conversion from Sat/vB to Sat/kvB is at the rate of
			// 1000 Sat/kvB == 1 Sat/vB
			Feerate: BTCAmount(int(feerate * 1000.0)),
		})
	}
	return results, nil
}

func (asset *BTCAsset) GetAPIFeeEstimateRate() ([]FeeEstimate, error) {
	asset.fees.mu.RLock()
	// If best block hasn't changed, return the cached estimates.
	if asset.GetBestBlockHeight() == asset.fees.LastBestblock &&
		asset.fees.LastBestblock > 0 {
		defer asset.fees.mu.RUnlock()
		return asset.fees.APIFeeRates, nil
	}

	feerates, err := asset.fetchAPIFeeRate()
	if err != nil {
		return nil, err
	}

	// Do not cache empty results.
	if len(feerates) == 0 {
		return nil, errors.New("API feerates not available")
	}

	asset.fees.mu.Lock()
	asset.fees.APIFeeRates = feerates
	asset.fees.LastBestblock = asset.GetBestBlockHeight()
	asset.fees.mu.Unlock()

	return feerates, nil
}

// SetUserFeeRate sets the fee rate in kvB units. Setting fee rate less than
// FallBackFeeRatePerkvB is not allowed.
func (asset *BTCAsset) SetUserFeeRate(feeRatePerkvB sharedW.AssetAmount) error {
	asset.fees.mu.Lock()
	defer asset.fees.mu.Unlock()

	if feeRatePerkvB.ToInt() < FallBackFeeRatePerkvB.ToInt() {
		return fmt.Errorf("minimum rate is %v Sat/kvB", FallBackFeeRatePerkvB)
	}

	asset.fees.SetFeeRatePerKvB = feeRatePerkvB
	return nil
}

// GetUserFeeRate returns the fee rate in kvB units. If not set it defaults to
// 1000 Sats/kvB if appropriate fee rate hasn't been set.
func (asset *BTCAsset) GetUserFeeRate() sharedW.AssetAmount {
	asset.fees.mu.RLock()
	defer asset.fees.mu.RUnlock()

	if asset.fees.SetFeeRatePerKvB == nil {
		// If not set, defaults to the fall back fee of 1000 sats/kvB = (1 Sat/vB)
		return FallBackFeeRatePerkvB
	}
	return asset.fees.SetFeeRatePerKvB
}
