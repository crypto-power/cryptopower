package dcr

import (
	"context"
	"fmt"
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
	"github.com/decred/dcrd/chaincfg/v3"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	"gitlab.com/raedah/cryptopower/libwallet/internal/vsp"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr/walletdata"
)

type Wallet struct {
	ID        int       `storm:"id,increment"`
	Name      string    `storm:"unique"`
	CreatedAt time.Time `storm:"index"`
	dbDriver  string
	rootDir   string
	db        *storm.DB

	EncryptedSeed         []byte
	IsRestored            bool
	HasDiscoveredAccounts bool
	PrivatePassphraseType int32

	chainParams  *chaincfg.Params
	dataDir      string
	loader       *loader.Loader
	walletDataDB *walletdata.DB

	synced            bool
	syncing           bool
	waitingForHeaders bool

	shuttingDown       chan bool
	cancelFuncs        []context.CancelFunc
	cancelAccountMixer context.CancelFunc `json:"-"`

	cancelAutoTicketBuyerMu sync.Mutex
	cancelAutoTicketBuyer   context.CancelFunc `json:"-"`

	vspClientsMu sync.Mutex
	vspClients   map[string]*vsp.Client

	// setUserConfigValue saves the provided key-value pair to a config database.
	// This function is ideally assigned when the `wallet.prepare` method is
	// called from a MultiWallet instance.
	setUserConfigValue configSaveFn

	// readUserConfigValue returns the previously saved value for the provided
	// key from a config database. Returns nil if the key wasn't previously set.
	// This function is ideally assigned when the `wallet.prepare` method is
	// called from a MultiWallet instance.
	readUserConfigValue configReadFn

	notificationListenersMu          sync.RWMutex
	syncData                         *SyncData
	accountMixerNotificationListener map[string]AccountMixerNotificationListener
	txAndBlockNotificationListeners  map[string]TxAndBlockNotificationListener
	blocksRescanProgressListener     BlocksRescanProgressListener

	vspMu sync.RWMutex
	vsps  []*VSP
}

// prepare gets a wallet ready for use by opening the transactions index database
// and initializing the wallet loader which can be used subsequently to create,
// load and unload the wallet.
func (wallet *Wallet) Prepare(rootDir string, db *storm.DB, chainParams *chaincfg.Params,
	setUserConfigValueFn configSaveFn, readUserConfigValueFn configReadFn) (err error) {

	wallet.db = db
	return wallet.prepare(rootDir, chainParams, setUserConfigValueFn, readUserConfigValueFn)
}

func (wallet *Wallet) prepare(rootDir string, chainParams *chaincfg.Params,
	setUserConfigValueFn configSaveFn, readUserConfigValueFn configReadFn) (err error) {

	wallet.chainParams = chainParams
	wallet.dataDir = filepath.Join(rootDir, strconv.Itoa(wallet.ID))
	wallet.rootDir = rootDir
	// wallet.db = db
	wallet.vspClients = make(map[string]*vsp.Client)
	wallet.setUserConfigValue = setUserConfigValueFn
	wallet.readUserConfigValue = readUserConfigValueFn

	// open database for indexing transactions for faster loading
	walletDataDBPath := filepath.Join(wallet.dataDir, walletdata.DbName)
	oldTxDBPath := filepath.Join(wallet.dataDir, walletdata.OldDbName)
	if exists, _ := fileExists(oldTxDBPath); exists {
		moveFile(oldTxDBPath, walletDataDBPath)
	}
	wallet.walletDataDB, err = walletdata.Initialize(walletDataDBPath, chainParams, &Transaction{})
	if err != nil {
		log.Error(err.Error())
		return err
	}

	wallet.syncData = &SyncData{
		syncProgressListeners: make(map[string]SyncProgressListener),
	}
	wallet.txAndBlockNotificationListeners = make(map[string]TxAndBlockNotificationListener)
	wallet.accountMixerNotificationListener = make(map[string]AccountMixerNotificationListener)

	// init loader
	wallet.loader = initWalletLoader(wallet.chainParams, wallet.dataDir, wallet.dbDriver)

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

	if _, loaded := wallet.loader.LoadedWallet(); loaded {
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
		return 0, errors.New(ErrWalletIsRestored)
	}

	return wallet.CreatedAt.UnixNano() / int64(time.Millisecond), nil
}

func (wallet *Wallet) NetType() string {
	return wallet.chainParams.Name
}

func (wallet *Wallet) Internal() *w.Wallet {
	lw, _ := wallet.loader.LoadedWallet()
	return lw
}

