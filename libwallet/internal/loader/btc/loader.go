package btc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver

	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

const walletDbName = "neutrino.db"

var log = loader.Log

// btcLoader implements the creating of new and opening of existing dcr wallets, while
// providing a callback system for other subsystems to handle the loading of a
// wallet.  This is primarely intended for use by the RPC servers, to enable
// methods and services which require the wallet when the wallet is loaded by
// another subsystem.
//
// btcLoader is safe for concurrent access.
type btcLoader struct {
	*loader.Loader

	chainParams *chaincfg.Params
	wallet      *wallet.Wallet
	db          walletdb.DB

	recoveryWindow uint32
	dbTimeout      time.Duration

	mu sync.RWMutex
}

// Confirm that btcLoader implements the complete asset loader interface.
var _ loader.AssetLoader = (*btcLoader)(nil)

// NewLoader constructs a BTC Loader.
func NewLoader(chainParams *chaincfg.Params, dbDirPath string, defaultDBTimeout time.Duration, recoveryWin uint32) loader.AssetLoader {

	return &btcLoader{
		chainParams:    chainParams,
		dbTimeout:      defaultDBTimeout,
		recoveryWindow: recoveryWin,

		Loader: loader.NewLoader(dbDirPath),
	}
}

// CreateNewWallet creates a new wallet using the provided walletID, public and private
// passphrases.  The seed is optional.  If non-nil, addresses are derived from
// this seed.  If nil, a secure random seed is generated.
func (l *btcLoader) CreateNewWallet(ctx context.Context, strWalletID string, pubPassphrase, privPassphrase, seed []byte) (*loader.LoaderWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet != nil {
		return nil, errors.New("wallet already opened")
	}

	// If the directory path doesn't exists, it creates it.
	neutrinoDBPath, err := l.CreateDirPath(strWalletID, utils.BTCWalletAsset, walletDbName)
	if err != nil {
		return nil, err
	}

	// Creates the db instance at the provided path.
	db, err := walletdb.Create(l.DbDriver, neutrinoDBPath, false, l.dbTimeout)
	if err != nil {
		return nil, fmt.Errorf("unable to create wallet db at %q: %v", neutrinoDBPath, err)
	}

	walletExists := func() (bool, error) { return true, nil }

	ldr, err := wallet.NewLoaderWithDB(l.chainParams, l.recoveryWindow, db, walletExists)
	if err != nil {
		return nil, err
	}

	wal, err := ldr.CreateNewWallet(pubPassphrase, privPassphrase, seed, time.Now().UTC())
	if err != nil {
		log.Errorf("Failed to create new wallet btc wallet: %v", err)
		return nil, err
	}

	l.db = db
	l.wallet = wal

	return &loader.LoaderWallets{BTC: wal}, nil
}

// CreateWatchingOnlyWallet creates a new watch-only wallet using the provided
// walletID, extended public key and public passphrase.
func (l *btcLoader) CreateWatchingOnlyWallet(ctx context.Context, strWalletID string, extendedPubKey string, pubPass []byte) (*loader.LoaderWallets, error) {
	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet != nil {
		return nil, errors.New("wallet already loaded")
	}
	// If the directory path doesn't exists, it creates it.
	neutrinoDBPath, err := l.CreateDirPath(strWalletID, utils.BTCWalletAsset, walletDbName)
	if err != nil {
		return nil, err
	}

	// Creates the db instance at the provided path.
	db, err := walletdb.Create(l.DbDriver, neutrinoDBPath, false, l.dbTimeout)
	if err != nil {
		return nil, fmt.Errorf("unable to create wallet db at %q: %v", neutrinoDBPath, err)
	}

	walletExists := func() (bool, error) { return true, nil }

	ldr, err := wallet.NewLoaderWithDB(l.chainParams, l.recoveryWindow, db, walletExists)
	if err != nil {
		return nil, err
	}

	wal, err := ldr.CreateNewWatchingOnlyWallet(pubPass, time.Now().UTC())
	if err != nil {
		log.Errorf("Failed to create watch only btc wallet: %v", err)
		return nil, err
	}

	return &loader.LoaderWallets{BTC: wal}, nil
}

// OpenExistingWallet opens the wallet from the loader's wallet database path
// and the public passphrase.
func (l *btcLoader) OpenExistingWallet(ctx context.Context, strWalletID string, pubPassphrase []byte) (*loader.LoaderWallets, error) {
	fmt.Println(" >>>>>>>> GOT HERE <<<<< ")
	defer l.mu.Unlock()
	l.mu.Lock()
	fmt.Println(" >>>>>>>> 111 GOT HERE <<<<< ")
	// constructs and checks if the file path exists
	neutrinoDBPath, exists, err := l.FileExists(strWalletID, utils.BTCWalletAsset, walletDbName)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("missing db at path %v", neutrinoDBPath)
	}

	db, err := walletdb.Open(l.DbDriver, neutrinoDBPath, false, l.dbTimeout)
	if err != nil {
		return nil, fmt.Errorf("unable to open wallet db at %q: %v", neutrinoDBPath, err)
	}
	fmt.Println(" >>>>>>>> 22 GOT HERE <<<<< ")

	walletExists := func() (bool, error) { return false, nil }

	ldr, err := wallet.NewLoaderWithDB(l.chainParams, l.recoveryWindow, db, walletExists)
	if err != nil {
		return nil, err
	}
	fmt.Println(" >>>>>>>> 33 GOT HERE <<<<< ")

	wal, err := ldr.OpenExistingWallet(pubPassphrase, false)
	if err != nil {
		log.Errorf("Failed to open existing btc wallet: %v", err)
		return nil, err
	}

	fmt.Println(" >>>>>>>> 44 GOT HERE <<<<< ")

	return &loader.LoaderWallets{BTC: wal}, nil
}

// GetDbDirPath returns the Loader's database directory path
func (l *btcLoader) GetDbDirPath() string {
	defer l.mu.RUnlock()
	l.mu.RLock()

	return filepath.Join(l.DbDirPath, string(utils.BTCWalletAsset))
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

	err := l.db.Close()
	if err != nil {
		return err
	}

	l.db = nil
	l.wallet = nil
	return nil
}

// WalletExists returns whether a file exists at the loader's database path.
// This may return an error for unexpected I/O failures.
func (l *btcLoader) WalletExists(strWalletID string) (bool, error) {
	defer l.mu.RUnlock()
	l.mu.RLock()

	_, exists, err := l.FileExists(strWalletID, utils.BTCWalletAsset, walletDbName)
	if err != nil {
		return false, err
	}
	return exists, nil
}
