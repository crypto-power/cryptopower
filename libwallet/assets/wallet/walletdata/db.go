package walletdata

import (
	"fmt"
	"os"

	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/v3"
	bolt "go.etcd.io/bbolt"
)

const (
	DbName    = "walletData.db"
	OldDbName = "tx.db"

	TxBucketName = "TxIndexInfo"
	KeyDbVersion = "DbVersion"

	// TxDbVersion is necessary to force re-indexing if changes are made to the structure of data being stored.
	// Increment this version number if db structure changes such that client apps need to re-index.
	TxDbVersion uint32 = 3
)

type DB struct {
	walletDataDB *storm.DB
	chainParams  *chaincfg.Params
	Close        func() error
}

// Initialize opens the existing storm db at `dbPath`
// and checks the database version for compatibility.
// If there is a version mismatch or the db does not exist at `dbPath`,
// a new db is created and the current db version number saved to the db.
func Initialize(dbPath string, chainParams *chaincfg.Params, txData interface{}) (*DB, error) {
	walletDataDB, err := openOrCreateDB(dbPath)
	if err != nil {
		return nil, err
	}

	walletDataDB, err = ensureTxDatabaseVersion(walletDataDB, dbPath, txData)
	if err != nil {
		return nil, err
	}

	// init bucket for saving/reading transaction objects
	err = walletDataDB.Init(txData)
	if err != nil {
		return nil, fmt.Errorf("error initializing tx bucket for wallet: %s", err.Error())
	}

	return &DB{
		walletDataDB,
		chainParams,
		walletDataDB.Close,
	}, nil
}

func openOrCreateDB(dbPath string) (*storm.DB, error) {
	var isNewDbFile bool

	// first check if db file exists at dbPath, if not we'll need to create it and set the db version
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			isNewDbFile = true
		} else {
			return nil, fmt.Errorf("error checking tx index database file: %s", err.Error())
		}
	}

	walletDataDB, err := storm.Open(dbPath)
	if err != nil {
		switch err {
		case bolt.ErrTimeout:
			// timeout error occurs if storm fails to acquire a lock on the database file
			return nil, fmt.Errorf("wallet data database is in use by another process")
		default:
			return nil, fmt.Errorf("error opening wallet data database: %s", err.Error())
		}
	}

	if isNewDbFile {
		err = walletDataDB.Set(TxBucketName, KeyDbVersion, TxDbVersion)
		if err != nil {
			os.RemoveAll(dbPath)
			return nil, fmt.Errorf("error initializing wallet data db: %s", err.Error())
		}
	}

	return walletDataDB, nil
}

// ensureTxDatabaseVersion checks the version of the existing db against `TxDbVersion`.
// If there's a difference, the current wallet data db file is deleted and a new one created.
func ensureTxDatabaseVersion(walletDataDB *storm.DB, dbPath string, txData interface{}) (*storm.DB, error) {
	var currentDbVersion uint32
	err := walletDataDB.Get(TxBucketName, KeyDbVersion, &currentDbVersion)
	if err != nil && err != storm.ErrNotFound {
		// ignore key not found errors as earlier db versions did not set a version number in the db.
		return nil, fmt.Errorf("error checking wallet data database version: %s", err.Error())
	}

	if currentDbVersion != TxDbVersion {
		if err = walletDataDB.Drop(txData); err != nil {
			return nil, fmt.Errorf("error deleting outdated wallet data database: %s", err.Error())
		}

		if err = walletDataDB.Set(TxBucketName, KeyDbVersion, TxDbVersion); err != nil {
			return nil, fmt.Errorf("error updating tx db version: %s", err.Error())
		}

		return walletDataDB, walletDataDB.Set(TxBucketName, KeyEndBlock, 0) // reset tx index
	}

	return walletDataDB, nil
}
