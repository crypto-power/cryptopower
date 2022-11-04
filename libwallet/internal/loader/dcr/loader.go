// Copyright (c) 2015-2018 The btcsuite developers
// Copyright (c) 2017-2022 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcr

import (
	"context"
	"path/filepath"
	"sync"

	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"

	_ "decred.org/dcrwallet/v2/wallet/drivers/bdb" // driver loaded during init
)

const walletDbName = "wallet.db"

var log = loader.Log

// dcrLoader implements the creating of new and opening of existing dcr wallets, while
// providing a callback system for other subsystems to handle the loading of a
// wallet.  This is primarily intended for use by the RPC servers, to enable
// methods and services which require the wallet when the wallet is loaded by
// another subsystem.
//
// dcrLoader is safe for concurrent access.
type dcrLoader struct {
	*loader.Loader

	callbacks   []func(*wallet.Wallet)
	chainParams *chaincfg.Params
	wallet      *wallet.Wallet
	db          wallet.DB

	stakeOptions            *StakeOptions
	gapLimit                uint32
	accountGapLimit         int
	disableCoinTypeUpgrades bool
	allowHighFees           bool
	manualTickets           bool
	relayFee                dcrutil.Amount
	mixSplitLimit           int

	mu sync.RWMutex
}

// StakeOptions contains the various options necessary for stake mining.
type StakeOptions struct {
	VotingEnabled       bool
	AddressReuse        bool
	VotingAddress       stdaddr.StakeAddress
	PoolAddress         stdaddr.StakeAddress
	PoolFees            float64
	StakePoolColdExtKey string
}

// Confirm the dcrLoader implements the assets loader interface.
var _ loader.AssetLoader = (*dcrLoader)(nil)

// LoaderConf models the configuration options of a dcr loader.
type LoaderConf struct {
	ChainParams             *chaincfg.Params
	DBDirPath               string
	StakeOptions            *StakeOptions
	GapLimit                uint32
	RelayFee                dcrutil.Amount
	AllowHighFees           bool
	DisableCoinTypeUpgrades bool
	ManualTickets           bool
	AccountGapLimit         int
	MixSplitLimit           int
}

// NewLoader constructs a DCR Loader.
func NewLoader(cfg *LoaderConf) loader.AssetLoader {

	return &dcrLoader{
		chainParams:             cfg.ChainParams,
		stakeOptions:            cfg.StakeOptions,
		gapLimit:                cfg.GapLimit,
		accountGapLimit:         cfg.AccountGapLimit,
		disableCoinTypeUpgrades: cfg.DisableCoinTypeUpgrades,
		allowHighFees:           cfg.AllowHighFees,
		manualTickets:           cfg.ManualTickets,
		relayFee:                cfg.RelayFee,
		mixSplitLimit:           cfg.MixSplitLimit,

		Loader: loader.NewLoader(cfg.DBDirPath),
	}
}

// onLoaded executes each added callback and prevents loader from loading any
// additional wallets.  Requires mutex to be locked.
func (l *dcrLoader) onLoaded(w *wallet.Wallet, db wallet.DB) {
	for _, fn := range l.callbacks {
		fn(w)
	}

	l.wallet = w
	l.db = db
	l.callbacks = nil // not needed anymore
}

// RunAfterLoad adds a function to be executed when the loader creates or opens
// a wallet. Functions are executed in a single goroutine in the order they are
// added.
func (l *dcrLoader) RunAfterLoad(fn func(*wallet.Wallet)) {
	l.mu.Lock()
	if l.wallet != nil {
		w := l.wallet
		l.mu.Unlock()
		fn(w)
	} else {
		l.callbacks = append(l.callbacks, fn)
		l.mu.Unlock()
	}
}