func (wallet *Wallet) WalletExists() (bool, error) {
	return wallet.loader.WalletExists()
}

func CreateNewWallet(walletName, privatePassphrase string, privatePassphraseType int32, db *storm.DB, rootDir, dbDriver string, chainParams *chaincfg.Params) (*Wallet, error) {
	seed, err := GenerateSeed()
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
		chainParams:   chainParams,
		cancelFuncs:   make([]context.CancelFunc, 0),
		CreatedAt:     time.Now(),
		EncryptedSeed: encryptedSeed,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]SyncProgressListener),
		},
		txAndBlockNotificationListeners:  make(map[string]TxAndBlockNotificationListener),
		accountMixerNotificationListener: make(map[string]AccountMixerNotificationListener),
		PrivatePassphraseType:            privatePassphraseType,
		HasDiscoveredAccounts:            true,
	}

	wallet.cancelFuncs = make([]context.CancelFunc, 0)

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare(wallet.rootDir, wallet.chainParams, wallet.walletConfigSetFn(wallet.ID), wallet.walletConfigReadFn(wallet.ID))
		if err != nil {
			return err
		}
		return wallet.CreateWallet(privatePassphrase, seed)
	})
}

func (wallet *Wallet) CreateWallet(privatePassphrase, seedMnemonic string) error {
	log.Info("Creating Wallet")
	if len(seedMnemonic) == 0 {
		return errors.New(ErrEmptySeed)
	}

	pubPass := []byte(w.InsecurePubPassphrase)
	privPass := []byte(privatePassphrase)
	seed, err := walletseed.DecodeUserInput(seedMnemonic)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = wallet.loader.CreateNewWallet(wallet.ShutdownContext(), pubPass, privPass, seed)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("Created Wallet")
	return nil
}

func CreateWatchOnlyWallet(walletName, extendedPublicKey string, db *storm.DB, rootDir, dbDriver string, chainParams *chaincfg.Params) (*Wallet, error) {
	wallet := &Wallet{
		Name:        walletName,
		db:          db,
		dbDriver:    dbDriver,
		rootDir:     rootDir,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]SyncProgressListener),
		},
		IsRestored:            true,
		HasDiscoveredAccounts: true,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare(wallet.rootDir, wallet.chainParams, wallet.walletConfigSetFn(wallet.ID), wallet.walletConfigReadFn(wallet.ID))
		if err != nil {
			return err
		}

		return wallet.createWatchingOnlyWallet(extendedPublicKey)
	})
}

func (wallet *Wallet) createWatchingOnlyWallet(extendedPublicKey string) error {
	pubPass := []byte(w.InsecurePubPassphrase)

	_, err := wallet.loader.CreateWatchingOnlyWallet(wallet.ShutdownContext(), extendedPublicKey, pubPass)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("Created Watching Only Wallet")
	return nil
}

func (wallet *Wallet) RenameWallet(newName string) error {
	if strings.HasPrefix(newName, "wallet-") {
		return errors.E(ErrReservedWalletName)
	}

	if exists, err := wallet.WalletNameExists(newName); err != nil {
		return translateError(err)
	} else if exists {
		return errors.New(ErrExist)
	}

	wallet.Name = newName
	return wallet.db.Save(wallet) // update WalletName field
}

func RestoreWallet(walletName, seedMnemonic, rootDir, dbDriver string, db *storm.DB, chainParams *chaincfg.Params, privatePassphrase string, privatePassphraseType int32) (*Wallet, error) {
	wallet := &Wallet{
		Name:                  walletName,
		PrivatePassphraseType: privatePassphraseType,
		db:                    db,
		dbDriver:              dbDriver,
		rootDir:               rootDir,
		chainParams:           chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]SyncProgressListener),
		},
		IsRestored:            true,
		HasDiscoveredAccounts: false,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare(wallet.rootDir, wallet.chainParams, wallet.walletConfigSetFn(wallet.ID), wallet.walletConfigReadFn(wallet.ID))
		if err != nil {
			return err
		}

		return wallet.CreateWallet(privatePassphrase, seedMnemonic)
	})
}

func (wallet *Wallet) DeleteWallet(privPass []byte) error {

	if wallet.IsConnectedToDecredNetwork() {
		wallet.CancelSync()
		defer func() {
			wallet.SpvSync()
		}()
	}

	err := wallet.deleteWallet(privPass)
	if err != nil {
		return translateError(err)
	}

	return nil
}

