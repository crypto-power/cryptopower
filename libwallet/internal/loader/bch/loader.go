package bch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/gcash/bchd/chaincfg"
	// "github.com/gcash/bchutil/hdkeychain"
	btcwallet "github.com/btcsuite/btcwallet/wallet"
	"github.com/dcrlabs/bchwallet/waddrmgr"
	"github.com/dcrlabs/bchwallet/wallet"
	_ "github.com/dcrlabs/bchwallet/walletdb/bdb" // bdb init() registers a driver

	"github.com/crypto-power/cryptopower/libwallet/internal/loader"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

var log = loader.Log

// bchLoader implements the creating of new and opening of existing bch wallets.
// This is primarily intended for use by the RPC servers, to enable
// methods and services which require the wallet when the wallet is loaded by
// another subsystem.
//
// bchLoader is safe for concurrent access.
type bchLoader struct {
	*loader.Loader

	chainParams *chaincfg.Params
	wallet      *wallet.Wallet

	recoveryWindow uint32
	dbTimeout      time.Duration
	keyscope       waddrmgr.KeyScope

	mu sync.RWMutex
}

// LoaderConf models the configuration options of a bch loader.
type LoaderConf struct {
	ChainParams      *chaincfg.Params
	DBDirPath        string
	DefaultDBTimeout time.Duration
	RecoveryWin      uint32
	Keyscope         waddrmgr.KeyScope
}

// Confirm that bchLoader implements the complete asset loader interface.
var _ loader.AssetLoader = (*bchLoader)(nil)

// NewLoader constructs a BCH Loader.
func NewLoader(cfg *LoaderConf) loader.AssetLoader {
	return &bchLoader{
		chainParams:    cfg.ChainParams,
		dbTimeout:      cfg.DefaultDBTimeout,
		recoveryWindow: cfg.RecoveryWin,
		keyscope:       cfg.Keyscope,

		Loader: loader.NewLoader(cfg.DBDirPath),
	}
}

// getWalletLoader creates the bch loader by configuring the path with the
// provided parameters. If createIfNotFound the missing directory path is created.
// This is mostly done when new wallets are being created. When reading existing
// wallets createIfNotFound is set to false signifying that if the path doesn't
// it can't be created. Lack of that path triggers an error.
func (l *bchLoader) getWalletLoader(walletID string, createIfNotFound bool) (*wallet.Loader, error) {
	var err error
	var dbpath string

	if createIfNotFound {
		// If the directory path doesn't exists, it creates it.
		dbpath, err = l.CreateDirPath(walletID, btcwallet.WalletDBName, utils.BCHWalletAsset)
		if err != nil {
			return nil, err
		}
	} else {
		var exists bool
		// constructs and checks if the file path exists
		dbpath, exists, err = l.FileExists(walletID, btcwallet.WalletDBName, utils.BCHWalletAsset)
		if err != nil {
			return nil, err
		}

		if !exists {
			return nil, fmt.Errorf("missing db at path %v", dbpath)
		}
	}

	// strip the db file name from the path
	path := filepath.Dir(dbpath)

	// Loader takes the db path without the actual db attached it.
	ldr := wallet.NewLoader(l.chainParams, path, true, uint32(l.dbTimeout))
	return ldr, nil
}

// CreateNewWallet creates a new wallet using the provided walletID, public and private
// passphrases.
func (l *bchLoader) CreateNewWallet(_ context.Context, params *loader.CreateWalletParams) (*loader.LoadedWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	defer func() {
		for i := range params.Seed {
			params.Seed[i] = 0
		}
	}()

	if l.wallet != nil {
		return nil, errors.New("wallet already opened")
	}

	ldr, err := l.getWalletLoader(params.WalletID, true)
	if err != nil {
		return nil, err
	}

	if len(params.Seed) == 0 {
		return nil, errors.New("ErrEmptySeed")
	}

	wal, err := ldr.CreateNewWallet(params.PubPassphrase, params.PrivPassphrase, params.Seed, time.Now())
	if err != nil {
		log.Errorf("Failed to create new wallet bch wallet: %v", err)
		return nil, err
	}

	l.wallet = wal

	return &loader.LoadedWallets{BCH: wal}, nil
}

