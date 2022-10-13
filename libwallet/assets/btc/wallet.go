package btc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/gcs"
	"github.com/btcsuite/btcwallet/chain"
	w "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader/btc"
	"gitlab.com/raedah/cryptopower/libwallet/utils"

	"github.com/asdine/storm"
)

type Wallet struct {
	ID            int       `storm:"id,increment"`
	Name          string    `storm:"unique"`
	CreatedAt     time.Time `storm:"index"`
	dbDriver      string
	rootDir       string
	db            *storm.DB
	EncryptedSeed []byte
	IsRestored    bool

	cl          neutrinoService
	neutrinoDB  walletdb.DB
	chainClient *chain.NeutrinoClient

	dataDir     string
	cancelFuncs []context.CancelFunc
	ctx         context.Context

	Synced bool

	chainParams *chaincfg.Params
	loader      loader.AssetLoader
	log         slog.Logger
	birthday    time.Time

	Type string
}

const (
	BTCWallet = "BTC"
)

// neutrinoService is satisfied by *neutrino.ChainService.
type neutrinoService interface {
	GetBlockHash(int64) (*chainhash.Hash, error)
	BestBlock() (*headerfs.BlockStamp, error)
	Peers() []*neutrino.ServerPeer
	GetBlockHeight(hash *chainhash.Hash) (int32, error)
	GetBlockHeader(*chainhash.Hash) (*wire.BlockHeader, error)
	GetCFilter(blockHash chainhash.Hash, filterType wire.FilterType, options ...neutrino.QueryOption) (*gcs.Filter, error)
	GetBlock(blockHash chainhash.Hash, options ...neutrino.QueryOption) (*btcutil.Block, error)
	Stop() error
}

var _ neutrinoService = (*neutrino.ChainService)(nil)

var (
	walletBirthday time.Time
	loggingInited  uint32
)

const (
	neutrinoDBName = "neutrino.db"
	logDirName     = "logs"
	logFileName    = "neutrino.log"
)

func parseChainParams(net string) (*chaincfg.Params, error) {
	switch net {
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	case "testnet3":
		return &chaincfg.TestNet3Params, nil
	case "regtest", "regnet", "simnet":
		return &chaincfg.RegressionNetParams, nil
	}
	return nil, fmt.Errorf("unknown network ID %v", net)
}

// logWriter implements an io.Writer that outputs to a rotating log file.
type logWriter struct {
	*rotator.Rotator
}

// logNeutrino initializes logging in the neutrino + wallet packages. Logging
// only has to be initialized once, so an atomic flag is used internally to
// return early on subsequent invocations.
//
// In theory, the the rotating file logger must be Close'd at some point, but
// there are concurrency issues with that since btcd and btcwallet have
// unsupervised goroutines still running after shutdown. So we leave the rotator
// running at the risk of losing some logs.
func logNeutrino(walletDir string) error {
	if !atomic.CompareAndSwapUint32(&loggingInited, 0, 1) {
		return nil
	}

	logSpinner, err := logRotator(walletDir)
	if err != nil {
		return fmt.Errorf("error initializing log rotator: %w", err)
	}

	backendLog := btclog.NewBackend(logWriter{logSpinner})

	logger := func(name string, lvl btclog.Level) btclog.Logger {
		l := backendLog.Logger(name)
		l.SetLevel(lvl)
		return l
	}

	neutrino.UseLogger(logger("NTRNO", btclog.LevelDebug))
	w.UseLogger(logger("BTCW", btclog.LevelInfo))
	wtxmgr.UseLogger(logger("TXMGR", btclog.LevelInfo))
	chain.UseLogger(logger("CHAIN", btclog.LevelInfo))

	return nil
}

// logRotator initializes a rotating file logger.
func logRotator(netDir string) (*rotator.Rotator, error) {
	const maxLogRolls = 8
	logDir := filepath.Join(netDir, logDirName)
	if err := os.MkdirAll(logDir, 0744); err != nil {
		return nil, fmt.Errorf("error creating log directory: %w", err)
	}

	logFilename := filepath.Join(logDir, logFileName)
	return rotator.New(logFilename, 32*1024, false, maxLogRolls)
}
func (wallet *Wallet) RawRequest(method string, params []json.RawMessage) (json.RawMessage, error) {
	// Not needed for spv wallet.
	return nil, errors.New("RawRequest not available on spv")
}

