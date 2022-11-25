package btc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver

	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

var log = loader.Log

// btcLoader implements the creating of new and opening of existing btc wallets.
// This is primarily intended for use by the RPC servers, to enable
// methods and services which require the wallet when the wallet is loaded by
// another subsystem.
//
// btcLoader is safe for concurrent access.
type btcLoader struct {
	*loader.Loader

	chainParams *chaincfg.Params
	wallet      *wallet.Wallet

	recoveryWindow uint32
	dbTimeout      time.Duration

	mu sync.RWMutex
}

// LoaderConf models the configuration options of a btc loader.
type LoaderConf struct {
	ChainParams      *chaincfg.Params
	DBDirPath        string
	DefaultDBTimeout time.Duration
	RecoveryWin      uint32
}

// Confirm that btcLoader implements the complete asset loader interface.
var _ loader.AssetLoader = (*btcLoader)(nil)

// NewLoader constructs a BTC Loader.
func NewLoader(cfg *LoaderConf) loader.AssetLoader {

	return &btcLoader{
		chainParams:    cfg.ChainParams,
		dbTimeout:      cfg.DefaultDBTimeout,
		recoveryWindow: cfg.RecoveryWin,

		Loader: loader.NewLoader(cfg.DBDirPath),
	}
}

// getWalletLoader creates the btc loader by configuring the path with the
// provided parameters. If createIfNotFound the missing directory path is created.
// This is mostly done when new wallets are being created. When reading existing
// wallets createIfNotFound is set to false signifying that if the path doesn't
// it can't be created. Lack of that path triggers an error.
func (l *btcLoader) getWalletLoader(walletID string, createIfNotFound bool) (*wallet.Loader, error) {
	var err error
	var dbpath string

	if createIfNotFound {
		// If the directory path doesn't exists, it creates it.
		dbpath, err = l.CreateDirPath(walletID, wallet.WalletDBName, utils.BTCWalletAsset)
		if err != nil {
			return nil, err
		}
	} else {
		var exists bool
		// constructs and checks if the file path exists
		dbpath, exists, err = l.FileExists(walletID, wallet.WalletDBName, utils.BTCWalletAsset)
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
	ldr := wallet.NewLoader(l.chainParams, path, true, l.dbTimeout, l.recoveryWindow)
	return ldr, nil
}

// CreateNewWallet creates a new wallet using the provided walletID, public and private
// passphrases.
func (l *btcLoader) CreateNewWallet(ctx context.Context, params *loader.CreateWalletParams) (*loader.LoaderWallets, error) {
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

	wal, err := ldr.CreateNewWallet(params.PubPassphrase, params.PrivPassphrase, params.Seed, time.Now().UTC())
	if err != nil {
		log.Errorf("Failed to create new wallet btc wallet: %v", err)
		return nil, err
	}

	l.wallet = wal

	return &loader.LoaderWallets{BTC: wal}, nil
}

// CreateWatchingOnlyWallet creates a new watch-only wallet using the provided
// walletID, extended public key and public passphrase.
func (l *btcLoader) CreateWatchingOnlyWallet(ctx context.Context, params *loader.WatchOnlyWalletParams) (*loader.LoaderWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet != nil {
		return nil, errors.New("wallet already loaded")
	}

	ldr, err := l.getWalletLoader(params.WalletID, true)
	if err != nil {
		return nil, err
	}

	wal, err := ldr.CreateNewWatchingOnlyWallet(params.PubPassphrase, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	// Newly created watch-only wallets is missing scope information.
	// Update wallet DB with scope data.
	err = walletdb.Update(wal.Database(), func(tx walletdb.ReadWriteTx) error {
		ns := tx.ReadWriteBucket([]byte("waddrmgr"))
		for scope, schema := range waddrmgr.ScopeAddrMap {
			_, err := wal.Manager.NewScopedKeyManager(
				ns, scope, schema,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Create extended key from the xpub string.
	extendedKety, err := hdkeychain.NewKeyFromString(params.ExtendedPubKey)
	if err != nil {
		return nil, err
	}

	// Import account into the newly created watch-only wallet.
	// The first argument to ImportAccount {default} will be the account when imported.
	// It doesn't matter what the account name use to be on a previous wallet, it'll be imported as default.
	addrType := waddrmgr.WitnessPubKey
	_, err = wal.ImportAccount("default", extendedKety, extendedKety.ParentFingerprint(), &addrType)
	if err != nil {
		return nil, err
	}

	l.wallet = wal

	return &loader.LoaderWallets{BTC: wal}, nil
}

// OpenExistingWallet opens the wallet from the loader's wallet database path
// and the public passphrase.
func (l *btcLoader) OpenExistingWallet(ctx context.Context, walletID string, pubPassphrase []byte) (*loader.LoaderWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	ldr, err := l.getWalletLoader(walletID, false)
	if err != nil {
		return nil, err
	}

	wal, err := ldr.OpenExistingWallet(pubPassphrase, false)
	if err != nil {
		log.Errorf("Failed to open existing btc wallet: %v", err)
		return nil, err
	}

	l.wallet = wal

	return &loader.LoaderWallets{BTC: wal}, nil
}

// GetDbDirPath returns the Loader's database directory path
func (l *btcLoader) GetDbDirPath() string {
	defer l.mu.RUnlock()
	l.mu.RLock()

	return filepath.Join(l.DbDirPath, utils.BTCWalletAsset.ToString())
}

// LoadedWallet returns the loaded wallet, if any, and a bool for whether the
// wallet has been loaded or not.  If true, the wallet pointer should be safe to
// dereference.
func (l *btcLoader) GetLoadedWallet() (*loader.LoaderWallets, bool) {
	l.mu.RLock()
	w := l.wallet
	l.mu.RUnlock()
	return &loader.LoaderWallets{BTC: w}, w != nil
}

// UnloadWallet stops the loaded wallet, returns errors if the wallet has not
// been loaded with CreateNewWallet or LoadExistingWallet.
func (l *btcLoader) UnloadWallet() error {
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
func (l *btcLoader) WalletExists(walletID string) (bool, error) {
	defer l.mu.RUnlock()
	l.mu.RLock()

	_, exists, err := l.FileExists(walletID, wallet.WalletDBName, utils.BTCWalletAsset)
	if err != nil {
		return false, err
	}
	return exists, nil
}
