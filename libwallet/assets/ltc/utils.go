package ltc

import (
	"encoding/binary"
	"time"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/dcrlabs/ltcwallet/waddrmgr"
	"github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/ltcsuite/ltcd/ltcutil/hdkeychain"
)

const (
	maxAmountLitoshi = ltcutil.MaxSatoshi // MaxSatoshi is the maximum transaction amount allowed in litoshi.

	// TestnetHDPath is the BIP 84 HD path used for deriving addresses on the
	// test network.
	TestnetHDPath = "m / 84' / 1' / "
	// MainnetHDPath is the BIP 84 HD path used for deriving addresses on the
	// main network.
	MainnetHDPath = "m / 84' / 0' / "

	// GenesisTimestampMainnet represents the genesis timestamp for the BTC mainnet.
	GenesisTimestampMainnet = 1317972665
	// GenesisTimestampTestnet represents the genesis timestamp for the BTC testnet.
	GenesisTimestampTestnet = 1486949366
	// TargetTimePerBlockMainnet represents the target time per block in seconds for LTC mainnet.
	TargetTimePerBlockMainnet = 150
	// TargetTimePerBlockTestnet represents the target time per block in seconds for LTC testnet.
	TargetTimePerBlockTestnet = 150
)

var wAddrMgrBkt = []byte("waddrmgr")

// GetScope returns the key scope that will be used within the waddrmgr to
// create an HD chain for deriving all of our required keys. A different
// scope is used for each specific coin type.
func GetScope() waddrmgr.KeyScope {
	// Construct the key scope that will be used within the waddrmgr to
	// create an HD chain for deriving all of our required keys. A different
	// scope is used for each specific coin type.
	return waddrmgr.KeyScopeBIP0084WithBitcoinCoinID
}

// AmountLTC converts a litoshi amount to a LTC amount.
func AmountLTC(amount int64) float64 {
	return ltcutil.Amount(amount).ToBTC()
}

// AmountLitoshi converts a LTC amount to a litoshi amount.
func AmountLitoshi(f float64) int64 {
	amount, err := ltcutil.NewAmount(f)
	if err != nil {
		log.Error(err)
		return -1
	}
	return int64(amount)
}

// ToAmount returns a LTC amount that implements the asset amount interface.
func (asset *Asset) ToAmount(v int64) sharedW.AssetAmount {
	return Amount(ltcutil.Amount(v))
}

// DeriveAccountXpub derives the xpub for the given account.
func (asset *Asset) DeriveAccountXpub(seedMnemonic string, wordSeedType sharedW.WordSeedType, account uint32, params *chaincfg.Params) (xpub string, err error) {
	seed, err := sharedW.DecodeSeedMnemonic(seedMnemonic, asset.Type, wordSeedType)
	if err != nil {
		return "", err
	}
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()

	// Derive the master extended key from the provided seed.
	masterNode, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return "", err
	}
	defer masterNode.Zero()

	path := []uint32{hardenedKey(GetScope().Purpose), hardenedKey(GetScope().Coin)}
	path = append(path, hardenedKey(account))

	currentKey := masterNode
	for _, pathPart := range path {
		currentKey, err = currentKey.Derive(pathPart)
		if err != nil {
			return "", err
		}
	}

	pubVersionBytes := make([]byte, len(params.HDPublicKeyID))
	copy(pubVersionBytes, params.HDPublicKeyID[:])

	switch params.Name {
	case chaincfg.TestNet4Params.Name:
		binary.BigEndian.PutUint32(pubVersionBytes, uint32(
			waddrmgr.HDVersionTestNetBIP0084,
		))

	case chaincfg.MainNetParams.Name:
		binary.BigEndian.PutUint32(pubVersionBytes, uint32(
			waddrmgr.HDVersionMainNetBIP0084,
		))
	case chaincfg.SimNetParams.Name:
		binary.BigEndian.PutUint32(pubVersionBytes, uint32(
			waddrmgr.HDVersionSimNetBIP0044,
		))
	default:
		return "", utils.ErrInvalidNet
	}

	currentKey, err = currentKey.CloneWithVersion(
		params.HDPrivateKeyID[:],
	)
	if err != nil {
		return "", err
	}
	currentKey, err = currentKey.Neuter()
	if err != nil {
		return "", err
	}
	currentKey, err = currentKey.CloneWithVersion(pubVersionBytes)
	if err != nil {
		return "", err
	}

	return currentKey.String(), nil
}

func hardenedKey(key uint32) uint32 {
	return key + hdkeychain.HardenedKeyStart
}

func secondsToDuration(secs float64) time.Duration {
	return time.Duration(secs) * time.Second
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
