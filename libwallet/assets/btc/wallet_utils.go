package btc

import (
	"os"
	"strconv"
	"strings"

	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

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

func WalletNameExists(walletName string, walledDbRef *storm.DB) (bool, error) {
	if strings.HasPrefix(walletName, "wallet-") {
		return false, errors.E(ErrReservedWalletName)
	}

	err := walledDbRef.One("Name", walletName, &Wallet{})
	if err == nil {
		return true, nil
	} else if err != storm.ErrNotFound {
		return false, err
	}

	return false, nil
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

func moveFile(sourcePath, destinationPath string) error {
	if exists, _ := fileExists(sourcePath); exists {
		return os.Rename(sourcePath, destinationPath)
	}
	return nil
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
