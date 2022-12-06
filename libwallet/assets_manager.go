package libwallet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"code.cryptopower.dev/group/cryptopower/libwallet/ext"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/politeia"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	bolt "go.etcd.io/bbolt"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
)

type Assets struct {
	DCR struct {
		Wallets    map[int]sharedW.Asset
		BadWallets map[int]*sharedW.Wallet
	}
	BTC struct {
		Wallets    map[int]sharedW.Asset
		BadWallets map[int]*sharedW.Wallet
	}
}

type AssetsManager struct {
	params *sharedW.InitParams
	Assets *Assets

	db sharedW.AssetsManagerDB // Interface to manage db access at the ASM.

	shuttingDown chan bool
	cancelFuncs  []context.CancelFunc
	chainsParams utils.ChainsParams

	Politeia        *politeia.Politeia
	ExternalService *ext.Service
}

// initializeAssetsFields validate the network provided is valid for all assets before proceeding
// to initialize the rest of the other fields.
func initializeAssetsFields(rootDir, dbDriver, logDir string, netType utils.NetworkType) (*AssetsManager, error) {
	dcrChainParams, err := initializeDCRWalletParameters(netType)
	if err != nil {
		log.Errorf("error initializing DCR parameters: %s", err.Error())
		return nil, errors.Errorf("error initializing DCR parameters: %s", err.Error())
	}

	btcChainParams, err := initializeBTCWalletParameters(netType)
	if err != nil {
		log.Errorf("error initializing BTC parameters: %s", err.Error())
		return nil, errors.Errorf("error initializing BTC parameters: %s", err.Error())
	}

	params := &sharedW.InitParams{
		DbDriver: dbDriver,
		RootDir:  rootDir,
		NetType:  netType,
		LogDir:   logDir,
	}

	mgr := &AssetsManager{
		params: params,
		Assets: new(Assets),
	}

	mgr.Assets.BTC.Wallets = make(map[int]sharedW.Asset)
	mgr.Assets.DCR.Wallets = make(map[int]sharedW.Asset)

	mgr.Assets.BTC.BadWallets = make(map[int]*sharedW.Wallet)
	mgr.Assets.DCR.BadWallets = make(map[int]*sharedW.Wallet)

	mgr.chainsParams.DCR = dcrChainParams
	mgr.chainsParams.BTC = btcChainParams
	return mgr, nil
}

func NewAssetsManager(rootDir, dbDriver, net, politeiaHost, logDir string) (*AssetsManager, error) {
	errors.Separator = ":: "

	netType := utils.NetworkType(net)

	// Create a root dir that has the path up the network folder.
	rootDir = filepath.Join(rootDir, net)
	if err := os.MkdirAll(rootDir, os.ModePerm); err != nil {
		return nil, errors.Errorf("failed to create rootDir: %v", err)
	}

	// validate the network type before proceeding to initialize the othe fields.
	mgr, err := initializeAssetsFields(rootDir, dbDriver, logDir, netType)
	if err != nil {
		return nil, err
	}

	if err := initLogRotator(filepath.Join(rootDir, logFileName)); err != nil {
		return nil, errors.Errorf("failed to init logRotator: %v", err.Error())
	}

	// Attempt to acquire lock on the wallets.db file.
	mwDB, err := storm.Open(filepath.Join(rootDir, walletsDbName))
	if err != nil {
		log.Errorf("Error opening wallets database: %s", err.Error())
		if err == bolt.ErrTimeout {
			// timeout error occurs if storm fails to acquire a lock on the database file
			return nil, errors.E(utils.ErrWalletDatabaseInUse)
		}
		return nil, errors.Errorf("error opening wallets database: %s", err.Error())
	}

	// init database for persistence of wallet objects
	if err = mwDB.Init(&sharedW.Wallet{}); err != nil {
		log.Errorf("Error initializing wallets database: %s", err.Error())
		return nil, err
	}

	politeia, err := politeia.New(politeiaHost, mwDB)
	if err != nil {
		return nil, err
	}

	mgr.params.DB = mwDB
	mgr.Politeia = politeia

	// initialize the ExternalService. ExternalService provides multiwallet with
	// the functionalities to retrieve data from 3rd party services. e.g Binance, Bittrex.
	mgr.ExternalService = ext.NewService(mgr.chainsParams.DCR)

	// Load existing wallets.
	if err := mgr.prepareExistingWallets(); err != nil {
		return nil, err
	}

	log.Infof("Loaded %d wallets", mgr.LoadedWalletsCount())

	// Attempt to set the log levels if a valid db interface was found.
	if mgr.db != nil {
		mgr.GetLogLevels()
	}

	mgr.listenForShutdown()

	return mgr, nil
}

