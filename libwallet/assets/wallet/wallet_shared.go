package wallet

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/walletseed"
	"github.com/asdine/storm"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet/walletdata"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

type Wallet struct {
	ID        int       `storm:"id,increment"`
	Name      string    `storm:"unique"`
	CreatedAt time.Time `storm:"index"`
	Type      utils.AssetType
	dbDriver  string
	rootDir   string
	db        *storm.DB

	EncryptedSeed         []byte
	IsRestored            bool
	HasDiscoveredAccounts bool
	PrivatePassphraseType int32

	netType      utils.NetworkType
	chainsParams *utils.ChainsParams

	loader       loader.AssetLoader
	walletDataDB *walletdata.DB

	// networkCancel function set to safely shutdown sync if in progress
	// before a task that would be affected by syncing is run i.e. Deleting
	// a wallet.
	networkCancel func()

	shuttingDown chan bool
	cancelFuncs  []context.CancelFunc

	// // setUserConfigValue saves the provided key-value pair to a config database.
	// // This function is ideally assigned when the `wallet.prepare` method is
	// // called from a MultiWallet instance.
	// setUserConfigValue configSaveFn

	// // readUserConfigValue returns the previously saved value for the provided
	// // key from a config database. Returns nil if the key wasn't previously set.
	// // This function is ideally assigned when the `wallet.prepare` method is
	// // called from a MultiWallet instance.
	// readUserConfigValue configReadFn

	mu sync.RWMutex
}

// prepare gets a wallet ready for use by opening the transactions index database
// and initializing the wallet loader which can be used subsequently to create,
// load and unload the wallet.
func (wallet *Wallet) Prepare(rootDir string, db *storm.DB, netType utils.NetworkType, loader loader.AssetLoader) (err error) {
	wallet.mu.Lock()
	defer wallet.mu.Unlock()

	wallet.db = db
	wallet.loader = loader
	wallet.netType = netType
	wallet.rootDir = rootDir
	return wallet.prepare()
}

// prepare is used initialize the assets common setup configuration.
// Should be called by every method that exports the shared wallet implementation.
func (wallet *Wallet) prepare() (err error) {
	// wallet.setUserConfigValue = wallet.walletConfigSetFn()
	// wallet.readUserConfigValue = wallet.walletConfigReadFn()

	// Confirms if the correct wallet type and network types were set and passed.
	// Wallet type should be preset by the caller otherwise an error is returned.
	wallet.chainsParams, err = utils.GetChainParams(wallet.Type, wallet.netType)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	// open database for indexing transactions for faster loading
	walletDataDBPath := filepath.Join(wallet.dataDir(), walletdata.DbName)
	oldTxDBPath := filepath.Join(wallet.dataDir(), walletdata.OldDbName)
	if exists, _ := fileExists(oldTxDBPath); exists {
		moveFile(oldTxDBPath, walletDataDBPath)
	}

	// Initialize the walletDataDb
	walletDb, err := walletdata.Initialize(walletDataDBPath, &Transaction{})
	if err != nil {
		log.Error(err.Error())
		return err
	}

	// Set ticket maturity and expiry if they are supported by the current asset.
	// By this point the wallet chains parameters have been resolved.
	switch wallet.Type {
	case utils.DCRWalletAsset:
		walletDb.SetTicketMaturity(int32(wallet.chainsParams.DCR.TicketMaturity)).
			SetTicketExpiry(int32(wallet.chainsParams.DCR.TicketExpiry))
	}

	wallet.walletDataDB = walletDb

	if wallet.networkCancel == nil {
		wallet.networkCancel = func() {
			log.Warnf("Network cancel callback missing")
		}
	}

	// init cancelFuncs slice to hold cancel functions for long running
	// operations and start go routine to listen for shutdown signal
	wallet.cancelFuncs = make([]context.CancelFunc, 0)
	wallet.shuttingDown = make(chan bool)
	go func() {
		<-wallet.shuttingDown
		for _, cancel := range wallet.cancelFuncs {
			cancel()
		}
	}()

	return nil
}

