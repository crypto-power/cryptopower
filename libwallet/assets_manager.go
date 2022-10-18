package libwallet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"gitlab.com/raedah/cryptopower/libwallet/ext"
	"gitlab.com/raedah/cryptopower/libwallet/internal/politeia"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	bolt "go.etcd.io/bbolt"

	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
)

type Assets struct {
	DCR struct {
		Wallets    map[int]*dcr.DCRAsset
		BadWallets map[int]*sharedW.Wallet
	}
	BTC struct {
		Wallets    map[int]*btc.BTCAsset
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

	dexClient       *DexClient
	Politeia        *politeia.Politeia
	ExternalService *ext.Service
}

func NewAssetsManager(rootDir, dbDriver, net, politeiaHost string) (*AssetsManager, error) {
	errors.Separator = ":: "

	netType := utils.NetworkType(net)
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

	rootDir = filepath.Join(rootDir, net)
	if err = os.MkdirAll(rootDir, os.ModePerm); err != nil {
		return nil, errors.Errorf("failed to create rootDir: %v", err)
	}

	if err = initLogRotator(filepath.Join(rootDir, logFileName)); err != nil {
		return nil, errors.Errorf("failed to init logRotator: %v", err.Error())
	}

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

	params := &sharedW.InitParams{
		DbDriver: dbDriver,
		RootDir:  rootDir,
		DB:       mwDB,
		NetType:  netType,
	}

	mgr := &AssetsManager{
		params:   params,
		Politeia: politeia,
		Assets:   new(Assets),
	}

	mgr.Assets.BTC.Wallets = make(map[int]*btc.BTCAsset)
	mgr.Assets.DCR.Wallets = make(map[int]*dcr.DCRAsset)

	mgr.Assets.BTC.BadWallets = make(map[int]*sharedW.Wallet)
	mgr.Assets.DCR.BadWallets = make(map[int]*sharedW.Wallet)

	mgr.chainsParams.DCR = dcrChainParams
	mgr.chainsParams.BTC = btcChainParams

	// initialize the ExternalService. ExternalService provides multiwallet with
	// the functionalities to retrieve data from 3rd party services. e.g Binance, Bittrex.
	mgr.ExternalService = ext.NewService(dcrChainParams)

	// read saved dcr wallets info from db and initialize wallets
	query := mgr.params.DB.Select(q.True()).OrderBy("ID")
	var wallets []*sharedW.Wallet
	err = query.Find(&wallets)
	if err != nil && err != storm.ErrNotFound {
		return nil, err
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
		switch wallet.Type {
		case utils.BTCWalletAsset:
			w, err := btc.LoadExisting(wallet, mgr.params)
			if err == nil && !isOK(w) {
				err = fmt.Errorf("missing wallet database file: %v", wallet.DataDir())
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
				err = fmt.Errorf("missing wallet database file: %v", wallet.DataDir())
				log.Debug(err)
			}
			if err != nil {
				mgr.Assets.DCR.BadWallets[wallet.ID] = wallet
				log.Warnf("Ignored dcr wallet load error for wallet %d (%s)", wallet.ID, wallet.Name)
			} else {
				mgr.Assets.DCR.Wallets[wallet.ID] = w
			}
		}
	}

	mgr.listenForShutdown()

	log.Infof("Loaded %d wallets", mgr.LoadedWalletsCount())

	// Attempt to set the log levels if a valid db interface was found.
	if mgr.db != nil {
		mgr.SetLogLevels()
	}

	if err = mgr.initDexClient(); err != nil {
		log.Errorf("DEX client set up error: %v", err)
	}

	return mgr, nil
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
		wallet.CancelSync()
		wallet.Shutdown()
	}

	for _, wallet := range mgr.Assets.BTC.Wallets {
		wallet.Shutdown()
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

func (mgr *AssetsManager) OpenWallets(startupPassphrase []byte) error {
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

func (mgr *AssetsManager) AllDCRWallets() (wallets []*dcr.DCRAsset) {
	for _, wallet := range mgr.Assets.DCR.Wallets {
		wallets = append(wallets, wallet)
	}
	return wallets
}

func (mgr *AssetsManager) AllBTCWallets() (wallets []*btc.BTCAsset) {
	for _, wallet := range mgr.Assets.BTC.Wallets {
		wallets = append(wallets, wallet)
	}
	return wallets
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