// prepare gets a wallet ready for use by opening the transactions index database
// and initializing the wallet loader which can be used subsequently to create,
// load and unload the wallet.
func (wallet *Wallet) Prepare(rootDir, net string, db *storm.DB, log slog.Logger) (err error) {
	wallet.db = db
	return wallet.prepare(rootDir, net, log)
}

func (wallet *Wallet) prepare(rootDir string, net string, log slog.Logger) (err error) {
	chainParams, err := parseChainParams(net)
	if err != nil {
		return err
	}

	ctx, cancelfunc := context.WithCancel(context.Background())
	wallet.ctx = ctx

	wallet.cancelFuncs = append(wallet.cancelFuncs, cancelfunc)

	wallet.chainParams = chainParams
	wallet.rootDir = rootDir
	wallet.dataDir = filepath.Join(rootDir, strconv.Itoa(wallet.ID))
	wallet.log = log
	wallet.loader = btc.NewLoader(chainParams, rootDir, time.Duration(100), 200)
	return nil
}

func (wallet *Wallet) Shutdown(walletDBRef *storm.DB) {
	// Trigger shuttingDown signal to cancel all contexts created with
	// `wallet.shutdownContext()` or `wallet.shutdownContextWithCancel()`.
	// wallet.shuttingDown <- true

	if _, loaded := wallet.loader.GetLoadedWallet(); loaded {
		wallet.loader.UnloadWallet()
	}

	for _, f := range wallet.cancelFuncs {
		f()
	}

	if walletDBRef != nil {
		walletDBRef.Close()
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
	lw, _ := wallet.loader.GetLoadedWallet()
	return lw.BTC
}

func (wallet *Wallet) WalletExists() (bool, error) {
	return wallet.loader.WalletExists(strconv.Itoa(wallet.ID))
}

func CreateNewWallet(walletName, privatePassphrase string, privatePassphraseType int32, db *storm.DB, rootDir, dbDriver string, chainParams *chaincfg.Params) (*Wallet, error) {

	encryptedSeed, err := hdkeychain.GenerateSeed(
		hdkeychain.RecommendedSeedLen,
	)
	if err != nil {
		return nil, err
	}

	wallet := &Wallet{
		Name:          walletName,
		db:            db,
		dbDriver:      dbDriver,
		rootDir:       rootDir,
		chainParams:   chainParams,
		CreatedAt:     time.Now(),
		EncryptedSeed: encryptedSeed,
		Type:          BTCWallet,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare(wallet.rootDir, "testnet3", wallet.log)
		if err != nil {
			return err
		}

		return wallet.createWallet(privatePassphrase, encryptedSeed)
	})
}

func (wallet *Wallet) createWallet(privatePassphrase string, seedMnemonic []byte) error {

	defer func() {
		for i := range seedMnemonic {
			seedMnemonic[i] = 0
		}
	}()

	if len(seedMnemonic) == 0 {
		return errors.New("ErrEmptySeed")
	}

	params := &loader.CreateWalletParams{
		WalletID:       strconv.Itoa(wallet.ID),
		PubPassphrase:  []byte(w.InsecurePubPassphrase),
		PrivPassphrase: []byte(privatePassphrase),
		Seed:           seedMnemonic,
	}

	_, err := wallet.loader.CreateNewWallet(wallet.ctx, params)
	if err != nil {
		return err
	}

	bailOnWallet := func() {
		if err := wallet.loader.UnloadWallet(); err != nil {
			fmt.Errorf("Error unloading wallet after createSPVWallet error: %v", err)
		}
	}

	neutrinoDBPath := filepath.Join(wallet.DataDir(), neutrinoDBName)
	db, err := walletdb.Create("bdb", neutrinoDBPath, true, 5*time.Second)
	if err != nil {
		bailOnWallet()
		return fmt.Errorf("unable to create wallet db at %q: %v", neutrinoDBPath, err)
	}
	if err = db.Close(); err != nil {
		bailOnWallet()
		return fmt.Errorf("error closing newly created wallet database: %w", err)
	}

	if err := wallet.loader.UnloadWallet(); err != nil {
		return fmt.Errorf("error unloading wallet: %w", err)
	}

	return nil
}

func CreateNewWatchOnlyWallet(walletName string, chainParams *chaincfg.Params) (*Wallet, error) {
	wallet := &Wallet{
		Name:       walletName,
		IsRestored: true,
		Type:       BTCWallet,
	}

	return wallet.saveNewWallet(func() error {
		err := wallet.prepare(wallet.rootDir, "testnet3", wallet.log)
		if err != nil {
			return err
		}

		return wallet.createWatchingOnlyWallet()
	})
}

