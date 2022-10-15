package wallet

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/walletseed"
	"github.com/asdine/storm"
	btchdkeychain "github.com/btcsuite/btcd/btcutil/hdkeychain"
	dcrhdkeychain "github.com/decred/dcrd/hdkeychain/v3"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/secretbox"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"golang.org/x/crypto/scrypt"
)

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
func (wallet *Wallet) DecryptSeed(privatePassphrase []byte) (string, error) {
	if wallet.EncryptedSeed == nil {
		return "", errors.New(utils.ErrInvalid)
	}

	return decryptWalletSeed(privatePassphrase, wallet.EncryptedSeed)
}

// VerifySeedForWallet compares seedMnemonic with the decrypted wallet.EncryptedSeed and clears wallet.EncryptedSeed if they match.
func (wallet *Wallet) VerifySeedForWallet(seedMnemonic string, privpass []byte) (bool, error) {
	decryptedSeed, err := decryptWalletSeed(privpass, wallet.EncryptedSeed)
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
	return nacl.Load(EncodeHex(hash))
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
	}

	if len(seed) > 0 {
		return walletseed.EncodeMnemonic(seed), nil
	}

	// Execution should never get here but error added as a safeguard to
	// ensure any new asset added must add its own custom way to generate wallet
	// seed added above, if need be.
	return "", fmt.Errorf("%v: (%v)", utils.ErrAssetUnknown, assetType)
}

// func verifySeed(seedMnemonic string) bool {
// 	_, err := walletseed.DecodeUserInput(seedMnemonic)
// 	return err == nil
// }

// func (wallet *Wallet) loadWalletTemporarily(ctx context.Context, walletDataDir, walletPublicPass string,
// 	onLoaded func(*w.Wallet) error) error {

// 	if walletPublicPass == "" {
// 		walletPublicPass = w.InsecurePubPassphrase
// 	}

// 	// initialize the wallet loader
// 	// open the wallet to get ready for temporary use
// 	wal, err := wallet.loader.OpenExistingWallet(ctx, strconv.Itoa(wallet.ID), []byte(walletPublicPass))
// 	if err != nil {
// 		return utils.TranslateError(err)
// 	}

// 	// unload wallet after temporary use
// 	defer wallet.loader.UnloadWallet()

// 	if onLoaded != nil {
// 		return onLoaded(wal.DCR)
// 	}

// 	return nil
// }
