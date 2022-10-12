package libwallet

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/txrules"
	"decred.org/dcrwallet/v2/walletseed"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/hdkeychain/v3"
	"github.com/decred/dcrd/wire"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	dcrLoader "gitlab.com/raedah/cryptopower/libwallet/internal/loader/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"
	mainW "gitlab.com/raedah/cryptopower/libwallet/wallets/wallet"
)

const (
	walletDbName = "wallet.db"

	// FetchPercentage is used to increase the initial estimate gotten during cfilters stage
	FetchPercentage = 0.38

	// Use 10% of estimated total headers fetch time to estimate rescan time
	RescanPercentage = 0.1

	// Use 80% of estimated total headers fetch time to estimate address discovery time
	DiscoveryPercentage = 0.8

	MaxAmountAtom = dcrutil.MaxAmount
	MaxAmountDcr  = dcrutil.MaxAmount / dcrutil.AtomsPerCoin

	TestnetHDPath       = "m / 44' / 1' / "
	LegacyTestnetHDPath = "m / 44’ / 11’ / "
	MainnetHDPath       = "m / 44' / 42' / "
	LegacyMainnetHDPath = "m / 44’ / 20’ / "

	DefaultRequiredConfirmations = 2

	LongAbbreviationFormat     = "long"
	ShortAbbreviationFormat    = "short"
	ShortestAbbreviationFormat = "shortest"
)

func (mw *MultiWallet) RequiredConfirmations() int32 {
	spendUnconfirmed := mw.ReadBoolConfigValueForKey(SpendUnconfirmedConfigKey, false)
	if spendUnconfirmed {
		return 0
	}
	return DefaultRequiredConfirmations
}

func (mw *MultiWallet) listenForShutdown() {

	mw.cancelFuncs = make([]context.CancelFunc, 0)
	mw.shuttingDown = make(chan bool)
	go func() {
		<-mw.shuttingDown
		for _, cancel := range mw.cancelFuncs {
			cancel()
		}
	}()
}

func (mw *MultiWallet) contextWithShutdownCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	mw.cancelFuncs = append(mw.cancelFuncs, cancel)
	return ctx, cancel
}

func (mw *MultiWallet) ValidateExtPubKey(extendedPubKey string) error {
	_, err := hdkeychain.NewKeyFromString(extendedPubKey, mw.chainParams)
	if err != nil {
		if err == hdkeychain.ErrInvalidChild {
			return errors.New(ErrUnusableSeed)
		}

		return errors.New(ErrInvalid)
	}

	return nil
}

func NormalizeAddress(addr string, defaultPort string) (string, error) {
	// If the first SplitHostPort errors because of a missing port and not
	// for an invalid host, add the port.  If the second SplitHostPort
	// fails, then a port is not missing and the original error should be
	// returned.
	host, port, origErr := net.SplitHostPort(addr)
	if origErr == nil {
		return net.JoinHostPort(host, port), nil
	}
	addr = net.JoinHostPort(addr, defaultPort)
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", origErr
	}
	return addr, nil
}

// For use with gomobile bind,
// doesn't support the alternative `GenerateSeed` function because it returns more than 2 types.
func GenerateSeed() (string, error) {
	seed, err := hdkeychain.GenerateSeed(hdkeychain.RecommendedSeedLen)
	if err != nil {
		return "", err
	}

	return walletseed.EncodeMnemonic(seed), nil
}

func VerifySeed(seedMnemonic string) bool {
	_, err := walletseed.DecodeUserInput(seedMnemonic)
	return err == nil
}

// ExtractDateOrTime returns the date represented by the timestamp as a date string if the timestamp is over 24 hours ago.
// Otherwise, the time alone is returned as a string.
func ExtractDateOrTime(timestamp int64) string {
	utcTime := time.Unix(timestamp, 0).UTC()
	if time.Now().UTC().Sub(utcTime).Hours() > 24 {
		return utcTime.Format("2006-01-02")
	} else {
		return utcTime.Format("15:04:05")
	}
}

