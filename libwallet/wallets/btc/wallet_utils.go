package btc

import (
	"os"
	"strconv"
	"strings"

	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
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