func (wallet *Wallet) Shutdown() {
	// Trigger shuttingDown signal to cancel all contexts created with
	// `wallet.ShutdownContext()` or `wallet.shutdownContextWithCancel()`.
	wallet.shuttingDown <- true

	if _, loaded := wallet.loader.GetLoadedWallet(); loaded {
		err := wallet.loader.UnloadWallet()
		if err != nil {
			log.Errorf("Failed to close wallet: %v", err)
		} else {
			log.Info("Closed wallet")
		}
	}

	if wallet.walletDataDB != nil {
		err := wallet.walletDataDB.Close()
		if err != nil {
			log.Errorf("tx db closed with error: %v", err)
		} else {
			log.Info("tx db closed successfully")
		}
	}
}

// WalletCreationTimeInMillis returns the wallet creation time for new
// wallets. Restored wallets would return an error.
func (wallet *Wallet) WalletCreationTimeInMillis() (int64, error) {
	if wallet.IsRestored {
		return 0, errors.New(utils.ErrWalletIsRestored)
	}
	return wallet.CreatedAt.UnixNano() / int64(time.Millisecond), nil
}

// DataDir returns the current wallet bucket. It is exported via the interface
// thus the need to be thread safe.
func (wallet *Wallet) DataDir() string {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.dataDir()
}

func (wallet *Wallet) dataDir() string {
	return filepath.Join(wallet.rootDir, string(wallet.Type), strconv.Itoa(wallet.ID))
}

// NetType returns the current network type. It is exported via the interface thus the
// the need to thread safe.
func (wallet *Wallet) NetType() utils.NetworkType {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.netType
}

// WalletType returns the current wallet's asset type. It is exported via the
// interface thus the the need to be thread safe.
func (wallet *Wallet) WalletType() utils.AssetType {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.Type
}

// Internal return the upstream wallet of the current asset created in the loader
// package. Since its exported via the interface thus the need to be thread safe.
func (wallet *Wallet) Internal() *loader.LoaderWallets {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	lw, _ := wallet.loader.GetLoadedWallet()
	return lw
}

func (wallet *Wallet) SetNetworkCancelCallback(callback func()) {
	wallet.networkCancel = callback
}

// GetWalletDataDb returns the walletdatadb instance. Its not exported via the
// but nonetheless has been made thread safe.
func (wallet *Wallet) GetWalletDataDb() *walletdata.DB {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.walletDataDB
}

func (wallet *Wallet) WalletExists() (bool, error) {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.loader.WalletExists(strconv.Itoa(wallet.ID))
}

func CreateNewWallet(walletName, privatePassphrase string, privatePassphraseType int32, db *storm.DB, rootDir, dbDriver string,
	assetType utils.AssetType, net utils.NetworkType, loader loader.AssetLoader) (*Wallet, error) {
	seed, err := generateSeed(assetType)
	if err != nil {
		return nil, err
	}

	encryptedSeed, err := encryptWalletSeed([]byte(privatePassphrase), seed)
	if err != nil {
		return nil, err
	}

	wallet := &Wallet{
		Name:          walletName,
		db:            db,
		dbDriver:      dbDriver,
		rootDir:       rootDir,
		CreatedAt:     time.Now(),
		EncryptedSeed: encryptedSeed,

		PrivatePassphraseType: privatePassphraseType,
		HasDiscoveredAccounts: true,
		Type:                  assetType,
		loader:                loader,
		netType:               net,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare()
		if err != nil {
			return err
		}
		return wallet.CreateWallet(privatePassphrase, seed)
	})
}

func (wallet *Wallet) CreateWallet(privatePassphrase, seedMnemonic string) error {
	log.Info("Creating Wallet")
	if len(seedMnemonic) == 0 {
		return errors.New(utils.ErrEmptySeed)
	}

	seed, err := walletseed.DecodeUserInput(seedMnemonic)
	if err != nil {
		log.Error(err)
		return err
	}

	params := &loader.CreateWalletParams{
		WalletID:       strconv.Itoa(wallet.ID),
		PubPassphrase:  []byte(w.InsecurePubPassphrase),
		PrivPassphrase: []byte(privatePassphrase),
		Seed:           seed,
	}

	ctx, _ := wallet.ShutdownContextWithCancel()
	_, err = wallet.loader.CreateNewWallet(ctx, params)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("Created Wallet")
	return nil
}