func FormatUTCTime(timestamp int64) string {
	return time.Unix(timestamp, 0).UTC().Format("2006-01-02 15:04:05")
}

func AmountCoin(amount int64) float64 {
	return dcrutil.Amount(amount).ToCoin()
}

func AmountAtom(f float64) int64 {
	amount, err := dcrutil.NewAmount(f)
	if err != nil {
		log.Error(err)
		return -1
	}
	return int64(amount)
}

func EncodeHex(hexBytes []byte) string {
	return hex.EncodeToString(hexBytes)
}

func EncodeBase64(text []byte) string {
	return base64.StdEncoding.EncodeToString(text)
}

func DecodeBase64(base64Text string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(base64Text)
}

func ShannonEntropy(text string) (entropy float64) {
	if text == "" {
		return 0
	}
	for i := 0; i < 256; i++ {
		px := float64(strings.Count(text, string(byte(i)))) / float64(len(text))
		if px > 0 {
			entropy += -px * math.Log2(px)
		}
	}
	return entropy
}

func TransactionDirectionName(direction int32) string {
	switch direction {
	case dcr.TxDirectionSent:
		return "Sent"
	case dcr.TxDirectionReceived:
		return "Received"
	case dcr.TxDirectionTransferred:
		return "Yourself"
	default:
		return "invalid"
	}
}

func CalculateTotalTimeRemaining(timeRemainingInSeconds int64) string {
	minutes := timeRemainingInSeconds / 60
	if minutes > 0 {
		return fmt.Sprintf("%d min", minutes)
	}
	return fmt.Sprintf("%d sec", timeRemainingInSeconds)
}

func CalculateDaysBehind(lastHeaderTime int64) string {
	diff := time.Since(time.Unix(lastHeaderTime, 0))
	daysBehind := int(math.Round(diff.Hours() / 24))
	if daysBehind == 0 {
		return "<1 day"
	} else if daysBehind == 1 {
		return "1 day"
	} else {
		return fmt.Sprintf("%d days", daysBehind)
	}
}

func roundUp(n float64) int32 {
	return int32(math.Round(n))
}

func WalletUniqueConfigKey(walletID int, key string) string {
	return fmt.Sprintf("%d%s", walletID, key)
}

func WalletExistsAt(directory string) bool {
	walletDbFilePath := filepath.Join(directory, walletDbName)
	exists, err := fileExists(walletDbFilePath)
	if err != nil {
		log.Errorf("wallet exists check error: %v", err)
	}
	return exists
}

func fileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func moveFile(sourcePath, destinationPath string) error {
	if exists, _ := fileExists(sourcePath); exists {
		return os.Rename(sourcePath, destinationPath)
	}
	return nil
}

// done returns whether the context's Done channel was closed due to
// cancellation or exceeded deadline.
func done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func backupFile(fileName string, suffix int) (newName string, err error) {
	newName = fileName + ".bak" + strconv.Itoa(suffix)
	exists, err := fileExists(newName)
	if err != nil {
		return "", err
	} else if exists {
		return backupFile(fileName, suffix+1)
	}

	err = moveFile(fileName, newName)
	if err != nil {
		return "", err
	}

	return newName, nil
}

func initWalletLoader(chainParams *chaincfg.Params, walletDataDir, walletDbDriver string) loader.AssetLoader {
	// TODO: Allow users provide values to override these defaults.
	cfg := &mainW.WalletConfig{
		GapLimit:                20,
		AllowHighFees:           false,
		RelayFee:                txrules.DefaultRelayFeePerKb,
		AccountGapLimit:         wallet.DefaultAccountGapLimit,
		DisableCoinTypeUpgrades: false,
		ManualTickets:           false,
		MixSplitLimit:           10,
	}

	stakeOptions := &dcrLoader.StakeOptions{
		VotingEnabled: false,
		AddressReuse:  false,
		VotingAddress: nil,
	}
	walletLoader := dcrLoader.NewLoader(chainParams, walletDataDir, stakeOptions,
		cfg.GapLimit, cfg.RelayFee, cfg.AllowHighFees, cfg.DisableCoinTypeUpgrades,
		cfg.ManualTickets, cfg.AccountGapLimit, cfg.MixSplitLimit)

	if walletDbDriver != "" {
		walletLoader.SetDatabaseDriver(walletDbDriver)
	}

	return walletLoader
}

