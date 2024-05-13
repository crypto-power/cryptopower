package bch

import (
	"encoding/binary"

	"decred.org/dcrwallet/v3/walletseed"
	btchdkeychain "github.com/btcsuite/btcd/btcutil/hdkeychain"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
	"github.com/tyler-smith/go-bip39"

	"github.com/gcash/bchutil/hdkeychain"
	btcchaincfg "github.com/btcsuite/btcd/chaincfg"
	btcwaddrmgr "github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/dcrlabs/bchwallet/waddrmgr"
)

const (
	maxAmountSatoshi = bchutil.MaxSatoshi // MaxSatoshi is the maximum transaction amount allowed in satoshi.

	// TestnetHDPath is the BIP 84 HD path used for deriving addresses on the
	// test network.
	TestnetHDPath = "m / 84' / 1' / "
	// MainnetHDPath is the BIP 84 HD path used for deriving addresses on the
	// main network.
	MainnetHDPath = "m / 84' / 0' / "
)

var wAddrMgrBkt = []byte("waddrmgr")

// GetScope returns the key scope that will be used within the waddrmgr to
// create an HD chain for deriving all of our required keys. A different
// scope is used for each specific coin type.
func GetScope() waddrmgr.KeyScope {
	// Construct the key scope that will be used within the waddrmgr to
	// create an HD chain for deriving all of our required keys. A different
	// scope is used for each specific coin type.
	return waddrmgr.KeyScopeBIP0044
}

// AmountBCH converts a satoshi amount to a BCH amount.
func AmountBCH(amount int64) float64 {
	return bchutil.Amount(amount).ToBCH()
}

// AmountSatoshi converts a BCH amount to a satoshi amount.
func AmountSatoshi(f float64) int64 {
	amount, err := bchutil.NewAmount(f)
	if err != nil {
		log.Error(err)
		return -1
	}
	return int64(amount)
}

// ToAmount returns a BCH amount that implements the asset amount interface.
func (asset *Asset) ToAmount(v int64) sharedW.AssetAmount {
	return Amount(bchutil.Amount(v))
}

// DeriveAccountXpub derives the xpub for the given account.
func (asset *Asset) DeriveAccountXpub(seedMnemonic string, wordSeedType sharedW.WordSeedType, account uint32, params *btcchaincfg.Params) (xpub string, err error) {
	var seed []byte
	if wordSeedType == sharedW.WordSeed33 {
		seed, err = walletseed.DecodeUserInput(seedMnemonic)
	} else {
		seed, err = bip39.EntropyFromMnemonic(seedMnemonic)
	}
	if err != nil {
		return "", err
	}
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()

	// Derive the master extended key from the provided seed.
	masterNode, err := btchdkeychain.NewMaster(seed, params)
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
			btcwaddrmgr.HDVersionTestNetBIP0084,
		))

	case chaincfg.MainNetParams.Name:
		binary.BigEndian.PutUint32(pubVersionBytes, uint32(
			btcwaddrmgr.HDVersionMainNetBIP0084,
		))
	case chaincfg.SimNetParams.Name:
		binary.BigEndian.PutUint32(pubVersionBytes, uint32(
			btcwaddrmgr.HDVersionSimNetBIP0044,
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
