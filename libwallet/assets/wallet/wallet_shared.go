package wallet

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"decred.org/dcrwallet/v4/errors"
	w "decred.org/dcrwallet/v4/wallet"
	"github.com/asdine/storm"
	"github.com/crypto-power/cryptopower/libwallet/assets/wallet/walletdata"
	"github.com/crypto-power/cryptopower/libwallet/internal/loader"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

type Wallet struct {
	ID        int       `storm:"id,increment"`
	Name      string    `storm:"unique"`
	CreatedAt time.Time `storm:"index"`
	Type      utils.AssetType
	dbDriver  string
	rootDir   string
	db        *storm.DB
	logDir    string

	EncryptedMnemonic     []byte
	IsBackedUp            bool
	IsRestored            bool
	HasDiscoveredAccounts bool
	PrivatePassphraseType int32

	netType      utils.NetworkType
	chainsParams *utils.ChainsParams
	loader       loader.AssetLoader
	walletDataDB *walletdata.DB

	// Birthday holds the timestamp of the birthday block from where wallet
	// restoration begins from. CreatedAt is available for audit purposes
	// in relation to how long the wallet has been in existence.
	Birthday time.Time

	// networkCancel function set to safely shutdown sync if in progress
	// before a task that would be affected by syncing is run i.e. Deleting
	// a wallet.
	// NB: Use of this method results to complete network shutdown and restarting
	// it back is almost impossible.
	networkCancel func()

	shuttingDown chan bool
	cancelFuncs  []context.CancelFunc

	mu sync.RWMutex
}

// prepare gets a wallet ready for use by opening the transactions index database
// and initializing the wallet loader which can be used subsequently to create,
// load and unload the wallet.
func (wallet *Wallet) Prepare(loader loader.AssetLoader, params *InitParams) (err error) {
	wallet.mu.Lock()
	defer wallet.mu.Unlock()

	wallet.db = params.DB
	wallet.loader = loader
	wallet.netType = params.NetType
	wallet.rootDir = params.RootDir
	wallet.logDir = params.LogDir
	return wallet.prepare()
}

// prepare is used to initialize the assets common setup configuration.
// Should be called by every method that exports the shared wallet implementation.
// The following should always be pre-loaded before calling prepare();
// wallet.db = db
// wallet.loader = loader
// wallet.netType = netType
// wallet.rootDir = rootDir
// wallet.Type = assetType
// wallet.logDir = params.LogDir
func (wallet *Wallet) prepare() (err error) {
	// Confirms if the correct wallet type and network types were set and passed.
	// Wallet type should be preset by the caller otherwise an error is returned.
	wallet.chainsParams, err = utils.GetChainParams(wallet.Type, wallet.netType)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	if wallet.networkCancel == nil {
		wallet.networkCancel = func() {
			log.Warnf("Network cancel callback missing")
		}
	}

	// open database for indexing transactions for faster loading
	var dbName string
	switch wallet.Type {
	case utils.DCRWalletAsset:
		dbName = walletdata.DCRDbName
	case utils.BTCWalletAsset:
		dbName = walletdata.BTCDBName
	case utils.LTCWalletAsset:
		dbName = walletdata.LTCDBName
	}

	walletDataDBPath := filepath.Join(wallet.dataDir(), dbName)

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
	// `wallet.shutdownContextWithCancel()`.
	wallet.shuttingDown <- true

	// Explicitly stop all network connectivity activities.
	if wallet.networkCancel != nil {
		wallet.networkCancel()
	}

	// Sets the upstream wallet instance to nil.
	if _, loaded := wallet.loader.GetLoadedWallet(); loaded {
		err := wallet.loader.UnloadWallet()
		if err != nil {
			log.Errorf("Failed to close wallet: %v", err)
		}
	}

	// close db connection as the last shutdown protocol.
	if wallet.walletDataDB != nil {
		err := wallet.walletDataDB.Close()
		if err != nil {
			log.Errorf("tx db closed with error: %v", err)
		}
	}

	log.Infof("(%s) full network shutdown protocols completed.", wallet.Name)
}