// prepareExistingWallets loads all the valid and bad wallets. It also attempts
// to extract the assets manager db access interface from one of the validly
// created wallets.
func (mgr *AssetsManager) prepareExistingWallets() error {
	// read all stored wallets info from the db and initialize wallets interfaces.
	query := mgr.params.DB.Select(q.True()).OrderBy("ID")
	var wallets []*sharedW.Wallet
	err := query.Find(&wallets)
	if err != nil && err != storm.ErrNotFound {
		return err
	}

	isOK := func(val interface{}) bool {
		var ok bool
		if val != nil {
			// Extracts the walletExists method and checks if the current wallet
			// walletDataDb file exists. Returns true if affirmative.
			ok, _ = val.(interface{ WalletExists() (bool, error) }).WalletExists()
			// Extracts the asset manager db interface from one of the wallets.
			// Assets Manager Db interface that exists in all wallets by default.
			if mgr.db == nil {
				mgr.setDBInterface(val.(sharedW.AssetsManagerDB))
			}
		}
		return ok
	}

	// prepare the wallets loaded from db for use
	for _, wallet := range wallets {
		path := filepath.Join(mgr.params.RootDir, wallet.DataDir())
		log.Infof("loading properties of wallet=%v at location=%v", wallet.Name, path)

		switch wallet.Type {
		case utils.BTCWalletAsset:
			w, err := btc.LoadExisting(wallet, mgr.params)
			if err == nil && !isOK(w) {
				err = fmt.Errorf("missing wallet database file: %v", path)
				log.Warn(err)
			}
			if err != nil {
				mgr.Assets.BTC.BadWallets[wallet.ID] = wallet
				log.Warnf("Ignored btc wallet load error for wallet %d (%s)", wallet.ID, wallet.Name)
			} else {
				mgr.Assets.BTC.Wallets[wallet.ID] = w
			}

		case utils.DCRWalletAsset:
			w, err := dcr.LoadExisting(wallet, mgr.params)
			if err == nil && !isOK(w) {
				err = fmt.Errorf("missing wallet database file: %v", path)
				log.Debug(err)
			}
			if err != nil {
				mgr.Assets.DCR.BadWallets[wallet.ID] = wallet
				log.Warnf("Ignored dcr wallet load error for wallet %d (%s)", wallet.ID, wallet.Name)
			} else {
				mgr.Assets.DCR.Wallets[wallet.ID] = w
			}

		default:
			// Classify all wallets with missing AssetTypes as DCR badwallets.
			mgr.Assets.DCR.BadWallets[wallet.ID] = wallet
		}
	}
	return nil
}

func (mgr *AssetsManager) listenForShutdown() {
	mgr.cancelFuncs = make([]context.CancelFunc, 0)
	mgr.shuttingDown = make(chan bool)
	go func() {
		<-mgr.shuttingDown
		for _, cancel := range mgr.cancelFuncs {
			cancel()
		}
	}()
}

func (mgr *AssetsManager) Shutdown() {
	log.Info("Shutting down libwallet")

	// Trigger shuttingDown signal to cancel all contexts created with `shutdownContextWithCancel`.
	mgr.shuttingDown <- true

	for _, wallet := range mgr.Assets.DCR.Wallets {
		wallet.CancelRescan()
		wallet.Shutdown() // Cancels the wallet sync too.
	}

	for _, wallet := range mgr.Assets.BTC.Wallets {
		wallet.CancelRescan()
		wallet.Shutdown() // Cancels the wallet sync too.
	}

	if mgr.params.DB != nil {
		if err := mgr.params.DB.Close(); err != nil {
			log.Errorf("db closed with error: %v", err)
		} else {
			log.Info("db closed successfully")
		}
	}

	if logRotator != nil {
		log.Info("Shutting down log rotator")
		logRotator.Close()
		log.Info("Shutdown log rotator successfully")
	}
}

// TODO: cryptopower should start using networks constants defined in the
// utils package instead of strings
func (mgr *AssetsManager) NetType() utils.NetworkType {
	return mgr.params.NetType
}

func (mgr *AssetsManager) LogDir() string {
	return filepath.Join(mgr.params.RootDir, logFileName)
}

func (mgr *AssetsManager) OpenWallets(startupPassphrase string) error {
	for _, wallet := range mgr.Assets.DCR.Wallets {
		if wallet.IsSyncing() {
			return errors.New(utils.ErrSyncAlreadyInProgress)
		}
	}

	//TODO: Check if any of the btc wallets is syncing.

	if err := mgr.VerifyStartupPassphrase(startupPassphrase); err != nil {
		return err
	}

	for _, wallet := range mgr.Assets.DCR.Wallets {
		err := wallet.OpenWallet()
		if err != nil {
			return err
		}
	}

	for _, wallet := range mgr.Assets.BTC.Wallets {
		err := wallet.OpenWallet()
		if err != nil {
			return err
		}
	}
	return nil
}

