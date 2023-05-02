package loader

import (
	"context"
	"os"
	"path/filepath"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	dcrW "decred.org/dcrwallet/v2/wallet"
	btcW "github.com/btcsuite/btcwallet/wallet"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ltcW "github.com/ltcsuite/ltcwallet/wallet"

	_ "code.cryptopower.dev/group/cryptopower/libwallet/badgerdb" // initialize badger driver
)

const defaultDbDriver = "bdb"

type Loader struct {
	// The full db path required to create the individual wallets follows the
	// format ~.cryptopower/[network_selected]/[asset_selected]/[wallet_ID].
	// DbDirPath by default is only expected to hold the path upto the
	// [network_selected] folder. The other details are added on demand.
	DbDirPath string
	// DbDriver defines the type of database driver in use by the individual
	// wallets.
	DbDriver string
}

// EthWalletInfo helps expose the accounts wallet interface and the underlying
// keystore implementation used.
type EthWalletInfo struct {
	Keystore *keystore.KeyStore
	Wallet   accounts.Wallet // Wallet interface
}

// LoaderWallets holds all the upstream wallets managed by the loader
type LoaderWallets struct {
	BTC *btcW.Wallet
	DCR *dcrW.Wallet
	LTC *ltcW.Wallet
	ETH *EthWalletInfo
}

type WatchOnlyWalletParams struct {
	WalletID       string
	ExtendedPubKey string
	PubPassphrase  []byte
}

type CreateWalletParams struct {
	WalletID       string
	PubPassphrase  []byte
	PrivPassphrase []byte
	Seed           []byte
}

// AssetLoader defines the interface exported by the loader implementation
// of each asset.
type AssetLoader interface {
	GetDbDirPath() string
	SetDatabaseDriver(driver string)

	OpenExistingWallet(ctx context.Context, WalletID string, pubPassphrase []byte) (*LoaderWallets, error)
	CreateNewWallet(ctx context.Context, params *CreateWalletParams) (*LoaderWallets, error)
	CreateWatchingOnlyWallet(ctx context.Context, params *WatchOnlyWalletParams) (*LoaderWallets, error)

	GetLoadedWallet() (*LoaderWallets, bool)
	UnloadWallet() error
	WalletExists(WalletID string) (bool, error)
}

func NewLoader(dbDirPath string) *Loader {
	return &Loader{
		DbDirPath: dbDirPath,
		DbDriver:  defaultDbDriver,
	}
}

// /SetDatabaseDriver specifies the database to be used by walletdb
func (l *Loader) SetDatabaseDriver(driver string) {
	l.DbDriver = driver
}

// CreateDirPath checks that fully qualified path to the wallet bucket exists.
// If it doesn't exist it's created. It also checks if the actual db file
// required exists, if it exists an error is returned otherwise it's created.
// Since the fully qualified path of the db is as follows:
// ~.cryptopower/[network_selected]/[asset_selected]/[wallet_ID]/[WalletDbName.db].
// l.DbDirPath provides the path of the network_selected.
// assetType provides the asset_selected name,
// WalletID provides the wallet Id of the wallet needed.
// assetType and WalletID are provided for every new bucket instance needed.
func (l *Loader) CreateDirPath(WalletID, walletDbName string, assetType utils.AssetType) (dbPath string, err error) {
	// At this point it is asserted that there is no existing database file, and
	// deleting anything won't destroy a wallet in use.  Defer a function that
	// attempts to remove any written database file if this function errors.
	defer func() {
		if err != nil {
			_ = os.Remove(dbPath)
		}
	}()

	folderPath := filepath.Join(l.DbDirPath, assetType.ToStringLower(), WalletID)
	// Ensure that the network directory exists.
	file, err := os.Stat(folderPath)
	if err != nil {
		if os.IsNotExist(err) {
			// error expected thus now attempt data directory creation
			if err = os.MkdirAll(folderPath, 700); err != nil {
				return "", err
			}
		} else {
			// if os.IsNotExist(err) returned false means that unexpected error
			// was returned thus exit the function with that error.
			return "", err
		}
	}

	// No error was returned. Else used here to maintain the scope of
	// file variable just with the if-else statement.
	if !file.IsDir() {
		return "", errors.Errorf("%q is not a directory", folderPath)
	}

	// Ensure the target db file doesn't exist.
	dbPath = filepath.Join(folderPath, walletDbName)
	exists, err := fileExists(dbPath)
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.Errorf("wallet DB already exists")
	}

	return
}

// FileExists checks if db bucket path identified by the following parameters
// exists.
func (l *Loader) FileExists(WalletID, walletDbName string, assetType utils.AssetType) (string, bool, error) {
	path := filepath.Join(l.DbDirPath, assetType.ToStringLower(), WalletID, walletDbName)
	b, err := fileExists(path)
	return path, b, err
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
