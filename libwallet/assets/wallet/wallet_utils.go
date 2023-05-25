package wallet

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/walletseed"
	"github.com/asdine/storm"
	btchdkeychain "github.com/btcsuite/btcd/btcutil/hdkeychain"
	dcrhdkeychain "github.com/decred/dcrd/hdkeychain/v3"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/secretbox"
	ltchdkeychain "github.com/ltcsuite/ltcd/ltcutil/hdkeychain"
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

// DecryptSeed decrypts wallet.EncryptedSeed using privatePassphrase
func (wallet *Wallet) DecryptSeed(privatePassphrase string) (string, error) {
	if wallet.EncryptedSeed == nil {
		return "", errors.New(utils.ErrInvalid)
	}

	return decryptWalletSeed([]byte(privatePassphrase), wallet.EncryptedSeed)
}

// VerifySeedForWallet compares seedMnemonic with the decrypted wallet.EncryptedSeed and clears wallet.EncryptedSeed if they match.
func (wallet *Wallet) VerifySeedForWallet(seedMnemonic, privpass string) (bool, error) {
	decryptedSeed, err := decryptWalletSeed([]byte(privpass), wallet.EncryptedSeed)
	if err != nil {
		return false, err
	}

	if decryptedSeed == seedMnemonic {
		wallet.EncryptedSeed = nil
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

// encryptWalletSeed encrypts the seed with secretbox.EasySeal using pass.
func encryptWalletSeed(pass []byte, seed string) ([]byte, error) {
	key, err := naclLoadFromPass(pass)
	if err != nil {
		return nil, err
	}
	return secretbox.EasySeal([]byte(seed), key), nil
}

// decryptWalletSeed decrypts the encryptedSeed with secretbox.EasyOpen using pass.
func decryptWalletSeed(pass []byte, encryptedSeed []byte) (string, error) {
	key, err := naclLoadFromPass(pass)
	if err != nil {
		return "", err
	}

	decryptedSeed, err := secretbox.EasyOpen(encryptedSeed, key)
	if err != nil {
		return "", errors.New(utils.ErrInvalidPassphrase)
	}

	return string(decryptedSeed), nil
}

// For use with gomobile bind,
// doesn't support the alternative `GenerateSeed` function because it returns more than 2 types.
func generateSeed(assetType utils.AssetType) (v string, err error) {
	var seed []byte
	switch assetType {
	case utils.BTCWalletAsset:
		seed, err = btchdkeychain.GenerateSeed(btchdkeychain.RecommendedSeedLen)
		if err != nil {
			return "", err
		}
	case utils.DCRWalletAsset:
		seed, err = dcrhdkeychain.GenerateSeed(dcrhdkeychain.RecommendedSeedLen)
		if err != nil {
			return "", err
		}
	case utils.LTCWalletAsset:
		seed, err = ltchdkeychain.GenerateSeed(ltchdkeychain.RecommendedSeedLen)
		if err != nil {
			return "", err
		}
	}

	if len(seed) > 0 {
		return walletseed.EncodeMnemonic(seed), nil
	}

	// Execution should never get here but error added as a safeguard to
	// ensure any new asset added must add its own custom way to generate wallet
	// seed added above, if need be.
	return "", fmt.Errorf("%v: (%v)", utils.ErrAssetUnknown, assetType)
}

func VerifySeed(seedMnemonic string, assetType utils.AssetType) bool {
	_, err := DecodeSeedMnemonic(seedMnemonic, assetType)
	return err == nil
}

func DecodeSeedMnemonic(seedMnemonic string, assetType utils.AssetType) (hashedSeed []byte, err error) {
	switch assetType {
	case utils.BTCWalletAsset, utils.DCRWalletAsset, utils.LTCWalletAsset:
		hashedSeed, err = walletseed.DecodeUserInput(seedMnemonic)
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