func (wallet *Wallet) saveNewWallet(setupWallet func() error) (*Wallet, error) {
	exists, err := wallet.WalletNameExists(wallet.Name)
	if err != nil {
		return nil, err
	} else if exists {
		return nil, errors.New(ErrExist)
	}

	if wallet.IsConnectedToDecredNetwork() {
		wallet.CancelSync()
		defer wallet.SpvSync()
	}
	// Perform database save operations in batch transaction
	// for automatic rollback if error occurs at any point.
	err = wallet.batchDbTransaction(func(db storm.Node) error {
		// saving struct to update ID property with an auto-generated value
		err := db.Save(wallet)
		if err != nil {
			return err
		}

		walletDataDir := filepath.Join(wallet.rootDir, strconv.Itoa(wallet.ID))

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
			wallet.Name = "wallet-" + strconv.Itoa(wallet.ID) // wallet-#
		}
		wallet.dataDir = walletDataDir

		err = db.Save(wallet) // update database with complete wallet information
		if err != nil {
			return err
		}

		return setupWallet()
	})

	if err != nil {
		return nil, translateError(err)
	}

	return wallet, nil
}

func (wallet *Wallet) LinkExistingWallet(walletName, walletDataDir, originalPubPass string, privatePassphraseType int32) (*Wallet, error) {
	// check if `walletDataDir` contains wallet.db
	if !WalletExistsAt(walletDataDir) {
		return nil, errors.New(ErrNotExist)
	}

	ctx, _ := wallet.contextWithShutdownCancel()

	// verify the public passphrase for the wallet being linked before proceeding
	if err := wallet.loadWalletTemporarily(ctx, walletDataDir, originalPubPass, nil); err != nil {
		return nil, err
	}

	wal := &Wallet{
		Name:                  walletName,
		PrivatePassphraseType: privatePassphraseType,
		IsRestored:            true,
		HasDiscoveredAccounts: false, // assume that account discovery hasn't been done
	}

	return wallet.saveNewWallet(func() error {
		// move wallet.db and tx.db files to newly created dir for the wallet
		currentWalletDbFilePath := filepath.Join(walletDataDir, walletDbName)
		newWalletDbFilePath := filepath.Join(wal.dataDir, walletDbName)
		if err := moveFile(currentWalletDbFilePath, newWalletDbFilePath); err != nil {
			return err
		}

		currentTxDbFilePath := filepath.Join(walletDataDir, walletdata.OldDbName)
		newTxDbFilePath := filepath.Join(wallet.dataDir, walletdata.DbName)
		if err := moveFile(currentTxDbFilePath, newTxDbFilePath); err != nil {
			return err
		}

		// prepare the wallet for use and open it
		err := (func() error {
			err := wallet.prepare(wallet.rootDir, wallet.chainParams, wallet.walletConfigSetFn(wallet.ID), wallet.walletConfigReadFn(wallet.ID))
			if err != nil {
				return err
			}

			if originalPubPass == "" || originalPubPass == w.InsecurePubPassphrase {
				return wallet.OpenWallet()
			}

			err = wallet.loadWalletTemporarily(ctx, wallet.dataDir, originalPubPass, func(tempWallet *w.Wallet) error {
				return tempWallet.ChangePublicPassphrase(ctx, []byte(originalPubPass), []byte(w.InsecurePubPassphrase))
			})
			if err != nil {
				return err
			}

			return wallet.OpenWallet()
		})()

		// restore db files to their original location if there was an error
		// in the wallet setup process above
		if err != nil {
			moveFile(newWalletDbFilePath, currentWalletDbFilePath)
			moveFile(newTxDbFilePath, currentTxDbFilePath)
		}

		return err
	})
}

func (wallet *Wallet) IsWatchingOnlyWallet() bool {
	if w, ok := wallet.loader.LoadedWallet(); ok {
		return w.WatchingOnly()
	}

	return false
}

func (wallet *Wallet) OpenWallet() error {
	pubPass := []byte(w.InsecurePubPassphrase)

	_, err := wallet.loader.OpenExistingWallet(wallet.ShutdownContext(), pubPass)
	if err != nil {
		log.Error(err)
		return translateError(err)
	}

	return nil
}

func (wallet *Wallet) WalletOpened() bool {
	return wallet.Internal() != nil
}

func (wallet *Wallet) UnlockWallet(privPass []byte) error {
	return wallet.unlockWallet(privPass)
}

func (wallet *Wallet) unlockWallet(privPass []byte) error {
	loadedWallet, ok := wallet.loader.LoadedWallet()
	if !ok {
		return fmt.Errorf("wallet has not been loaded")
	}

	ctx, _ := wallet.ShutdownContextWithCancel()
	err := loadedWallet.Unlock(ctx, privPass, nil)
	if err != nil {
		return translateError(err)
	}

	return nil
}