func (wallet *Wallet) TargetTimePerBlockMinutes() float64 {
	if wallet.Type == utils.BTCWalletAsset {
		return wallet.chainsParams.BTC.TargetTimePerBlock.Minutes()
	}
	return wallet.chainsParams.DCR.TargetTimePerBlock.Minutes()
}

// WalletCreationTimeInMillis returns the wallet creation time for new
// wallets. Restored wallets would return an error.
func (wallet *Wallet) WalletCreationTimeInMillis() (int64, error) {
	if wallet.IsRestored {
		return 0, errors.New(utils.ErrWalletIsRestored)
	}
	return wallet.CreatedAt.UnixNano() / int64(time.Millisecond), nil
}

// DataDir returns the current wallet bucket directory. It is exported via the interface
// thus the need to be thread safe.
func (wallet *Wallet) DataDir() string {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.dataDir()
}

func (wallet *Wallet) dataDir() string {
	dirName := ""
	// testnet datadir takes a special structure to differentiate "testnet4" and "testnet3"
	// data directory.
	if wallet.netType == utils.Testnet {
		dirName = utils.NetDir(wallet.Type, wallet.netType)
	}
	return filepath.Join(wallet.rootDir, dirName, wallet.Type.ToStringLower(), strconv.Itoa(wallet.ID))
}

// RootDir returns the root of current wallet bucket. It is exported via the interface
// thus the need to be thread safe.
// RootD
func (wallet *Wallet) RootDir() string {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.rootDir
}

// SetNetType is used to set the net type if it doesn't exist. This method is
// used before the actual wallet is loaded otherwise once loaded the netType
// can't be altered. This method help create the full method with the unique
// path for the folder structure for the testnet data dirs.
func (wallet *Wallet) SetNetType(netType utils.NetworkType) {
	wallet.mu.Lock()
	defer wallet.mu.Unlock()
	if string(wallet.netType) == "" {
		wallet.netType = netType
	}
}

// NetType returns the current network type. It is exported via the interface thus the
// the need to thread safe.
func (wallet *Wallet) NetType() utils.NetworkType {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.netType
}

// GetAssetType returns the current wallet's asset type. It is exported via the
// interface thus the the need to be thread safe.
func (wallet *Wallet) GetAssetType() utils.AssetType {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.Type
}

// Internal return the upstream wallet of the current asset created in the loader
// package. Since its exported via the interface thus the need to be thread safe.
func (wallet *Wallet) Internal() *loader.LoadedWallets {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	lw, _ := wallet.loader.GetLoadedWallet()
	return lw
}

func (wallet *Wallet) IsWalletBackedUp() bool {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	if wallet.IsRestored || wallet.IsWatchingOnlyWallet() {
		return true
	}
	return wallet.IsBackedUp
}

func (wallet *Wallet) HasWalletSeed() bool {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return len(wallet.EncryptedMnemonic) > 0
}

func (wallet *Wallet) GetWalletID() int {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.ID
}

func (wallet *Wallet) GetWalletName() string {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.Name
}

