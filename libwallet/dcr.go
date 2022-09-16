package libwallet

import (
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"

	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/v3"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"
	bolt "go.etcd.io/bbolt"
)

// initializeDCRWalletParameters initializes the fields each DCR wallet is going to need to be setup
// such as chainparams, root directory, network and database references 
func initializeDCRWallet(rootDir, dbDriver, netType string) (*storm.DB, *chaincfg.Params, string, error) {
	var db *storm.DB

	rootDir = filepath.Join(rootDir, netType, "dcr")
	err := os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		return nil, nil, "", errors.Errorf("failed to create dcr rootDir: %v", err)
	}

	chainParams, err := utils.ChainParams(netType)
	if err != nil {
		return db, chainParams, "", err
	}

	db, err = storm.Open(filepath.Join(rootDir, walletsDbName))
	if err != nil {
		log.Errorf("Error opening dcr wallets database: %s", err.Error())
		if err == bolt.ErrTimeout {
			// timeout error occurs if storm fails to acquire a lock on the database file
			return db, chainParams, "", errors.E(ErrWalletDatabaseInUse)
		}
		return db, chainParams, "", errors.Errorf("error opening dcr wallets database: %s", err.Error())
	}

	// init database for saving/reading wallet objects
	err = db.Init(&dcr.Wallet{})
	if err != nil {
		log.Errorf("Error initializing wallets database: %s", err.Error())
		return nil, chainParams, "", err
	}

	// init database for saving/reading proposal objects
	err = db.Init(&dcr.Proposal{})
	if err != nil {
		log.Errorf("Error initializing wallets database: %s", err.Error())
		return db, chainParams, "", err
	}

	return db, chainParams, rootDir, nil
}