// CreateWatchingOnlyWallet creates a new watch-only wallet using the provided
// walletID, extended public key and public passphrase.
func (l *bchLoader) CreateWatchingOnlyWallet(_ context.Context, params *loader.WatchOnlyWalletParams) (*loader.LoadedWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	// if l.wallet != nil {
	// 	return nil, errors.New("wallet already loaded")
	// }

	// ldr, err := l.getWalletLoader(params.WalletID, true)
	// if err != nil {
	// 	return nil, err
	// }

	// wal, err := ldr.CreateNewWatchingOnlyWallet(params.PubPassphrase, time.Now().UTC())
	// if err != nil {
	// 	return nil, err
	// }

	// // Create extended key from the xpub string.
	// extendedKety, err := hdkeychain.NewKeyFromString(params.ExtendedPubKey)
	// if err != nil {
	// 	return nil, err
	// }

	// // ImportAccountWithScope imports an account into the newly created watch-only wallet
	// // using the supported scope. The first parameter "default" will be the imported account's
	// // name, It doesn't matter what the account name use to be on a previous wallet.
	// //  Since the MasterFingerPrint is not provided when inputing the extended
	// // public key, 0 is set instead.
	// addrSchema := waddrmgr.ScopeAddrMap[l.keyscope]
	// _, err = wal.ImportAccountWithScope("default", extendedKety, 0, l.keyscope, addrSchema)
	// if err != nil {
	// 	return nil, err
	// }

	// l.wallet = wal

	return &loader.LoadedWallets{BCH: nil}, nil
}

// OpenExistingWallet opens the wallet from the loader's wallet database path
// and the public passphrase.
func (l *bchLoader) OpenExistingWallet(_ context.Context, walletID string, pubPassphrase []byte) (*loader.LoadedWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	ldr, err := l.getWalletLoader(walletID, false)
	if err != nil {
		return nil, err
	}

	wal, err := ldr.OpenExistingWallet(pubPassphrase, false)
	if err != nil {
		log.Errorf("Failed to open existing bch wallet: %v", err)
		return nil, err
	}
	l.wallet = wal

	return &loader.LoadedWallets{BCH: wal}, nil
}

// GetDbDirPath returns the Loader's database directory path
func (l *bchLoader) GetDbDirPath() string {
	defer l.mu.RUnlock()
	l.mu.RLock()

	return filepath.Join(l.DbDirPath, utils.BCHWalletAsset.ToStringLower())
}

// LoadedWallet returns the loaded wallet, if any, and a bool for whether the
// wallet has been loaded or not.  If true, the wallet pointer should be safe to
// dereference.
func (l *bchLoader) GetLoadedWallet() (*loader.LoadedWallets, bool) {
	l.mu.RLock()
	w := l.wallet
	l.mu.RUnlock()
	return &loader.LoadedWallets{BCH: w}, w != nil
}

// UnloadWallet stops the loaded wallet, returns errors if the wallet has not
// been loaded with CreateNewWallet or LoadExistingWallet.
func (l *bchLoader) UnloadWallet() error {
	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet == nil {
		return errors.New("wallet is unopened")
	}

	l.wallet = nil
	return nil
}

// WalletExists returns whether a file exists at the loader's database path.
// This may return an error for unexpected I/O failures.
func (l *bchLoader) WalletExists(walletID string) (bool, error) {
	defer l.mu.RUnlock()
	l.mu.RLock()

	_, exists, err := l.FileExists(walletID, btcwallet.WalletDBName, utils.BCHWalletAsset)
	if err != nil {
		return false, err
	}
	return exists, nil
}