func (wallet *Wallet) ContainsDiscoveredAccounts() bool {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	// Address discovery was previously not marked as completed for watch only wallets.
	// This caused the receive page to be inaccessible for watch only wallets even after
	// address discovery was completed.
	// This is a temporary fix to allow users to continue using their watch-only wallets
	// (i.e being able to see their receive address).
	// This fix would also incorrectly report watch-only wallets that have not finished address discovery as
	// having completed it.
	// Although this is not a critical issue, becuase until a watch only wallet has finished the syncing
	// process (including address discovery), the reveive page will not be accessible.
	if wallet.IsWatchingOnlyWallet() {
		return true
	}
	return wallet.HasDiscoveredAccounts
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

// GetBirthday returns the timestamp when the wallet was created or its keys were
// first used. This helps to check if a wallet requires auto rescan and recovery
// on wallet startup.
func (wallet *Wallet) GetBirthday() time.Time {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	return wallet.Birthday
}

// SetBirthday allows updating the birthday time to a more precise value that is
// verified by the network.
func (wallet *Wallet) SetBirthday(birthday time.Time) {
	if birthday.IsZero() {
		log.Error("updated birthday time cannot be zero")
		return
	}

	wallet.mu.Lock()
	wallet.Birthday = birthday
	// Triggers db update with the new birthday time.
	_ = wallet.db.Save(wallet)
	wallet.mu.Unlock()
}

func CreateNewWallet(pass *AuthInfo, loader loader.AssetLoader,
	params *InitParams, assetType utils.AssetType,
) (*Wallet, error) {
	if pass.WordSeedType.ToInt() == 0 {
		return nil, errors.New("please select word seed type")
	}

	mnemonic, err := generateMnemonic(pass.WordSeedType)
	if err != nil {
		return nil, err
	}

	encryptedMnemonic, err := encryptWalletMnemonic([]byte(pass.PrivatePass), mnemonic)
	if err != nil {
		return nil, err
	}

	wallet := &Wallet{
		Name:                  pass.Name,
		db:                    params.DB,
		dbDriver:              params.DbDriver,
		rootDir:               params.RootDir,
		logDir:                params.LogDir,
		CreatedAt:             time.Now(),
		EncryptedMnemonic:     encryptedMnemonic,
		PrivatePassphraseType: pass.PrivatePassType,
		HasDiscoveredAccounts: true,
		Type:                  assetType,
		loader:                loader,
		netType:               params.NetType,
	}

	if err := wallet.saveNewWallet(func() error {
		err := wallet.prepare()
		if err != nil {
			return err
		}
		return wallet.createWallet(pass.PrivatePass, mnemonic, pass.WordSeedType)
	}); err != nil {
		return nil, err
	}
	if params.NetType == utils.DEXTest {
		addr := "127.0.0.1"
		if params.DEXTestAddr != "" {
			addr = params.DEXTestAddr
		}
		wallet.SetStringConfigValueForKey(SpvPersistentPeerAddressesConfigKey, addr)
	}
	return wallet, nil
}

func (wallet *Wallet) createWallet(privatePassphrase, seedMnemonic string, wordSeedType WordSeedType) error {
	log.Info("Creating Wallet")
	if len(seedMnemonic) == 0 {
		return errors.New(utils.ErrEmptySeed)
	}

	seed, err := DecodeSeedMnemonic(seedMnemonic, wallet.Type, wordSeedType)
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

func CreateWatchOnlyWallet(walletName, extendedPublicKey string, loader loader.AssetLoader,
	params *InitParams, assetType utils.AssetType,
) (*Wallet, error) {
	wallet := &Wallet{
		Name:     walletName,
		db:       params.DB,
		dbDriver: params.DbDriver,
		rootDir:  params.RootDir,
		logDir:   params.LogDir,

		IsRestored: true,
		// Setting HasDiscoveredAccounts to false causes address recovery to be
		// triggered on startup if the first recovery was prematurely stopped.
		HasDiscoveredAccounts: false,
		Type:                  assetType,
		loader:                loader,
		netType:               params.NetType,
	}

	if err := wallet.saveNewWallet(func() error {
		err := wallet.prepare()
		if err != nil {
			return err
		}
		return wallet.createWatchingOnlyWallet(extendedPublicKey)
	}); err != nil {
		return nil, err
	}
	if params.NetType == utils.DEXTest {
		addr := "127.0.0.1"
		if params.DEXTestAddr != "" {
			addr = params.DEXTestAddr
		}
		wallet.SetStringConfigValueForKey(SpvPersistentPeerAddressesConfigKey, addr)
	}
	return wallet, nil
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

func RestoreWallet(seedMnemonic string, pass *AuthInfo, loader loader.AssetLoader,
	params *InitParams, assetType utils.AssetType,
) (*Wallet, error) {
	// Ensure the encrypted seeds are available before creating wallet so we can
	// return early.
	encryptedMnemonic, err := encryptWalletMnemonic([]byte(pass.PrivatePass), seedMnemonic)
	if err != nil {
		log.Errorf("wallet.createWallet: error encrypting wallet seed: %v", err)
		return nil, err
	}

	wallet := &Wallet{
		Name:                  pass.Name,
		PrivatePassphraseType: pass.PrivatePassType,
		db:                    params.DB,
		dbDriver:              params.DbDriver,
		rootDir:               params.RootDir,
		logDir:                params.LogDir,

		EncryptedMnemonic:     encryptedMnemonic,
		IsRestored:            true,
		HasDiscoveredAccounts: false,
		Type:                  assetType,
		loader:                loader,
		netType:               params.NetType,
	}

	if err := wallet.saveNewWallet(func() error {
		err := wallet.prepare()
		if err != nil {
			return err
		}
		return wallet.createWallet(pass.PrivatePass, seedMnemonic, pass.WordSeedType)
	}); err != nil {
		return nil, err
	}
	if params.NetType == utils.DEXTest {
		addr := "127.0.0.1"
		if params.DEXTestAddr != "" {
			addr = params.DEXTestAddr
		}
		wallet.SetStringConfigValueForKey(SpvPersistentPeerAddressesConfigKey, addr)
	}
	return wallet, nil
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

func (wallet *Wallet) DeleteWallet(privPass string) error {
	err := wallet.deleteWallet(privPass)
	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

// saveNewWallet completes setting up the wallet. Since sync can only be
// initiated after wallet setup is complete, no sync cancel is necessary here.
func (wallet *Wallet) saveNewWallet(setupWallet func() error) error {
	exists, err := wallet.WalletNameExists(wallet.Name)
	if err != nil {
		return utils.TranslateError(err)
	} else if exists {
		return errors.New(utils.ErrExist)
	}

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

		err = os.MkdirAll(walletDataDir, utils.UserFilePerm) // create wallet dir
		if err != nil {
			return err
		}

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
		return utils.TranslateError(err)
	}

	return nil
}

func (wallet *Wallet) IsWatchingOnlyWallet() bool {
	if w, ok := wallet.loader.GetLoadedWallet(); ok {
		switch wallet.Type {
		case utils.DCRWalletAsset:
			return w.DCR.WatchingOnly()
		case utils.BTCWalletAsset:
			return w.BTC.Manager.WatchOnly()
		case utils.LTCWalletAsset:
			return w.LTC.Manager.WatchOnly()
		}
	}

	return false
}

func (wallet *Wallet) OpenWallet() error {
	pubPass := []byte(w.InsecurePubPassphrase)
	ctx, _ := wallet.ShutdownContextWithCancel()
	_, err := wallet.loader.OpenExistingWallet(ctx, strconv.Itoa(wallet.ID), pubPass)
	if err != nil {
		log.Error(err)
		return utils.TranslateError(err)
	}

	return nil
}

// WalletOpened checks if the upstream loader instance of the asset wallet
// is loaded (i.e. open).
func (wallet *Wallet) WalletOpened() bool {
	switch wallet.Type {
	case utils.BTCWalletAsset:
		return wallet.Internal().BTC != nil
	case utils.DCRWalletAsset:
		return wallet.Internal().DCR != nil
	case utils.LTCWalletAsset:
		return wallet.Internal().LTC != nil
	default:
		return false
	}
}

func (wallet *Wallet) UnlockWallet(privPass string) (err error) {
	loadedWallet, ok := wallet.loader.GetLoadedWallet()
	if !ok {
		return errors.New(utils.ErrWalletNotLoaded)
	}

	switch wallet.Type {
	case utils.BTCWalletAsset:
		err = loadedWallet.BTC.Unlock([]byte(privPass), nil)
	case utils.DCRWalletAsset:
		ctx, _ := wallet.ShutdownContextWithCancel()
		err = loadedWallet.DCR.Unlock(ctx, []byte(privPass), nil)
	case utils.LTCWalletAsset:
		err = loadedWallet.LTC.Unlock([]byte(privPass), nil)
	}

	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

func (wallet *Wallet) LockWallet() {
	loadedWallet, ok := wallet.loader.GetLoadedWallet()
	if !ok {
		return
	}

	if !wallet.IsLocked() {
		switch wallet.Type {
		case utils.BTCWalletAsset:
			loadedWallet.BTC.Lock()
		case utils.DCRWalletAsset:
			loadedWallet.DCR.Lock()
		case utils.LTCWalletAsset:
			loadedWallet.LTC.Lock()
		}
	}
}

func (wallet *Wallet) IsLocked() bool {
	loadedWallet, ok := wallet.loader.GetLoadedWallet()
	if !ok {
		return false
	}

	switch wallet.Type {
	case utils.BTCWalletAsset:
		return loadedWallet.BTC.Locked()
	case utils.DCRWalletAsset:
		return loadedWallet.DCR.Locked()
	case utils.LTCWalletAsset:
		return loadedWallet.LTC.Locked()
	default:
		return false
	}
}

// ChangePrivatePassphraseForWallet attempts to change the wallet's passphrase and re-encrypts the seed with the new passphrase.
func (wallet *Wallet) ChangePrivatePassphraseForWallet(oldPrivatePassphrase, newPrivatePassphrase string, privatePassphraseType int32) error {
	if privatePassphraseType != PassphraseTypePin && privatePassphraseType != PassphraseTypePass {
		return errors.New(utils.ErrInvalid)
	}

	oldPassphrase := []byte(oldPrivatePassphrase)
	newPassphrase := []byte(newPrivatePassphrase)
	encryptedMnemonic := wallet.EncryptedMnemonic

	if encryptedMnemonic != nil {
		decryptedSeed, err := decryptWalletMnemonic(oldPassphrase, encryptedMnemonic)
		if err != nil {
			return err
		}

		encryptedMnemonic, err = encryptWalletMnemonic(newPassphrase, decryptedSeed)
		if err != nil {
			return err
		}
	}

	err := wallet.changePrivatePassphrase(oldPassphrase, newPassphrase)
	if err != nil {
		return utils.TranslateError(err)
	}

	wallet.EncryptedMnemonic = encryptedMnemonic
	wallet.PrivatePassphraseType = privatePassphraseType
	err = wallet.db.Save(wallet)
	if err != nil {
		log.Errorf("error saving wallet-[%d] to database after passphrase change: %v", wallet.ID, err)

		err2 := wallet.changePrivatePassphrase(newPassphrase, oldPassphrase)
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
	case utils.LTCWalletAsset:
		err = wallet.Internal().LTC.ChangePrivatePassphrase(oldPass, newPass)
	}
	if err != nil {
		return utils.TranslateError(err)
	}
	return nil
}

func (wallet *Wallet) deleteWallet(privatePassphrase string) error {
	if !wallet.IsWatchingOnlyWallet() {
		err := wallet.UnlockWallet(privatePassphrase)
		if err != nil {
			return err
		}
		wallet.LockWallet()
	}

	wallet.Shutdown() // Initiates full network shutdown here.

	err := wallet.db.DeleteStruct(wallet)
	if err != nil {
		return utils.TranslateError(err)
	}

	log.Info("Deleting Wallet")
	err = os.RemoveAll(wallet.dataDir())
	if err != nil {
		// Currently there is no way to close the file in the datadir
		// before deleting it in the window.
		// Will have to wait for Neutrino's update to provide a way to do this

		// Dir of deleted wallet will clean on next time open application
		log.Warn("Wallet deleted without completely deleting the data dir. It will be deleted on next restart")
		err = nil
	}
	return err
}

func (wallet *Wallet) LogFile() string {
	wallet.mu.RLock()
	defer wallet.mu.RUnlock()
	switch wallet.Type {
	case utils.BTCWalletAsset:
		return filepath.Join(wallet.logDir, btcLogFilename)
	case utils.DCRWalletAsset:
		return filepath.Join(wallet.logDir, dcrLogFilename)
	case utils.LTCWalletAsset:
		return filepath.Join(wallet.logDir, ltcLogFilename)
	}
	return ""
}
