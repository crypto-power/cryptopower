package dcr

import (
	"context"

	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/secretbox"
	"golang.org/x/crypto/scrypt"

	w "decred.org/dcrwallet/v2/wallet"

	"strings"
)

const (
	walletsDbName = "wallets.db"
)

func (wallet *Wallet) markWalletAsDiscoveredAccounts() error {
	if wallet == nil {
		return errors.New(ErrNotExist)
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

func (wallet *Wallet) WalletNameExists(walletName string) (bool, error) {
	if strings.HasPrefix(walletName, "wallet-") {
		return false, errors.E(ErrReservedWalletName)
	}

	err := wallet.db.One("Name", walletName, &Wallet{})
	if err == nil {
		return true, nil
	} else if err != storm.ErrNotFound {
		return false, err
	}

	return false, nil
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
		return "", errors.New(ErrInvalidPassphrase)
	}

	return string(decryptedSeed), nil
}

func (wallet *Wallet) loadWalletTemporarily(ctx context.Context, walletDataDir, walletPublicPass string,
	onLoaded func(*w.Wallet) error) error {

	if walletPublicPass == "" {
		walletPublicPass = w.InsecurePubPassphrase
	}

	// initialize the wallet loader
	walletLoader := initWalletLoader(wallet.chainParams, walletDataDir, wallet.dbDriver)

	// open the wallet to get ready for temporary use
	wal, err := walletLoader.OpenExistingWallet(ctx, []byte(walletPublicPass))
	if err != nil {
		return translateError(err)
	}

	// unload wallet after temporary use
	defer walletLoader.UnloadWallet()

	if onLoaded != nil {
		return onLoaded(wal)
	}

	return nil
}