func (wallet *Wallet) LockWallet() {
	if wallet.IsAccountMixerActive() {
		log.Error("LockWallet ignored due to active account mixer")
		return
	}

	if !wallet.Internal().Locked() {
		wallet.Internal().Lock()
	}
}

func (wallet *Wallet) IsLocked() bool {
	return wallet.Internal().Locked()
}

// ChangePrivatePassphraseForWallet attempts to change the wallet's passphrase and re-encrypts the seed with the new passphrase.
func (wallet *Wallet) ChangePrivatePassphraseForWallet(oldPrivatePassphrase, newPrivatePassphrase []byte, privatePassphraseType int32) error {
	if privatePassphraseType != PassphraseTypePin && privatePassphraseType != PassphraseTypePass {
		return errors.New(ErrInvalid)
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
		return translateError(err)
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
			return errors.New(ErrSavingWallet)
		}

		return errors.New(ErrChangingPassphrase)
	}

	return nil
}

func (wallet *Wallet) changePrivatePassphrase(oldPass []byte, newPass []byte) error {
	defer func() {
		for i := range oldPass {
			oldPass[i] = 0
		}

		for i := range newPass {
			newPass[i] = 0
		}
	}()

	err := wallet.Internal().ChangePrivatePassphrase(wallet.ShutdownContext(), oldPass, newPass)
	if err != nil {
		return translateError(err)
	}
	return nil
}

func (wallet *Wallet) deleteWallet(privatePassphrase []byte) error {
	defer func() {
		for i := range privatePassphrase {
			privatePassphrase[i] = 0
		}
	}()

	if _, loaded := wallet.loader.LoadedWallet(); !loaded {
		return errors.New(ErrWalletNotLoaded)
	}

	if !wallet.IsWatchingOnlyWallet() {
		err := wallet.Internal().Unlock(wallet.ShutdownContext(), privatePassphrase, nil)
		if err != nil {
			return translateError(err)
		}
		wallet.Internal().Lock()
	}

	wallet.Shutdown()

	err := wallet.db.DeleteStruct(wallet)
	if err != nil {
		return translateError(err)
	}

	log.Info("Deleting Wallet")
	return os.RemoveAll(wallet.dataDir)
}

// DecryptSeed decrypts wallet.EncryptedSeed using privatePassphrase
func (wallet *Wallet) DecryptSeed(privatePassphrase []byte) (string, error) {
	if wallet.EncryptedSeed == nil {
		return "", errors.New(ErrInvalid)
	}

	return decryptWalletSeed(privatePassphrase, wallet.EncryptedSeed)
}

// AccountXPubMatches checks if the xpub of the provided account matches the
// provided legacy or SLIP0044 xpub. While both the legacy and SLIP0044 xpubs
// will be checked for watch-only wallets, other wallets will only check the
// xpub that matches the coin type key used by the wallet.
func (wallet *Wallet) AccountXPubMatches(account uint32, legacyXPub, slip044XPub string) (bool, error) {
	ctx := wallet.ShutdownContext()

	acctXPubKey, err := wallet.Internal().AccountXpub(ctx, account)
	if err != nil {
		return false, err
	}
	acctXPub := acctXPubKey.String()

	if wallet.IsWatchingOnlyWallet() {
		// Coin type info isn't saved for watch-only wallets, so check
		// against both legacy and SLIP0044 coin types.
		return acctXPub == legacyXPub || acctXPub == slip044XPub, nil
	}

	cointype, err := wallet.Internal().CoinType(ctx)
	if err != nil {
		return false, err
	}

	if cointype == wallet.chainParams.LegacyCoinType {
		return acctXPub == legacyXPub, nil
	} else {
		return acctXPub == slip044XPub, nil
	}
}

// VerifySeedForWallet compares seedMnemonic with the decrypted wallet.EncryptedSeed and clears wallet.EncryptedSeed if they match.
func (wallet *Wallet) VerifySeedForWallet(seedMnemonic string, privpass []byte) (bool, error) {
	decryptedSeed, err := decryptWalletSeed(privpass, wallet.EncryptedSeed)
	if err != nil {
		return false, err
	}

	if decryptedSeed == seedMnemonic {
		wallet.EncryptedSeed = nil
		return true, translateError(wallet.db.Save(wallet))
	}

	return false, errors.New(ErrInvalid)
}

func (wallet *Wallet) DataDir() string {
	return wallet.dataDir
}

func (wallet *Wallet) Synced() bool {
	return wallet.synced
}
