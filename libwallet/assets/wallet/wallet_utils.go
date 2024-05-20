package wallet

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"decred.org/dcrwallet/v3/errors"
	"decred.org/dcrwallet/v3/walletseed"
	"github.com/asdine/storm"
	btchdkeychain "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	dcrhdkeychain "github.com/decred/dcrd/hdkeychain/v3"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/secretbox"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/scrypt"
)

const (
	// Users cannot set a wallet with this prefix.
	reservedWalletPrefix = "wallet-"

	defaultDCRRequiredConfirmations = 2

	//  - 6 confirmation is the standard for most transactions to be considered
	// secure, enough for large payments between $10,000 - $1,000,000.
	defaultBTCRequiredConfirmations = 6

	defaultLTCRequiredConfirmations = 6

	// UnminedTxHeight defines the block height of the txs in the mempool
	UnminedTxHeight int32 = -1

	// btcLogFilename defines the btc log file name
	btcLogFilename = "btc.log"

	// dcrLogFilename defines the dcr log file name
	dcrLogFilename = "dcr.log"

	// ltcLogFilename defines the ltc log file name
	ltcLogFilename = "ltc.log"
)

// InvalidBlock defines invalid height and timestamp returned in case of an error.
var InvalidBlock = &BlockInfo{
	Height:    -1, // No block has this height.
	Timestamp: -1, // Evaluates to 1969-12-31 11:59:59 +0000
}

// RequiredConfirmations specifies the minimum number of confirmations
// a transaction needs to be consider as confirmed.
func (wallet *Wallet) RequiredConfirmations() int32 {
	var spendUnconfirmed bool
	wallet.ReadUserConfigValue(SpendUnconfirmedConfigKey, &spendUnconfirmed)
	if spendUnconfirmed {
		return 0
	}

	switch wallet.Type {
	case utils.BTCWalletAsset:
		return defaultBTCRequiredConfirmations
	case utils.DCRWalletAsset:
		return defaultDCRRequiredConfirmations
	case utils.LTCWalletAsset:
		return defaultLTCRequiredConfirmations
	}
	return -1 // Not supposed to happen
}

func (wallet *Wallet) ShutdownContextWithCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	wallet.cancelFuncs = append(wallet.cancelFuncs, cancel)
	return ctx, cancel
}

func (wallet *Wallet) MarkWalletAsDiscoveredAccounts() error {
	if wallet == nil {
		return errors.New(utils.ErrNotExist)
	}

	log.Infof("Set discovered accounts = true for wallet %d", wallet.ID)
	wallet.HasDiscoveredAccounts = true
	err := wallet.db.Save(wallet)
	if err != nil {
		return err
	}

	return nil
}

func (wallet *Wallet) batchDbTransaction(dbOp func(node storm.Node) error) (err error) {
	dbTx, err := wallet.db.Begin(true)
	if err != nil {
		return err
	}

	// Commit or rollback the transaction after f returns or panics.  Do not
	// recover from the panic to keep the original stack trace intact.
	panicked := true
	defer func() {
		if panicked || err != nil {
			dbTx.Rollback()
			return
		}

		err = dbTx.Commit()
	}()

	err = dbOp(dbTx)
	panicked = false
	return err
}

// DecryptSeed decrypts wallet.EncryptedMnemonic using privatePassphrase
func (wallet *Wallet) DecryptSeed(privatePassphrase string) (string, error) {
	if wallet.EncryptedMnemonic == nil {
		return "", errors.New(utils.ErrNoSeed)
	}

	return decryptWalletMnemonic([]byte(privatePassphrase), wallet.EncryptedMnemonic)
}

// VerifySeedForWallet compares seedMnemonic with the decrypted
// wallet.EncryptedMnemonic.
func (wallet *Wallet) VerifySeedForWallet(seedMnemonic, privpass string) (bool, error) {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()

	decryptedMnemonic, err := decryptWalletMnemonic([]byte(privpass), wallet.EncryptedMnemonic)
	if err != nil {
		return false, err
	}

	if decryptedMnemonic == seedMnemonic {
		if wallet.IsBackedUp {
			return true, nil // return early
		}

		wallet.IsBackedUp = true
		return true, utils.TranslateError(wallet.db.Save(wallet))
	}

	return false, errors.New(utils.ErrInvalid)
}

// naclLoadFromPass derives a nacl.Key from pass using scrypt.Key.
func naclLoadFromPass(pass []byte) (nacl.Key, error) {
	const N, r, p = 1 << 15, 8, 1

	hash, err := scrypt.Key(pass, nil, N, r, p, 32)
	if err != nil {
		return nil, err
	}
	return nacl.Load(utils.EncodeHex(hash))
}

// encryptWalletMnemonic encrypts the mnemonic with secretbox.EasySeal using pass.
func encryptWalletMnemonic(pass []byte, mnemonic string) ([]byte, error) {
	key, err := naclLoadFromPass(pass)
	if err != nil {
		return nil, err
	}
	return secretbox.EasySeal([]byte(mnemonic), key), nil
}

