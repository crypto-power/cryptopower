package eth

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
)

type ethLoader struct {
	*loader.Loader

	wallet *keystore.KeyStore
	mu     sync.RWMutex
}

type LoaderConf struct {
	DBDirPath string
}

var _ loader.AssetLoader = (*ethLoader)(nil)

func NewLoader(cfg *LoaderConf) loader.AssetLoader {
	return &ethLoader{
		Loader: loader.NewLoader(cfg.DBDirPath),
	}
}

func (l *ethLoader) CreateNewWallet(ctx context.Context, params *loader.CreateWalletParams) (*loader.LoaderWallets, error) {
	ks, err := l.getWalletKeystore(params.WalletID, true)
	if err != nil {
		return nil, err
	}

	// generates a private key using the provided hashed seed.
	privKey, err := crypto.ToECDSA(params.Seed)
	if err != nil {
		return nil, err
	}

	// ImportECDSA stores the private key in the datadir as a json file.
	// if an account account generate using the same seed already exists,
	// an error is returned.
	_, err = ks.ImportECDSA(privKey, string(params.PrivPassphrase))
	if err != nil {
		return nil, err
	}

	return &loader.LoaderWallets{ETH: ks}, nil
}

// CreateWatchingOnlyWallet creates a new watch-only wallet using the provided
// walletID, extended public key and public passphrase.
func (l *ethLoader) CreateWatchingOnlyWallet(ctx context.Context, params *loader.WatchOnlyWalletParams) (*loader.LoaderWallets, error) {
	return nil, utils.ErrETHMethodNotImplemented("CreateWatchingOnlyWallet")
}

// OpenExistingWallet opens the wallet from the loader's wallet database path
// and the public passphrase.  If the loader is being called by a context where
// standard input prompts may be used during wallet upgrades, setting
// canConsolePrompt will enable these prompts.
func (l *ethLoader) OpenExistingWallet(ctx context.Context, WalletID string, pubPassphrase []byte) (*loader.LoaderWallets, error) {
	ks, err := l.getWalletKeystore(WalletID, false)
	if err != nil {
		return nil, err
	}

	if len(ks.Accounts()) == 0 {
		return nil, errors.New("found no existing ETH account")
	}

	return &loader.LoaderWallets{ETH: ks}, nil
}

// getWalletLoader creates the btc loader by configuring the path with the
// provided parameters. If createIfNotFound the missing directory path is created.
// This is mostly done when new wallets are being created. When reading existing
// wallets createIfNotFound is set to false signifying that if the path doesn't
// it can't be created. Lack of that path triggers an error.
func (l *ethLoader) getWalletKeystore(walletID string, createIfNotFound bool) (*keystore.KeyStore, error) {
	var err error
	var dbpath string

	if createIfNotFound {
		// If the directory path doesn't exists, it creates it.
		dbpath, err = l.CreateDirPath(walletID, "", utils.ETHWalletAsset)
		if err != nil {
			return nil, err
		}
	} else {
		var exists bool
		// constructs and checks if the file path exists
		dbpath, exists, err = l.FileExists(walletID, "", utils.ETHWalletAsset)
		if err != nil {
			return nil, err
		}

		if !exists {
			return nil, fmt.Errorf("missing db at path %v", dbpath)
		}
	}

	// strip the db file name from the path
	dbpath = filepath.Dir(dbpath)
	ks := keystore.NewKeyStore(dbpath, keystore.StandardScryptN, keystore.StandardScryptP)
	return ks, nil
}

// GetDbDirPath returns the Loader's database directory path
func (l *ethLoader) GetDbDirPath() string {
	defer l.mu.RUnlock()
	l.mu.RLock()

	return filepath.Join(l.DbDirPath, utils.ETHWalletAsset.ToStringLower())
}

// LoadedWallet returns the loaded wallet, if any, and a bool for whether the
// wallet has been loaded or not.  If true, the wallet pointer should be safe to
// dereference.
func (l *ethLoader) GetLoadedWallet() (*loader.LoaderWallets, bool) {
	l.mu.RLock()
	w := l.wallet
	l.mu.RUnlock()
	return &loader.LoaderWallets{ETH: w}, w != nil
}

// UnloadWallet stops the loaded wallet, returns errors if the wallet has not
// been loaded with CreateNewWallet or LoadExistingWallet.
func (l *ethLoader) UnloadWallet() error {
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
func (l *ethLoader) WalletExists(walletID string) (bool, error) {
	defer l.mu.RUnlock()
	l.mu.RLock()

	_, exists, err := l.FileExists(walletID, "", utils.ETHWalletAsset)
	if err != nil {
		return false, err
	}
	return exists, nil
}