func (mgr *AssetsManager) DCRBadWallets() map[int]*sharedW.Wallet {
	return mgr.Assets.DCR.BadWallets
}

func (mgr *AssetsManager) BTCBadWallets() map[int]*sharedW.Wallet {
	return mgr.Assets.BTC.BadWallets
}

func (mgr *AssetsManager) LoadedWalletsCount() int32 {
	return int32(len(mgr.Assets.DCR.Wallets) + len(mgr.Assets.BTC.Wallets))
}

func (mgr *AssetsManager) OpenedWalletsCount() int32 {
	var count int32
	for _, wallet := range mgr.Assets.DCR.Wallets {
		if wallet.WalletOpened() {
			count++
		}
	}
	for _, wallet := range mgr.Assets.BTC.Wallets {
		if wallet.WalletOpened() {
			count++
		}
	}
	return count
}

// PiKeys returns the sanctioned Politeia keys for the current network.
func (mgr *AssetsManager) PiKeys() [][]byte {
	return mgr.chainsParams.DCR.PiKeys
}

func (mgr *AssetsManager) AllDCRWallets() (wallets []sharedW.Asset) {
	for _, wallet := range mgr.Assets.DCR.Wallets {
		wallets = append(wallets, wallet)
	}
	return wallets
}

func (mgr *AssetsManager) AllBTCWallets() (wallets []sharedW.Asset) {
	for _, wallet := range mgr.Assets.BTC.Wallets {
		wallets = append(wallets, wallet)
	}
	return wallets
}

func (mgr *AssetsManager) DeleteWallet(walletID int, privPass string) error {
	wallet := mgr.WalletWithID(walletID)
	if err := wallet.DeleteWallet(privPass); err != nil {
		return err
	}

	switch wallet.GetAssetType() {
	case utils.BTCWalletAsset:
		delete(mgr.Assets.BTC.Wallets, walletID)
	case utils.DCRWalletAsset:
		delete(mgr.Assets.DCR.Wallets, walletID)
	}

	return nil
}

func (mgr *AssetsManager) WalletWithID(walletID int) sharedW.Asset {
	if wallet, ok := mgr.Assets.BTC.Wallets[walletID]; ok {
		return wallet
	}
	if wallet, ok := mgr.Assets.DCR.Wallets[walletID]; ok {
		return wallet
	}
	return nil
}

func (mgr *AssetsManager) getbadWallet(walletID int) *sharedW.Wallet {
	if badWallet, ok := mgr.Assets.BTC.BadWallets[walletID]; ok {
		return badWallet
	}
	if badWallet, ok := mgr.Assets.DCR.BadWallets[walletID]; ok {
		return badWallet
	}
	return nil
}

func (mgr *AssetsManager) DeleteBadWallet(walletID int) error {
	wallet := mgr.getbadWallet(walletID)
	if wallet == nil {
		return errors.New(utils.ErrNotExist)
	}

	log.Info("Deleting bad wallet")

	err := mgr.params.DB.DeleteStruct(wallet)
	if err != nil {
		return utils.TranslateError(err)
	}

	os.RemoveAll(wallet.DataDir())

	switch wallet.GetAssetType() {
	case utils.BTCWalletAsset:
		delete(mgr.Assets.BTC.BadWallets, walletID)
	case utils.DCRWalletAsset:
		delete(mgr.Assets.DCR.BadWallets, walletID)
	}

	return nil
}

// RootDirFileSizeInBytes returns the total directory size of
// Assets Manager's root directory in bytes.
func (mgr *AssetsManager) RootDirFileSizeInBytes() (int64, error) {
	var size int64
	err := filepath.Walk(mgr.params.RootDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func (mgr *AssetsManager) WalletWithSeed(walletType utils.AssetType, seedMnemonic string) (int, error) {
	switch walletType {
	case utils.BTCWalletAsset:
		return mgr.BTCWalletWithSeed(seedMnemonic)
	case utils.DCRWalletAsset:
		return mgr.DCRWalletWithSeed(seedMnemonic)
	default:
		return -1, utils.ErrAssetUnknown
	}
}

func (mgr *AssetsManager) RestoreWallet(walletType utils.AssetType, walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	switch walletType {
	case utils.BTCWalletAsset:
		return mgr.RestoreBTCWallet(walletName, seedMnemonic, privatePassphrase, privatePassphraseType)
	case utils.DCRWalletAsset:
		return mgr.RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase, privatePassphraseType)
	default:
		return nil, utils.ErrAssetUnknown
	}
}

func (mgr *AssetsManager) WalletWithXPub(walletType utils.AssetType, xPub string) (int, error) {
	switch walletType {
	case utils.DCRWalletAsset:
		return mgr.DCRWalletWithXPub(xPub)
	case utils.BTCWalletAsset:
		return mgr.BTCWalletWithXPub(xPub)
	default:
		return -1, utils.ErrAssetUnknown
	}
}