// CreateWatchingOnlyWallet creates a new watch-only wallet using the provided
// walletID, extended public key and public passphrase.
func (l *dcrLoader) CreateWatchingOnlyWallet(ctx context.Context, params *loader.WatchOnlyWalletParams) (*loader.LoaderWallets, error) {
	const op errors.Op = "loader.CreateWatchingOnlyWallet"

	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet != nil {
		return nil, errors.E(op, errors.Exist, "wallet already loaded")
	}

	dbPath, err := l.CreateDirPath(params.WalletID, walletDbName, utils.DCRWalletAsset)
	if err != nil {
		return nil, errors.E(op, err)
	}

	db, err := wallet.CreateDB(l.DbDriver, dbPath)
	if err != nil {
		return nil, errors.E(op, err)
	}

	// Initialize the watch-only database for the wallet before opening.
	err = wallet.CreateWatchOnly(ctx, db, params.ExtendedPubKey, params.PubPassphrase, l.chainParams)
	if err != nil {
		return nil, errors.E(op, err)
	}

	// Open the watch-only wallet.
	so := l.stakeOptions
	cfg := &wallet.Config{
		DB:                      db,
		PubPassphrase:           params.PubPassphrase,
		VotingEnabled:           so.VotingEnabled,
		AddressReuse:            so.AddressReuse,
		VotingAddress:           so.VotingAddress,
		PoolAddress:             so.PoolAddress,
		PoolFees:                so.PoolFees,
		GapLimit:                l.gapLimit,
		AccountGapLimit:         l.accountGapLimit,
		DisableCoinTypeUpgrades: l.disableCoinTypeUpgrades,
		StakePoolColdExtKey:     so.StakePoolColdExtKey,
		ManualTickets:           l.manualTickets,
		AllowHighFees:           l.allowHighFees,
		RelayFee:                l.relayFee,
		MixSplitLimit:           l.mixSplitLimit,
		Params:                  l.chainParams,
	}
	w, err := wallet.Open(ctx, cfg)
	if err != nil {
		return nil, errors.E(op, err)
	}

	l.onLoaded(w, db)
	return &loader.LoaderWallets{DCR: w}, nil
}

// CreateNewWallet creates a new wallet using the provided walletID, public and private
// passphrases.  The seed is optional.  If non-nil, addresses are derived from
// this seed.  If nil, a secure random seed is generated.
func (l *dcrLoader) CreateNewWallet(ctx context.Context, params *loader.CreateWalletParams) (*loader.LoaderWallets, error) {
	const op errors.Op = "loader.CreateNewWallet"

	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet != nil {
		return nil, errors.E(op, errors.Exist, "wallet already opened")
	}

	dbPath, err := l.CreateDirPath(params.WalletID, walletDbName, utils.DCRWalletAsset)
	if err != nil {
		return nil, errors.E(op, err)
	}

	db, err := wallet.CreateDB(l.DbDriver, dbPath)
	if err != nil {
		return nil, errors.E(op, err)
	}

	// Initialize the newly created database for the wallet before opening.
	err = wallet.Create(ctx, db, params.PubPassphrase, params.PrivPassphrase, params.Seed, l.chainParams)
	if err != nil {
		return nil, errors.E(op, err)
	}

	// Open the newly-created wallet.
	so := l.stakeOptions
	cfg := &wallet.Config{
		DB:                      db,
		PubPassphrase:           params.PubPassphrase,
		VotingEnabled:           so.VotingEnabled,
		AddressReuse:            so.AddressReuse,
		VotingAddress:           so.VotingAddress,
		PoolAddress:             so.PoolAddress,
		PoolFees:                so.PoolFees,
		GapLimit:                l.gapLimit,
		AccountGapLimit:         l.accountGapLimit,
		DisableCoinTypeUpgrades: l.disableCoinTypeUpgrades,
		StakePoolColdExtKey:     so.StakePoolColdExtKey,
		ManualTickets:           l.manualTickets,
		AllowHighFees:           l.allowHighFees,
		RelayFee:                l.relayFee,
		Params:                  l.chainParams,
	}
	w, err := wallet.Open(ctx, cfg)
	if err != nil {
		return nil, errors.E(op, err)
	}

	l.onLoaded(w, db)
	return &loader.LoaderWallets{DCR: w}, nil
}