func (wallet *Wallet) createWatchingOnlyWallet() error {
	params := &loader.WatchOnlyWalletParams{
		WalletID:      strconv.Itoa(wallet.ID),
		PubPassphrase: []byte(w.InsecurePubPassphrase),
	}

	_, err := wallet.loader.CreateWatchingOnlyWallet(wallet.ctx, params)
	if err != nil {
		return err
	}

	return nil
}

func (wallet *Wallet) IsWatchingOnlyWallet() bool {
	if _, ok := wallet.loader.GetLoadedWallet(); ok {
		return false
	}

	return false
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

func (wallet *Wallet) OpenWallet() error {
	pubPass := []byte(w.InsecurePubPassphrase)

	_, err := wallet.loader.OpenExistingWallet(wallet.ctx, strconv.Itoa(wallet.ID), pubPass)
	if err != nil {
		return translateError(err)
	}

	return nil
}

func (wallet *Wallet) WalletOpened() bool {
	return wallet.Internal() != nil
}

func (wallet *Wallet) UnlockWallet(privPass []byte) error {
	loadedWallet, ok := wallet.loader.GetLoadedWallet()
	if !ok {
		return fmt.Errorf("wallet has not been loaded")
	}

	err := loadedWallet.BTC.Unlock(privPass, nil)
	if err != nil {
		return translateError(err)
	}

	return nil
}

func (wallet *Wallet) LockWallet() {
	if !wallet.Internal().Locked() {
		wallet.Internal().Lock()
	}
}

func (wallet *Wallet) IsLocked() bool {
	return wallet.Internal().Locked()
}

func (wallet *Wallet) ChangePrivatePassphrase(oldPass []byte, newPass []byte) error {
	defer func() {
		for i := range oldPass {
			oldPass[i] = 0
		}

		for i := range newPass {
			newPass[i] = 0
		}
	}()

	err := wallet.Internal().ChangePrivatePassphrase(oldPass, newPass)
	if err != nil {
		return translateError(err)
	}
	return nil
}

func (wallet *Wallet) DeleteWallet(privatePassphrase []byte) error {
	defer func() {
		for i := range privatePassphrase {
			privatePassphrase[i] = 0
		}
	}()

	if _, loaded := wallet.loader.GetLoadedWallet(); !loaded {
		return errors.New(ErrWalletNotLoaded)
	}

	if !wallet.IsWatchingOnlyWallet() {
		err := wallet.Internal().Unlock(privatePassphrase, nil)
		if err != nil {
			return translateError(err)
		}
		wallet.Internal().Lock()
	}

	return os.RemoveAll(wallet.dataDir)
}

func (wallet *Wallet) ConnectSPVWallet(ctx context.Context, wg *sync.WaitGroup) (err error) {
	return wallet.connect(ctx, wg)
}

// connect will start the wallet and begin syncing.
func (wallet *Wallet) connect(ctx context.Context, wg *sync.WaitGroup) error {
	if err := logNeutrino(wallet.dataDir); err != nil {
		return fmt.Errorf("error initializing btcwallet+neutrino logging: %v", err)
	}

	err := wallet.startWallet()
	if err != nil {
		return err
	}

	// Nanny for the caches checkpoints and txBlocks caches.
	wg.Add(1)

	return nil
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (wallet *Wallet) startWallet() error {
	// timeout and recoverWindow arguments borrowed from btcwallet directly.
	wallet.loader = btc.NewLoader(wallet.chainParams, wallet.dataDir, time.Duration(100), 200)

	exists, err := wallet.loader.WalletExists(strconv.Itoa(wallet.ID))
	if err != nil {
		return fmt.Errorf("error verifying wallet existence: %v", err)
	}
	if !exists {
		return errors.New("wallet not found")
	}

	wallet.log.Debug("Starting native BTC wallet...")
	btcw, err := wallet.loader.OpenExistingWallet(wallet.ctx, strconv.Itoa(wallet.ID), []byte(w.InsecurePubPassphrase))
	if err != nil {
		return fmt.Errorf("couldn't load wallet: %w", err)
	}

	bailOnWallet := func() {
		if err := wallet.loader.UnloadWallet(); err != nil {
			wallet.log.Errorf("Error unloading wallet: %v", err)
		}
	}

	neutrinoDBPath := filepath.Join(wallet.DataDir(), neutrinoDBName)
	wallet.neutrinoDB, err = walletdb.Create("bdb", neutrinoDBPath, true, w.DefaultDBTimeout)
	if err != nil {
		bailOnWallet()
		return fmt.Errorf("unable to create wallet db at %q: %v", neutrinoDBPath, err)
	}

	bailOnWalletAndDB := func() {
		if err := wallet.neutrinoDB.Close(); err != nil {
			wallet.log.Errorf("Error closing neutrino database: %v", err)
		}
		bailOnWallet()
	}

	// Depending on the network, we add some addpeers or a connect peer. On
	// regtest, if the peers haven't been explicitly set, add the simnet harness
	// alpha node as an additional peer so we don't have to type it in. On
	// mainet and testnet3, add a known reliable persistent peer to be used in
	// addition to normal DNS seed-based peer discovery.
	var addPeers []string
	var connectPeers []string
	switch wallet.chainParams.Net {
	case wire.MainNet:
		addPeers = []string{"cfilters.ssgen.io"}
	case wire.TestNet3:
		addPeers = []string{"dex-test.ssgen.io"}
	case wire.TestNet, wire.SimNet: // plain "wire.TestNet" is regnet!
		connectPeers = []string{"localhost:20575"}
	}
	wallet.log.Debug("Starting neutrino chain service...")
	chainService, err := neutrino.NewChainService(neutrino.Config{
		DataDir:       wallet.dataDir,
		Database:      wallet.neutrinoDB,
		ChainParams:   *wallet.chainParams,
		PersistToDisk: true, // keep cfilter headers on disk for efficient rescanning
		AddPeers:      addPeers,
		ConnectPeers:  connectPeers,
		// WARNING: PublishTransaction currently uses the entire duration
		// because if an external bug, but even if the resolved, a typical
		// inv/getdata round trip is ~4 seconds, so we set this so neutrino does
		// not cancel queries too readily.
		BroadcastTimeout: 6 * time.Second,
	})
	if err != nil {
		bailOnWalletAndDB()
		return fmt.Errorf("couldn't create Neutrino ChainService: %v", err)
	}

	bailOnEverything := func() {
		if err := chainService.Stop(); err != nil {
			wallet.log.Errorf("Error closing neutrino chain service: %v", err)
		}
		bailOnWalletAndDB()
	}

	wallet.cl = chainService
	wallet.chainClient = chain.NewNeutrinoClient(wallet.chainParams, chainService)

	if err = wallet.chainClient.Start(); err != nil { // lazily starts connmgr
		bailOnEverything()
		return fmt.Errorf("couldn't start Neutrino client: %v", err)
	}

	wallet.log.Info("Synchronizing wallet with network...")
	btcw.BTC.SynchronizeRPC(wallet.chainClient)

	return nil
}

// saveNewWallet performs the following tasks using a db batch operation to ensure
// that db changes are rolled back if any of the steps below return an error.
//
// - saves the initial wallet info to btcWallet.walletsDb to get a wallet id
// - creates a data directory for the wallet using the auto-generated wallet id
// - updates the initial wallet info with name, dataDir (created above), db driver
//   and saves the updated info to btcWallet.walletsDb
// - calls the provided `setupWallet` function to perform any necessary creation,
//   restoration or linking of the just saved wallet
//
// IFF all the above operations succeed, the wallet info will be persisted to db
// and the wallet will be added to `btcWallet.wallets`.
func (wallet *Wallet) saveNewWallet(setupWallet func() error) (*Wallet, error) {
	exists, err := WalletNameExists(wallet.Name, wallet.db)
	if err != nil {
		return nil, err
	} else if exists {
		return nil, errors.New(ErrExist)
	}

	// Perform database save operations in batch transaction
	// for automatic rollback if error occurs at any point.
	err = wallet.batchDbTransaction(func(db storm.Node) error {
		// saving struct to update ID property with an auto-generated value
		err := db.Save(wallet)
		if err != nil {
			return err
		}

		walletDataDir := wallet.DataDir()

		dirExists, err := fileExists(walletDataDir)
		if err != nil {
			return err
		} else if dirExists {
			_, err := backupFile(walletDataDir, 1)
			if err != nil {
				return err
			}

		}

		os.MkdirAll(walletDataDir, os.ModePerm) // create wallet dir

		if wallet.Name == "" {
			wallet.Name = "wallet-" + strconv.Itoa(wallet.ID) // wallet-#
		}

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

func (wallet *Wallet) DataDir() string {
	return filepath.Join(wallet.rootDir, string(utils.BTCWalletAsset), strconv.Itoa(wallet.ID))
}