func CreateWatchOnlyWallet(walletName, extendedPublicKey string, db *storm.DB, rootDir, dbDriver string,
	assetType utils.AssetType, net utils.NetworkType, loader loader.AssetLoader) (*Wallet, error) {
	wallet := &Wallet{
		Name:     walletName,
		db:       db,
		dbDriver: dbDriver,
		rootDir:  rootDir,

		IsRestored:            true,
		HasDiscoveredAccounts: true,
		Type:                  assetType,
		loader:                loader,
		netType:               net,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare()
		if err != nil {
			return err
		}
		return wallet.createWatchingOnlyWallet(extendedPublicKey)
	})
}

func (wallet *Wallet) createWatchingOnlyWallet(extendedPublicKey string) error {
	params := &loader.WatchOnlyWalletParams{
		WalletID:       strconv.Itoa(wallet.ID),
		PubPassphrase:  []byte(w.InsecurePubPassphrase),
		ExtendedPubKey: extendedPublicKey,
	}

	ctx, _ := wallet.ShutdownContextWithCancel()
	_, err := wallet.loader.CreateWatchingOnlyWallet(ctx, params)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("Created Watching Only Wallet")
	return nil
}

func RestoreWallet(walletName, seedMnemonic, rootDir, dbDriver string, db *storm.DB, privatePassphrase string,
	privatePassphraseType int32, assetType utils.AssetType, net utils.NetworkType, loader loader.AssetLoader) (*Wallet, error) {
	wallet := &Wallet{
		Name:                  walletName,
		PrivatePassphraseType: privatePassphraseType,
		db:                    db,
		dbDriver:              dbDriver,
		rootDir:               rootDir,

		IsRestored:            true,
		HasDiscoveredAccounts: false,
		Type:                  assetType,
		loader:                loader,
		netType:               net,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare()
		if err != nil {
			return err
		}
		return wallet.CreateWallet(privatePassphrase, seedMnemonic)
	})
}

func (wallet *Wallet) WalletNameExists(walletName string) (bool, error) {
	if strings.HasPrefix(walletName, reservedWalletPrefix) {
		return false, errors.E(utils.ErrReservedWalletName)
	}

	err := wallet.db.One("Name", walletName, &Wallet{})
	if err == nil {
		return true, nil
	} else if err != storm.ErrNotFound {
		return false, err
	}

	return false, nil
}

func (wallet *Wallet) RenameWallet(newName string) error {
	if exists, err := wallet.WalletNameExists(newName); err != nil {
		return utils.TranslateError(err)
	} else if exists {
		return errors.New(utils.ErrExist)
	}

	wallet.Name = newName
	return wallet.db.Save(wallet) // update WalletName field
}