// makePlural is used with the TimeElapsed function. makePlural checks if the arguments passed is > 1,
// if true, it adds "s" after the given time to make it plural
func makePlural(x float64) string {
	if int(x) == 1 {
		return ""
	}
	return "s"
}

// TimeElapsed returns the formatted time diffrence between two times as a string.
// If the argument `fullTime` is set to true, then the full time available is returned e.g 3 hours, 2 minutes, 20 seconds ago,
// as opposed to 3 hours ago.
// If the argument `abbreviationFormat` is set to `long` the time format is e.g 2 minutes
// If the argument `abbreviationFormat` is set to `short` the time format is e.g 2 mins
// If the argument `abbreviationFormat` is set to `shortest` the time format is e.g 2 m
func TimeElapsed(now, then time.Time, abbreviationFormat string, fullTime bool) string {
	var parts []string
	var text string

	year2, month2, day2 := now.Date()
	hour2, minute2, second2 := now.Clock()

	year1, month1, day1 := then.Date()
	hour1, minute1, second1 := then.Clock()

	year := math.Abs(float64(year2 - year1))
	month := math.Abs(float64(month2 - month1))
	day := math.Abs(float64(day2 - day1))
	hour := math.Abs(float64(hour2 - hour1))
	minute := math.Abs(float64(minute2 - minute1))
	second := math.Abs(float64(second2 - second1))

	week := math.Floor(day / 7)

	if year > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(year))+" year"+makePlural(year))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(year))+" yr"+makePlural(year))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(year))+" y")
		}
	}

	if month > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(month))+" month"+makePlural(month))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(month))+" mon"+makePlural(month))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(month))+" m")
		}
	}

	if week > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(week))+" week"+makePlural(week))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(week))+" wk"+makePlural(week))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(week))+" w")
		}
	}

	if day > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(day))+" day"+makePlural(day))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(day))+" dy"+makePlural(day))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(day))+" d")
		}
	}

	if hour > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(hour))+" hour"+makePlural(hour))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(hour))+" hr"+makePlural(hour))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(hour))+" h")
		}
	}

	if minute > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(minute))+" minute"+makePlural(minute))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(minute))+" min"+makePlural(minute))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(minute))+" mi")
		}
	}

	if second > 0 {
		if abbreviationFormat == LongAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(second))+" second"+makePlural(second))
		} else if abbreviationFormat == ShortAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(second))+" sec"+makePlural(second))
		} else if abbreviationFormat == ShortestAbbreviationFormat {
			parts = append(parts, strconv.Itoa(int(second))+" s")
		}
	}

	if now.After(then) {
		text = " ago"
	} else {
		text = " after"
	}

	if len(parts) == 0 {
		return "just now"
	}

	if fullTime {
		return strings.Join(parts, ", ") + text
	}
	return parts[0] + text
}

// voteVersion was borrowed from upstream, and needs to always be in
// sync with the upstream method. This is the LOC to the upstream version:
// https://github.com/decred/dcrwallet/blob/master/wallet/wallet.go#L266
func voteVersion(params *chaincfg.Params) uint32 {
	switch params.Net {
	case wire.MainNet:
		return 9
	case 0x48e7a065: // TestNet2
		return 6
	case wire.TestNet3:
		return 10
	case wire.SimNet:
		return 10
	default:
		return 1
	}
}

// HttpGet helps to convert json(Byte data) into a struct object.
func HttpGet(url string, respObj interface{}) (*http.Response, []byte, error) {
	rq := new(http.Client)
	resp, err := rq.Get((url))
	if err != nil {
		return nil, nil, err
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return resp, respBytes, fmt.Errorf("%d response from server: %v", resp.StatusCode, string(respBytes))
	}

	err = json.Unmarshal(respBytes, respObj)
	return resp, respBytes, err
}