// decryptWalletMnemonic decrypts the encryptedMnemonic with secretbox.EasyOpen using pass.
func decryptWalletMnemonic(pass []byte, encryptedMnemonic []byte) (string, error) {
	key, err := naclLoadFromPass(pass)
	if err != nil {
		return "", err
	}

	decryptedMnemonic, err := secretbox.EasyOpen(encryptedMnemonic, key)
	if err != nil {
		return "", errors.New(utils.ErrInvalidPassphrase)
	}

	return string(decryptedMnemonic), nil
}

// For use with gomobile bind,
// doesn't support the alternative `GenerateSeed` function because it returns more than 2 types.
func generateMnemonic(wordSeedType WordSeedType) (v string, err error) {
	var entropy []byte
	//33-word seeds and 24-word seeds both use length 32 (256 bits) while 12-word seed uses length 16 (128 bits).
	var length uint8 = dcrhdkeychain.RecommendedSeedLen
	if wordSeedType == WordSeed12 {
		length = dcrhdkeychain.MinSeedBytes
	}

	entropy, err = btchdkeychain.GenerateSeed(length)
	if err != nil {
		return "", err
	}

	if len(entropy) > 0 {
		if wordSeedType == WordSeed33 {
			// Generate 33-word seeds from PGWord list
			return walletseed.EncodeMnemonic(entropy), nil
		}
		// Create mnemonic from entropy
		// Use bip39 to generate 12-word seeds and 24-word seeds
		mnemonic, err := bip39.NewMnemonic(entropy)
		if err != nil {
			return "", err
		}
		return mnemonic, nil
	}

	return "", fmt.Errorf("entropy is empty")
}

func VerifyMnemonic(seedMnemonic string, assetType utils.AssetType, seedType WordSeedType) bool {
	_, err := DecodeSeedMnemonic(seedMnemonic, assetType, seedType)
	return err == nil
}

func DecodeSeedMnemonic(seedMnemonic string, assetType utils.AssetType, seedType WordSeedType) (hashedSeed []byte, err error) {
	switch assetType {
	case utils.BTCWalletAsset, utils.DCRWalletAsset, utils.LTCWalletAsset:
		words := strings.Split(strings.TrimSpace(seedMnemonic), " ")
		var entropy []byte
		// seedMnemonic is hex string
		if len(words) == 1 {
			entropy, err = hex.DecodeString(words[0])
			if seedType == WordSeed33 {
				return entropy, err
			}
			seedMnemonic, err = bip39.NewMnemonic(entropy)
			if err != nil {
				return nil, err
			}
		}

		// seedMnemonic is list of words
		if seedType == WordSeed33 {
			hashedSeed, err = walletseed.DecodeUserInput(seedMnemonic)
		} else {
			hashedSeed, err = bip39.NewSeedWithErrorChecking(seedMnemonic, "")
		}
	default:
		err = fmt.Errorf("%v: (%v)", utils.ErrAssetUnknown, assetType)
	}
	return
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

// Balances returns the spendable balance and total balance of the wallet.
func Balances(w Asset) (AssetAmount, AssetAmount, error) {
	accountsResult, err := w.GetAccountsRaw()
	if err != nil {
		return w.ToAmount(0), w.ToAmount(0), err
	}

	var totalSpendable, totalBalance int64
	for _, account := range accountsResult.Accounts {
		totalSpendable += account.Balance.Spendable.ToInt()
		totalBalance += account.Balance.Total.ToInt()
	}

	return w.ToAmount(totalSpendable), w.ToAmount(totalBalance), nil
}

// SortTxs is a shared function that sorts the provided txs slice in ascending
// or descending order depending on newestFirst.
func SortTxs(txs []*Transaction, newestFirst bool) {
	sort.SliceStable(txs, func(i, j int) bool {
		if newestFirst {
			return txs[i].Timestamp > txs[j].Timestamp
		}
		return txs[i].Timestamp < txs[j].Timestamp
	})
}

// ParseWalletPeers is a convenience function that converts the provided
// peerAddresses string to an array of valid peer addresses.
func ParseWalletPeers(peerAddresses string, port string) ([]string, []error) {
	var persistentPeers []string
	var errs []error
	if peerAddresses != "" {
		addresses := strings.Split(peerAddresses, ";")
		for _, address := range addresses {
			host, p, err := net.SplitHostPort(address)
			// If err assume because port was not supplied.
			if err != nil {
				host = address
				p = port
			}
			peerAddress, err := utils.NormalizeAddress(host, p)
			if err != nil {
				errs = append(errs, fmt.Errorf("SPV peer address(%s) is invalid: %v", peerAddress, err))
			} else {
				persistentPeers = append(persistentPeers, peerAddress)
			}
		}
	}

	return persistentPeers, errs
}