func (wallet *Wallet) DeleteWallet(privPass []byte) error {
	// functions to safely cancel sync before proceeding
	wallet.networkCancel()

	err := wallet.deleteWallet(privPass)
	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

func (wallet *Wallet) saveNewWallet(setupWallet func() error) (*Wallet, error) {
	exists, err := wallet.WalletNameExists(wallet.Name)
	if err != nil {
		return nil, utils.TranslateError(err)
	} else if exists {
		return nil, errors.New(utils.ErrExist)
	}

	// safely cancel sync before proceeding
	wallet.networkCancel()

	// Perform database save operations in batch transaction
	// for automatic rollback if error occurs at any point.
	err = wallet.batchDbTransaction(func(db storm.Node) error {
		// saving struct to update ID property with an auto-generated value
		err := db.Save(wallet)
		if err != nil {
			return err
		}

		walletDataDir := wallet.dataDir()
		dirExists, err := fileExists(walletDataDir)
		if err != nil {
			return err
		} else if dirExists {
			newDirName, err := backupFile(walletDataDir, 1)
			if err != nil {
				return err
			}
			log.Infof("Undocumented file at %s moved to %s", walletDataDir, newDirName)
		}

		os.MkdirAll(walletDataDir, os.ModePerm) // create wallet dir

		if wallet.Name == "" {
			wallet.Name = reservedWalletPrefix + strconv.Itoa(wallet.ID) // wallet-#
		}

		err = db.Save(wallet) // update database with complete wallet information
		if err != nil {
			return err
		}
		return setupWallet()
	})

	if err != nil {
		return nil, utils.TranslateError(err)
	}

	return wallet, nil
}

// func (wallet *Wallet) LinkExistingWallet(walletName, walletDataDir, originalPubPass string, privatePassphraseType int32) (*Wallet, error) {
// 	// check if `walletDataDir` contains wallet.db
// 	if !WalletExistsAt(walletDataDir) {
// 		return nil, errors.New(utils.ErrNotExist)
// 	}

// 	ctx, _ := wallet.ContextWithShutdownCancel()

// 	// verify the public passphrase for the wallet being linked before proceeding
// 	if err := wallet.loadWalletTemporarily(ctx, walletDataDir, originalPubPass, nil); err != nil {
// 		return nil, err
// 	}

// 	wal := &Wallet{
// 		Name:                  walletName,
// 		PrivatePassphraseType: privatePassphraseType,
// 		IsRestored:            true,
// 		HasDiscoveredAccounts: false, // assume that account discovery hasn't been done
// 		Type:                  DCRWallet,
// 	}

// 	return wallet.saveNewWallet(func() error {
// 		// move wallet.db and tx.db files to newly created dir for the wallet
// 		currentWalletDbFilePath := filepath.Join(walletDataDir, walletDbName)
// 		newWalletDbFilePath := filepath.Join(wal.DataDir(), walletDbName)
// 		if err := moveFile(currentWalletDbFilePath, newWalletDbFilePath); err != nil {
// 			return err
// 		}

// 		currentTxDbFilePath := filepath.Join(walletDataDir, walletdata.OldDbName)
// 		newTxDbFilePath := filepath.Join(wallet.DataDir(), walletdata.DbName)
// 		if err := moveFile(currentTxDbFilePath, newTxDbFilePath); err != nil {
// 			return err
// 		}

// 		// prepare the wallet for use and open it
// 		err := (func() error {
// 			err := wallet.prepare()
// 			if err != nil {
// 				return err
// 			}

// 			if originalPubPass == "" || originalPubPass == w.InsecurePubPassphrase {
// 				return wallet.OpenWallet()
// 			}

// 			err = wallet.loadWalletTemporarily(ctx, wallet.DataDir(), originalPubPass, func(tempWallet *w.Wallet) error {
// 				return tempWallet.ChangePublicPassphrase(ctx, []byte(originalPubPass), []byte(w.InsecurePubPassphrase))
// 			})
// 			if err != nil {
// 				return err
// 			}

// 			return wallet.OpenWallet()
// 		})()

// 		// restore db files to their original location if there was an error
// 		// in the wallet setup process above
// 		if err != nil {
// 			moveFile(newWalletDbFilePath, currentWalletDbFilePath)
// 			moveFile(newTxDbFilePath, currentTxDbFilePath)
// 		}

// 		return err
// 	})
// }

func (wallet *Wallet) IsWatchingOnlyWallet() bool {
	if w, ok := wallet.loader.GetLoadedWallet(); ok {
		switch wallet.Type {
		case utils.DCRWalletAsset:
			return w.DCR.WatchingOnly()
		case utils.BTCWalletAsset:
			return w.BTC.Manager.WatchOnly()
		}
	}

	return false
}

func (wallet *Wallet) OpenWallet() error {
	// load all the necessary configurations and valid the preset asset type
	// and the net type.
	if err := wallet.prepare(); err != nil {
		return err
	}

	pubPass := []byte(w.InsecurePubPassphrase)

	ctx, _ := wallet.ShutdownContextWithCancel()
	_, err := wallet.loader.OpenExistingWallet(ctx, strconv.Itoa(wallet.ID), pubPass)
	if err != nil {
		log.Error(err)
		return utils.TranslateError(err)
	}

	return nil
}

func (wallet *Wallet) WalletOpened() bool {
	switch wallet.Type {
	case utils.BTCWalletAsset:
		return wallet.Internal().BTC != nil
	case utils.DCRWalletAsset:
		return wallet.Internal().DCR != nil
	default:
		return false
	}
}

func (wallet *Wallet) UnlockWallet(privPass []byte) (err error) {
	loadedWallet, ok := wallet.loader.GetLoadedWallet()
	if !ok {
		return errors.New(utils.ErrWalletNotLoaded)
	}

	switch wallet.Type {
	case utils.BTCWalletAsset:
		err = loadedWallet.BTC.Unlock(privPass, nil)
	case utils.DCRWalletAsset:
		ctx, _ := wallet.ShutdownContextWithCancel()
		err = loadedWallet.DCR.Unlock(ctx, privPass, nil)
	}

	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

func (wallet *Wallet) LockWallet() {
	// Attempt to safely shutdown network sync before proceeding.
	wallet.networkCancel()

	if !wallet.IsLocked() {
		switch wallet.Type {
		case utils.BTCWalletAsset:
			wallet.Internal().BTC.Lock()
		case utils.DCRWalletAsset:
			wallet.Internal().DCR.Lock()
		}
	}
}

func (wallet *Wallet) IsLocked() bool {
	switch wallet.Type {
	case utils.BTCWalletAsset:
		return wallet.Internal().BTC.Locked()
	case utils.DCRWalletAsset:
		return wallet.Internal().DCR.Locked()
	default:
		return false
	}
}

// ChangePrivatePassphraseForWallet attempts to change the wallet's passphrase and re-encrypts the seed with the new passphrase.
func (wallet *Wallet) ChangePrivatePassphraseForWallet(oldPrivatePassphrase, newPrivatePassphrase []byte, privatePassphraseType int32) error {
	if privatePassphraseType != PassphraseTypePin && privatePassphraseType != PassphraseTypePass {
		return errors.New(utils.ErrInvalid)
	}
	encryptedSeed := wallet.EncryptedSeed
	if encryptedSeed != nil {
		decryptedSeed, err := decryptWalletSeed(oldPrivatePassphrase, encryptedSeed)
		if err != nil {
			return err
		}

		encryptedSeed, err = encryptWalletSeed(newPrivatePassphrase, decryptedSeed)
		if err != nil {
			return err
		}
	}

	err := wallet.changePrivatePassphrase(oldPrivatePassphrase, newPrivatePassphrase)
	if err != nil {
		return utils.TranslateError(err)
	}

	wallet.EncryptedSeed = encryptedSeed
	wallet.PrivatePassphraseType = privatePassphraseType
	err = wallet.db.Save(wallet)
	if err != nil {
		log.Errorf("error saving wallet-[%d] to database after passphrase change: %v", wallet.ID, err)

		err2 := wallet.changePrivatePassphrase(newPrivatePassphrase, oldPrivatePassphrase)
		if err2 != nil {
			log.Errorf("error undoing wallet passphrase change: %v", err2)
			log.Errorf("error wallet passphrase was changed but passphrase type and newly encrypted seed could not be saved: %v", err)
			return errors.New(utils.ErrSavingWallet)
		}

		return errors.New(utils.ErrChangingPassphrase)
	}

	return nil
}

func (wallet *Wallet) changePrivatePassphrase(oldPass []byte, newPass []byte) (err error) {
	defer func() {
		for i := range oldPass {
			oldPass[i] = 0
		}

		for i := range newPass {
			newPass[i] = 0
		}
	}()

	switch wallet.Type {
	case utils.BTCWalletAsset:
		err = wallet.Internal().BTC.ChangePrivatePassphrase(oldPass, newPass)
	case utils.DCRWalletAsset:
		ctx, _ := wallet.ShutdownContextWithCancel()
		err = wallet.Internal().DCR.ChangePrivatePassphrase(ctx, oldPass, newPass)
	}
	if err != nil {
		return utils.TranslateError(err)
	}
	return nil
}

func (wallet *Wallet) deleteWallet(privatePassphrase []byte) error {
	defer func() {
		for i := range privatePassphrase {
			privatePassphrase[i] = 0
		}
	}()

	if !wallet.IsWatchingOnlyWallet() {
		err := wallet.UnlockWallet(privatePassphrase)
		if err != nil {
			return err
		}
		wallet.LockWallet()
	}

	wallet.Shutdown()

	err := wallet.db.DeleteStruct(wallet)
	if err != nil {
		return utils.TranslateError(err)
	}

	log.Info("Deleting Wallet")
	return os.RemoveAll(wallet.dataDir())
}
