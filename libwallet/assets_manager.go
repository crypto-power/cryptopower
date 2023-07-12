package libwallet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"decred.org/dcrwallet/v3/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/libwallet/internal/politeia"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	bolt "go.etcd.io/bbolt"

	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/assets/ltc"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
)

// Assets is a struct that holds all the assets supported by the wallet.
type Assets struct {
	DCR struct {
		Wallets    map[int]sharedW.Asset
		BadWallets map[int]*sharedW.Wallet
	}
	BTC struct {
		Wallets    map[int]sharedW.Asset
		BadWallets map[int]*sharedW.Wallet
	}
	LTC struct {
		Wallets    map[int]sharedW.Asset
		BadWallets map[int]*sharedW.Wallet
	}
}

// AssetsManager is a struct that holds all the necessary parameters
// to manage the assets supported by the wallet.
type AssetsManager struct {
	params *sharedW.InitParams
	Assets *Assets

	db sharedW.AssetsManagerDB // Interface to manage db access at the ASM.

	shuttingDown chan bool
	cancelFuncs  []context.CancelFunc
	chainsParams utils.ChainsParams

	Politeia        *politeia.Politeia
	InstantSwap     *instantswap.InstantSwap
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

	ltcChainParams, err := initializeLTCWalletParameters(netType)
	if err != nil {
		log.Errorf("error initializing LTC parameters: %s", err.Error())
		return nil, errors.Errorf("error initializing LTC parameters: %s", err.Error())
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
	mgr.Assets.LTC.Wallets = make(map[int]sharedW.Asset)

	mgr.Assets.BTC.BadWallets = make(map[int]*sharedW.Wallet)
	mgr.Assets.DCR.BadWallets = make(map[int]*sharedW.Wallet)
	mgr.Assets.LTC.BadWallets = make(map[int]*sharedW.Wallet)

	mgr.chainsParams.DCR = dcrChainParams
	mgr.chainsParams.BTC = btcChainParams
	mgr.chainsParams.LTC = ltcChainParams
	return mgr, nil
}

// NewAssetsManager creates a new AssetsManager instance.
func NewAssetsManager(rootDir, dbDriver, politeiaHost, logDir string, netType utils.NetworkType) (*AssetsManager, error) {
	errors.Separator = ":: "

	// Create a root dir that has the path up the network folder.
	rootDir = filepath.Join(rootDir, string(netType))
	if err := os.MkdirAll(rootDir, utils.UserFilePerm); err != nil {
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

	instantSwap, err := instantswap.NewInstantSwap(mwDB)
	if err != nil {
		return nil, err
	}

	mgr.params.DB = mwDB
	mgr.Politeia = politeia
	mgr.InstantSwap = instantSwap

	// initialize the ExternalService. ExternalService provides assetsManager with
	// the functionalities to retrieve data from 3rd party services. e.g Binance, Bittrex.
	mgr.ExternalService = ext.NewService(mgr.chainsParams.DCR)

	// clean all deleted wallet if exist
	mgr.cleanDeletedWallets()

	// Load existing wallets.
	if err := mgr.prepareExistingWallets(); err != nil {
		return nil, err
	}

	log.Infof("Loaded %d wallets", mgr.LoadedWalletsCount())

	// Attempt to set the log levels if a valid db interface was found.
	if mgr.IsAssetManagerDB() {
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
		// preset the network type so as to generate correct folder path
		wallet.SetNetType(mgr.NetType())

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

		case utils.LTCWalletAsset:
			w, err := ltc.LoadExisting(wallet, mgr.params)
			if err == nil && !isOK(w) {
				err = fmt.Errorf("missing wallet database file: %v", path)
				log.Debug(err)
			}
			if err != nil {
				mgr.Assets.LTC.BadWallets[wallet.ID] = wallet
				log.Warnf("Ignored ltc wallet load error for wallet %d (%s)", wallet.ID, wallet.Name)
			} else {
				mgr.Assets.LTC.Wallets[wallet.ID] = w
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

// Shutdown shuts down the assets manager and all its wallets.
func (mgr *AssetsManager) Shutdown() {
	log.Info("Shutting down libwallet")

	// Trigger shuttingDown signal to cancel all contexts created with `shutdownContextWithCancel`.
	mgr.shuttingDown <- true

	// Shutdown politeia if its syncing
	if mgr.Politeia.IsSyncing() {
		mgr.Politeia.StopSync()
	}

	// Shutdown instant swap if its syncing
	if mgr.InstantSwap.IsSyncing() {
		mgr.InstantSwap.StopSync()
	}

	for _, wallet := range mgr.AllWallets() {
		wallet.Shutdown() // Cancels the wallet sync too.
		wallet.CancelRescan()
	}

	// Disable all active network connections
	utils.ShutdownHTTPClients()

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

// NetType returns the network type of the assets manager.
// It is either mainnet or testnet.
func (mgr *AssetsManager) NetType() utils.NetworkType {
	return mgr.params.NetType
}

// LogDir returns the log directory of the assets manager.
func (mgr *AssetsManager) LogDir() string {
	return filepath.Join(mgr.params.RootDir, logFileName)
}

// IsAssetManagerDB returns true if the asset manager db interface was extracted
// from one of the loaded valid wallets. Assets Manager Db interface exists in
// all wallets by default. If no valid asset manager db interface exists,
// there is no valid wallet loaded yet; - they maybe no wallets at all to load.
func (mgr *AssetsManager) IsAssetManagerDB() bool {
	return mgr.db != nil
}

// OpenWallets opens all wallets in the assets manager.
func (mgr *AssetsManager) OpenWallets(startupPassphrase string) error {
	for _, wallet := range mgr.AllWallets() {
		if wallet.IsSyncing() {
			return errors.New(utils.ErrSyncAlreadyInProgress)
		}
	}

	if err := mgr.VerifyStartupPassphrase(startupPassphrase); err != nil {
		return err
	}

	for _, wallet := range mgr.AllWallets() {
		select {
		case <-mgr.shuttingDown:
			// If shutdown protocol is detected, exit immediately.
			return nil
		default:
			err := wallet.OpenWallet()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DCRBadWallets returns a map of all bad DCR wallets.
func (mgr *AssetsManager) DCRBadWallets() map[int]*sharedW.Wallet {
	return mgr.Assets.DCR.BadWallets
}

// BTCBadWallets returns a map of all bad BTC wallets.
func (mgr *AssetsManager) BTCBadWallets() map[int]*sharedW.Wallet {
	return mgr.Assets.BTC.BadWallets
}

// LTCBadWallets returns a map of all bad LTC wallets.
func (mgr *AssetsManager) LTCBadWallets() map[int]*sharedW.Wallet {
	return mgr.Assets.LTC.BadWallets
}

// LoadedWalletsCount returns the number of wallets loaded in the assets manager.
func (mgr *AssetsManager) LoadedWalletsCount() int32 {
	return int32(len(mgr.AllWallets()))
}

// OpenedWalletsCount returns the number of wallets opened in the assets manager.
func (mgr *AssetsManager) OpenedWalletsCount() int32 {
	var count int32
	for _, wallet := range mgr.AllWallets() {
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

// sortWallets returns the watchonly wallets ordered last.
func (mgr *AssetsManager) sortWallets(assetType utils.AssetType) []sharedW.Asset {
	normalWallets := make([]sharedW.Asset, 0)
	watchOnlyWallets := make([]sharedW.Asset, 0)

	var unsortedWallets map[int]sharedW.Asset
	switch assetType {
	case utils.DCRWalletAsset:
		unsortedWallets = mgr.Assets.DCR.Wallets
	case utils.BTCWalletAsset:
		unsortedWallets = mgr.Assets.BTC.Wallets
	case utils.LTCWalletAsset:
		unsortedWallets = mgr.Assets.LTC.Wallets
	}

	for _, wallet := range unsortedWallets {
		if wallet.IsWatchingOnlyWallet() {
			watchOnlyWallets = append(watchOnlyWallets, wallet)
		} else {
			normalWallets = append(normalWallets, wallet)
		}
	}
	return append(normalWallets, watchOnlyWallets...)
}

// AllDCRWallets returns all DCR wallets in the assets manager.
func (mgr *AssetsManager) AllDCRWallets() (wallets []sharedW.Asset) {
	return mgr.sortWallets(utils.DCRWalletAsset)
}

// AllBTCWallets returns all BTC wallets in the assets manager.
func (mgr *AssetsManager) AllBTCWallets() (wallets []sharedW.Asset) {
	return mgr.sortWallets(utils.BTCWalletAsset)
}

// AllLTCWallets returns all LTC wallets in the assets manager.
func (mgr *AssetsManager) AllLTCWallets() (wallets []sharedW.Asset) {
	return mgr.sortWallets(utils.LTCWalletAsset)
}

// AllWallets returns all wallets in the assets manager.
func (mgr *AssetsManager) AllWallets() (wallets []sharedW.Asset) {
	wallets = mgr.AllDCRWallets()
	wallets = append(wallets, mgr.AllBTCWallets()...)
	wallets = append(wallets, mgr.AllLTCWallets()...)
	return wallets
}

// DeleteWallet deletes a wallet from the assets manager.
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
	case utils.LTCWalletAsset:
		delete(mgr.Assets.LTC.Wallets, walletID)
	}

	return nil
}

// WalletWithID returns a wallet with the given ID.
func (mgr *AssetsManager) WalletWithID(walletID int) sharedW.Asset {
	if wallet, ok := mgr.Assets.BTC.Wallets[walletID]; ok {
		return wallet
	}
	if wallet, ok := mgr.Assets.DCR.Wallets[walletID]; ok {
		return wallet
	}
	if wallet, ok := mgr.Assets.LTC.Wallets[walletID]; ok {
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
	if badWallet, ok := mgr.Assets.LTC.BadWallets[walletID]; ok {
		return badWallet
	}
	return nil
}

// DeleteBadWallet deletes a bad wallet from the assets manager.
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
	case utils.LTCWalletAsset:
		delete(mgr.Assets.LTC.BadWallets, walletID)
	}

	return nil
}

// RootDirFileSizeInBytes returns the total directory size of
// Assets Manager's root directory in bytes.
func (mgr *AssetsManager) RootDirFileSizeInBytes(dataDir string) (int64, error) {
	var size int64
	err := filepath.Walk(dataDir, func(_ string, info os.FileInfo, err error) error {
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

// WalletWithSeed returns the ID of the wallet with the given seed. If a wallet
// with the given seed does not exist, it returns -1.
func (mgr *AssetsManager) WalletWithSeed(walletType utils.AssetType, seedMnemonic string) (int, error) {
	switch walletType {
	case utils.BTCWalletAsset:
		return mgr.BTCWalletWithSeed(seedMnemonic)
	case utils.DCRWalletAsset:
		return mgr.DCRWalletWithSeed(seedMnemonic)
	case utils.LTCWalletAsset:
		return mgr.LTCWalletWithSeed(seedMnemonic)
	default:
		return -1, utils.ErrAssetUnknown
	}
}

// RestoreWallet restores a wallet from the given seed.
func (mgr *AssetsManager) RestoreWallet(walletType utils.AssetType, walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	switch walletType {
	case utils.BTCWalletAsset:
		return mgr.RestoreBTCWallet(walletName, seedMnemonic, privatePassphrase, privatePassphraseType)
	case utils.DCRWalletAsset:
		return mgr.RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase, privatePassphraseType)
	case utils.LTCWalletAsset:
		return mgr.RestoreLTCWallet(walletName, seedMnemonic, privatePassphrase, privatePassphraseType)
	default:
		return nil, utils.ErrAssetUnknown
	}
}

// WalletWithXPub returns the ID of the wallet with the given xpub. If a wallet
// with the given xpub does not exist, it returns -1.
func (mgr *AssetsManager) WalletWithXPub(walletType utils.AssetType, xPub string) (int, error) {
	switch walletType {
	case utils.DCRWalletAsset:
		return mgr.DCRWalletWithXPub(xPub)
	case utils.BTCWalletAsset:
		return mgr.BTCWalletWithXPub(xPub)
	case utils.LTCWalletAsset:
		return mgr.LTCWalletWithXPub(xPub)
	default:
		return -1, utils.ErrAssetUnknown
	}
}

// on windows os after a wallet is deleted, the dir of deleted wallet still exists,
// cleanDeletedWallets will check the data dir of all deleted wallets and remove them.
func (mgr *AssetsManager) cleanDeletedWallets() {
	// read all stored wallets info from the db and initialize wallets interfaces.
	query := mgr.params.DB.Select(q.True()).OrderBy("ID")
	var wallets []*sharedW.Wallet
	err := query.Find(&wallets)
	if err != nil && err != storm.ErrNotFound {
		log.Error("Fail to get all wallet to check deleted wallets")
		return
	}

	log.Info("Starting check and remove all dir of deleted wallets....")
	validWallets := make(map[string]bool, len(wallets))
	deletedWalletDirs := make([]string, 0)

	// filter all valid wallets
	for _, wallet := range wallets {
		key := wallet.Type.ToStringLower() + strconv.Itoa(wallet.ID)
		validWallets[key] = true
	}

	// filter all wallets to be deleted.
	for _, wType := range mgr.AllAssetTypes() {
		dirName := ""
		if mgr.NetType() == utils.Testnet {
			dirName = utils.NetDir(wType, utils.Testnet)
		}
		rootDir := filepath.Join(mgr.params.RootDir, dirName, wType.ToStringLower())
		files, err := os.ReadDir(rootDir)
		if err != nil {
			log.Errorf("can't read %s root wallet type: %v", wType, err)
			return
		}
		for _, f := range files {
			key := wType.ToStringLower() + f.Name()
			if f.IsDir() && !validWallets[key] {
				deletedWalletDirs = append(deletedWalletDirs, filepath.Join(rootDir, f.Name()))
			}
		}
	}

	if len(deletedWalletDirs) == 0 {
		log.Info("No wallets to clean were found")
		return
	}

	for _, v := range deletedWalletDirs {
		err = os.RemoveAll(v)
		if err != nil {
			log.Errorf("Can't remove the wallet with error: %v", err)
		}
	}

	log.Info("Clean all deleted wallets")
}

// AllAssetTypes returns all asset types supported by the assets manager.
func (mgr *AssetsManager) AllAssetTypes() []utils.AssetType {
	return []utils.AssetType{
		utils.DCRWalletAsset,
		utils.BTCWalletAsset,
		utils.LTCWalletAsset,
	}
}
