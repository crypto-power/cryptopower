package loader

import (
	"context"
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v3/errors"
	dcrW "decred.org/dcrwallet/v3/wallet"
	btcW "github.com/btcsuite/btcwallet/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	ltcW "github.com/dcrlabs/ltcwallet/wallet"

	_ "github.com/crypto-power/cryptopower/libwallet/badgerdb" // initialize badger driver
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

// LoadedWallets holds all the upstream wallets managed by the loader
type LoadedWallets struct {
	BTC *btcW.Wallet
	DCR *dcrW.Wallet
	LTC *ltcW.Wallet
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

	OpenExistingWallet(ctx context.Context, WalletID string, pubPassphrase []byte) (*LoadedWallets, error)
	CreateNewWallet(ctx context.Context, params *CreateWalletParams) (*LoadedWallets, error)
	CreateWatchingOnlyWallet(ctx context.Context, params *WatchOnlyWalletParams) (*LoadedWallets, error)

	GetLoadedWallet() (*LoadedWallets, bool)
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
	if file, err := os.Stat(folderPath); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err = os.MkdirAll(folderPath, utils.UserFilePerm); err != nil {
			return "", err
		}
	} else if !file.IsDir() {
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
