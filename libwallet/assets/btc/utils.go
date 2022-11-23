package btc

import (
	"encoding/binary"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/walletseed"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/waddrmgr"
)

const (
	maxAmountSatoshi = btcutil.MaxSatoshi // MaxSatoshi is the maximum transaction amount allowed in satoshi.

	TestnetHDPath = "m / 84' / 1' / "
	MainnetHDPath = "m / 84' / 0' / "
)

var wAddrMgrBkt = []byte("waddrmgr")

func (asset *BTCAsset) GetScope() waddrmgr.KeyScope {
	// Construct the key scope that will be used within the waddrmgr to
	// create an HD chain for deriving all of our required keys. A different
	// scope is used for each specific coin type.
	return waddrmgr.KeyScopeBIP0084
}

func AmountBTC(amount int64) float64 {
	return btcutil.Amount(amount).ToBTC()
}

func AmountSatoshi(f float64) int64 {
	amount, err := btcutil.NewAmount(f)
	if err != nil {
		log.Error(err)
		return -1
	}
	return int64(amount)
}

// Returns a BTC amount that implements the asset amount interface.
func (asset *BTCAsset) ToAmount(v int64) sharedW.AssetAmount {
	return BTCAmount(btcutil.Amount(v))
}

func hardenedKey(key uint32) uint32 {
	return key + hdkeychain.HardenedKeyStart
}

func (asset *BTCAsset) DeriveAccountXpub(seedMnemonic string, account uint32, params *chaincfg.Params) (xpub string, err error) {
	seed, err := walletseed.DecodeUserInput(seedMnemonic)
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

	path := []uint32{hardenedKey(asset.GetScope().Purpose), hardenedKey(asset.GetScope().Coin)}
	path = append(path, hardenedKey(account))

	var currentKey = masterNode
	for _, pathPart := range path {
		currentKey, err = currentKey.Derive(pathPart)
		if err != nil {
			return "", err
		}
	}

	pubVersionBytes := make([]byte, len(params.HDPublicKeyID))
	copy(pubVersionBytes, params.HDPublicKeyID[:])

	switch params.Name {
	case chaincfg.TestNet3Params.Name:
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