// OpenExistingWallet opens the wallet from the loader's wallet database path
// and the public passphrase.  If the loader is being called by a context where
// standard input prompts may be used during wallet upgrades, setting
// canConsolePrompt will enable these prompts.
func (l *dcrLoader) OpenExistingWallet(ctx context.Context, walletID string, pubPassphrase []byte) (*loader.LoaderWallets, error) {
	const op errors.Op = "loader.OpenExistingWallet"

	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet != nil {
		return nil, errors.E(op, errors.Exist, "wallet already opened")
	}

	var err error

	// Open the database using the boltdb backend.
	dbPath, _, err := l.FileExists(walletID, walletDbName, utils.DCRWalletAsset)
	if err != nil {
		log.Warnf("unable to open wallet db at %q: %v", dbPath, err)
		return nil, errors.E(op, err)
	}

	db, err := wallet.OpenDB(l.DbDriver, dbPath)
	if err != nil {
		log.Errorf("Failed to open database: %v", err)
		return nil, errors.E(op, err)
	}
	// If this function does not return to completion the database must be
	// closed.  Otherwise, because the database is locked on opens, any
	// other attempts to open the wallet will hang, and there is no way to
	// recover since this db handle would be leaked.
	defer func() {
		if err != nil {
			db.Close()
		}
	}()

	so := l.stakeOptions
	cfg := &wallet.Config{
		DB:                      db,
		PubPassphrase:           pubPassphrase,
		VotingEnabled:           so.VotingEnabled,
		AddressReuse:            so.AddressReuse,
		VotingAddress:           so.VotingAddress,
		PoolAddress:             so.PoolAddress,
		PoolFees:                so.PoolFees,
		GapLimit:                l.gapLimit,
		AccountGapLimit:         l.accountGapLimit,
		DisableCoinTypeUpgrades: l.disableCoinTypeUpgrades,
		StakePoolColdExtKey:     so.StakePoolColdExtKey,
		ManualTickets:           l.manualTickets,
		AllowHighFees:           l.allowHighFees,
		RelayFee:                l.relayFee,
		MixSplitLimit:           l.mixSplitLimit,
		Params:                  l.chainParams,
	}
	w, err := wallet.Open(ctx, cfg)
	if err != nil {
		return nil, errors.E(op, err)
	}

	l.onLoaded(w, db)
	return &loader.LoaderWallets{DCR: w}, nil
}

// GetDbDirPath returns the Loader's database directory path
func (l *dcrLoader) GetDbDirPath() string {
	defer l.mu.RUnlock()
	l.mu.RLock()

	return filepath.Join(l.DbDirPath, utils.DCRWalletAsset.ToStringLower())
}

// WalletExists returns whether a file exists at the loader's database path.
// This may return an error for unexpected I/O failures.
func (l *dcrLoader) WalletExists(walletID string) (bool, error) {
	defer l.mu.RUnlock()
	l.mu.RLock()

	const op errors.Op = "loader.WalletExists"
	_, exists, err := l.FileExists(walletID, walletDbName, utils.DCRWalletAsset)
	if err != nil {
		return false, errors.E(op, err)
	}
	return exists, nil
}

// LoadedWallet returns the loaded wallet, if any, and a bool for whether the
// wallet has been loaded or not.  If true, the wallet pointer should be safe to
// dereference.
func (l *dcrLoader) GetLoadedWallet() (*loader.LoaderWallets, bool) {
	l.mu.RLock()
	w := l.wallet
	l.mu.RUnlock()
	return &loader.LoaderWallets{DCR: w}, w != nil
}

// UnloadWallet stops the loaded wallet, if any, and closes the wallet database.
// Returns with errors.Invalid if the wallet has not been loaded with
// CreateNewWallet or LoadExistingWallet.  The Loader may be reused if this
// function returns without error.
func (l *dcrLoader) UnloadWallet() error {
	const op errors.Op = "loader.UnloadWallet"

	defer l.mu.Unlock()
	l.mu.Lock()

	if l.wallet == nil {
		return errors.E(op, errors.Invalid, "wallet is unopened")
	}

	err := l.db.Close()
	if err != nil {
		return errors.E(op, err)
	}

	l.wallet = nil
	l.db = nil
	return nil
}

// NetworkBackend returns the associated wallet network backend, if any, and a
// bool describing whether a non-nil network backend was set.
func (l *dcrLoader) NetworkBackend() (n wallet.NetworkBackend, ok bool) {
	l.mu.Lock()
	if l.wallet != nil {
		n, _ = l.wallet.NetworkBackend()
	}
	l.mu.Unlock()
	return n, n != nil
}
