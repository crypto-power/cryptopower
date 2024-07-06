package btc

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"decred.org/dcrwallet/v4/errors"
	"github.com/btcsuite/btcd/btcutil"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

const (
	// MainnetAPIFeeRateURL defines the URL to fetch the mainnet fee rate from.
	MainnetAPIFeeRateURL = "https://blockstream.info/api/fee-estimates"
	// TestnetAPIFeeRateURL defines the URL to fetch the testnet fee rate from.
	TestnetAPIFeeRateURL = "https://blockstream.info/testnet/api/fee-estimates"

	// Since the introduction of segwit account, a different tx size measument was
	// introduced (Sat/VB). When sending a transaction from the legacy account,
	// 1B (byte) = 1vB (virtual byte). When sending a transaction from segwit
	// (legacy segwit, bech32, taproot), then 1B = 4vB.

	// 1,000 sat/kvB = 1 sat/vB
	// 1 sat/vB = 0.25 sat/wu
	// 0.25 sat/wu = 250 sat/kwu
	// 20 sat/vB = 5,000 sat/kwu

	// 1vB = 0.0001 kvB
	// 1 BTC = 10 ^ 8 Sats = 100,000,000 Sats.

	// FallBackFeeRatePerkvB defines the default fee rate to be used if API source of the
	// current fee rates fails. Fee rate in Sat/kvB => 50,000 Sat/kvB = 50 Sat/vB.
	// This feerate guarrantees relatively low fee cost and extremely fast tx
	// confirmation.
	FallBackFeeRatePerkvB btcutil.Amount = 50 * 1000

	// MinFeeRatePerkvB defines the minimum fee rate a user can set on a tx.
	MinFeeRatePerkvB btcutil.Amount = 1000 // Equals to 1 sat/vB.
)

// feeEstimateCache helps to cache the resolved fee rate until a new
// block is mined
type feeEstimateCache struct {
	// SetFeeRatePerkvB defines the fee rate. If set, the user wants to apply for
	// all his transactions.
	SetFeeRatePerkvB sharedW.AssetAmount
	// If not empty, they hold the fee rate queries from the API when the best
	// block was set at LastBestBlock.
	APIFeeRates []sharedW.FeeEstimate
	// LastBestblock defines the last height when results were cached. This
	// helps to keep the API calls to under control.
	LastBestblock int32

	mu sync.RWMutex
}

// fetchAPIFeeRate queries the API fee rate.
func (asset *Asset) fetchAPIFeeRate() ([]sharedW.FeeEstimate, error) {
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

	resp := make(map[string]float64, 0)

	req := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: feerateURL,
	}

	if _, err := utils.HTTPRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("fetching API fee estimates failed: %v", err)
	}

	// if no data was returned, return an error.
	if len(resp) == 0 {
		return nil, errors.New("API fee estimates not found")
	}

	results := make([]sharedW.FeeEstimate, 0, len(resp))

	// Fee rate returned is in Sat/vB units.
	for blocks, feerate := range resp {
		vals, err := strconv.ParseInt(blocks, 10, 64)
		if err != nil {
			// Invalid blocks confirmation found ignore it,
			continue
		}

		results = append(results, sharedW.FeeEstimate{
			ConfirmedBlocks: int32(vals),
			// Fee rate conversion from Sat/vB to Sat/kvB is at the rate of
			// 1000 Sat/kvB == 1 Sat/vB
			Feerate: Amount(int(feerate * 1000.0)),
		})
	}
	return results, nil
}

// GetAPIFeeEstimateRate returns the fee estimates from the API.
func (asset *Asset) GetAPIFeeEstimateRate() (feerates []sharedW.FeeEstimate, err error) {
	asset.fees.mu.RLock()
	feerates = asset.fees.APIFeeRates
	lastblock := asset.fees.LastBestblock
	asset.fees.mu.RUnlock()

	// If best block hasn't changed, return the cached estimates.
	if asset.GetBestBlockHeight() == lastblock && lastblock > 0 {
		return feerates, nil
	}

	feerates, err = asset.fetchAPIFeeRate()
	if err != nil {
		return nil, err
	}

	// Do not cache empty results.
	if len(feerates) == 0 {
		return nil, errors.New("API feerates not available")
	}

	sort.Slice(feerates, func(i, j int) bool {
		return feerates[i].ConfirmedBlocks < feerates[j].ConfirmedBlocks
	})

	if len(feerates) > 5 {
		// TODO: subject to confirmation! => persist top five fee rates only.
		feerates = feerates[:5]
	}

	asset.fees.mu.Lock()
	asset.fees.APIFeeRates = feerates
	asset.fees.LastBestblock = asset.GetBestBlockHeight()
	asset.fees.mu.Unlock()

	return feerates, nil
}

// SetUserFeeRate sets the fee rate in kvB units. Setting fee rate less than
// MinFeeRatePerkvB is not allowed.
func (asset *Asset) SetUserFeeRate(feeRatePerkvB sharedW.AssetAmount) error {
	asset.fees.mu.Lock()
	defer asset.fees.mu.Unlock()

	if feeRatePerkvB.ToInt() < int64(MinFeeRatePerkvB) {
		return fmt.Errorf("minimum rate is %d Sat/kvB", int64(MinFeeRatePerkvB))
	}

	asset.fees.SetFeeRatePerkvB = feeRatePerkvB
	return nil
}

// GetUserFeeRate returns the fee rate in kvB units. If not set it defaults to
// FallBackFeeRatePerkvB.
func (asset *Asset) GetUserFeeRate() sharedW.AssetAmount {
	asset.fees.mu.RLock()
	defer asset.fees.mu.RUnlock()

	if asset.fees.SetFeeRatePerkvB == nil {
		// If not set, defaults to the fall back fee of 1000 sats/kvB = (1 Sat/vB)
		return Amount(FallBackFeeRatePerkvB)
	}
	return asset.fees.SetFeeRatePerkvB
}
