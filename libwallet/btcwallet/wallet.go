package btcwallet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/gcs"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
)

type Wallet struct {
	ID            int       `storm:"id,increment"`
	Name          string    `storm:"unique"`
	CreatedAt     time.Time `storm:"index"`
	EncryptedSeed []byte

	cl          neutrinoService
	neutrinoDB  walletdb.DB
	chainClient *chain.NeutrinoClient
	dataDir     string
	chainParams *chaincfg.Params
	loader      *wallet.Loader
	log         slog.Logger
	birthday    time.Time
}

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

func NewSpvWallet(walletName string, encryptedSeed []byte, net string, log slog.Logger) (*Wallet, error) {
	chainParams, err := parseChainParams(net)
	if err != nil {
		return nil, err
	}

	return &Wallet{
		Name:          walletName,
		chainParams:   chainParams,
		CreatedAt:     time.Now(),
		EncryptedSeed: encryptedSeed,
		log:           log,
	}, nil
}

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
	wallet.UseLogger(logger("BTCW", btclog.LevelInfo))
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
func (w *Wallet) RawRequest(method string, params []json.RawMessage) (json.RawMessage, error) {
	// Not needed for spv wallet.
	return nil, errors.New("RawRequest not available on spv")
}

// createSPVWallet creates a new SPV wallet.
func (w *Wallet) CreateSPVWallet(privPass []byte, seed []byte, dbDir string) error {
	net := w.chainParams
	w.dataDir = filepath.Join(dbDir, strconv.Itoa(w.ID))

	if err := logNeutrino(w.dataDir); err != nil {
		return fmt.Errorf("error initializing btcwallet+neutrino logging: %v", err)
	}

	logDir := filepath.Join(w.dataDir, logDirName)
	err := os.MkdirAll(logDir, 0744)
	if err != nil {
		return fmt.Errorf("error creating wallet directories: %v", err)
	}

	loader := wallet.NewLoader(net, w.dataDir, true, 60*time.Second, 250)
	pubPass := []byte(wallet.InsecurePubPassphrase)

	_, err = loader.CreateNewWallet(pubPass, privPass, seed, walletBirthday)
	if err != nil {
		return fmt.Errorf("CreateNewWallet error: %w", err)
	}

	bailOnWallet := func() {
		if err := loader.UnloadWallet(); err != nil {
			w.log.Errorf("Error unloading wallet after createSPVWallet error: %v", err)
		}
	}

	neutrinoDBPath := filepath.Join(w.dataDir, neutrinoDBName)
	db, err := walletdb.Create("bdb", neutrinoDBPath, true, 5*time.Second)
	if err != nil {
		bailOnWallet()
		return fmt.Errorf("unable to create wallet db at %q: %v", neutrinoDBPath, err)
	}
	if err = db.Close(); err != nil {
		bailOnWallet()
		return fmt.Errorf("error closing newly created wallet database: %w", err)
	}

	if err := loader.UnloadWallet(); err != nil {
		return fmt.Errorf("error unloading wallet: %w", err)
	}

	return nil
}

func (wallet *Wallet) DataDir() string {
	return wallet.dataDir
}

// prepare gets a wallet ready for use by opening the transactions index database
// and initializing the wallet loader which can be used subsequently to create,
// load and unload the wallet.
func (w *Wallet) Prepare(rootDir string, net string, log slog.Logger) (err error) {
	chainParams, err := parseChainParams(net)
	if err != nil {
		return err
	}

	w.chainParams = chainParams
	w.dataDir = filepath.Join(rootDir, strconv.Itoa(w.ID))
	w.log = log
	w.loader = wallet.NewLoader(w.chainParams, w.dataDir, true, 60*time.Second, 250)
	return nil
}

func (w *Wallet) ConnectSPVWallet(ctx context.Context, wg *sync.WaitGroup) (err error) {
	return w.connect(ctx, wg)
}

// connect will start the wallet and begin syncing.
func (w *Wallet) connect(ctx context.Context, wg *sync.WaitGroup) error {
	if err := logNeutrino(w.dataDir); err != nil {
		return fmt.Errorf("error initializing btcwallet+neutrino logging: %v", err)
	}

	err := w.startWallet()
	if err != nil {
		return err
	}

	// txNotes := w.wallet.txNotifications()

	// Nanny for the caches checkpoints and txBlocks caches.
	wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	defer w.stop()
	// 	defer txNotes.Done()

	// 	ticker := time.NewTicker(time.Minute * 20)
	// 	defer ticker.Stop()
	// 	expiration := time.Hour * 2
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			w.txBlocksMtx.Lock()
	// 			for txHash, entry := range w.txBlocks {
	// 				if time.Since(entry.lastAccess) > expiration {
	// 					delete(w.txBlocks, txHash)
	// 				}
	// 			}
	// 			w.txBlocksMtx.Unlock()

	// 			w.checkpointMtx.Lock()
	// 			for outPt, check := range w.checkpoints {
	// 				if time.Since(check.lastAccess) > expiration {
	// 					delete(w.checkpoints, outPt)
	// 				}
	// 			}
	// 			w.checkpointMtx.Unlock()

	// 		case note := <-txNotes.C:
	// 			if len(note.AttachedBlocks) > 0 {
	// 				lastBlock := note.AttachedBlocks[len(note.AttachedBlocks)-1]
	// 				syncTarget := atomic.LoadInt32(&w.syncTarget)

	// 				for ib := range note.AttachedBlocks {
	// 					for _, nt := range note.AttachedBlocks[ib].Transactions {
	// 						w.log.Debugf("Block %d contains wallet transaction %v", note.AttachedBlocks[ib].Height, nt.Hash)
	// 					}
	// 				}

	// 				if syncTarget == 0 || (lastBlock.Height < syncTarget && lastBlock.Height%10_000 != 0) {
	// 					continue
	// 				}

	// 				select {
	// 				case w.tipChan <- &block{
	// 					hash:   *lastBlock.Hash,
	// 					height: int64(lastBlock.Height),
	// 				}:
	// 				default:
	// 					w.log.Warnf("tip report channel was blocking")
	// 				}
	// 			}

	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	return nil
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (w *Wallet) startWallet() error {
	// timeout and recoverWindow arguments borrowed from btcwallet directly.
	w.loader = wallet.NewLoader(w.chainParams, w.dataDir, true, 60*time.Second, 250)

	exists, err := w.loader.WalletExists()
	if err != nil {
		return fmt.Errorf("error verifying wallet existence: %v", err)
	}
	if !exists {
		return errors.New("wallet not found")
	}

	w.log.Debug("Starting native BTC wallet...")
	btcw, err := w.loader.OpenExistingWallet([]byte(wallet.InsecurePubPassphrase), false)
	if err != nil {
		return fmt.Errorf("couldn't load wallet: %w", err)
	}

	bailOnWallet := func() {
		if err := w.loader.UnloadWallet(); err != nil {
			w.log.Errorf("Error unloading wallet: %v", err)
		}
	}

	neutrinoDBPath := filepath.Join(w.dataDir, neutrinoDBName)
	w.neutrinoDB, err = walletdb.Create("bdb", neutrinoDBPath, true, wallet.DefaultDBTimeout)
	if err != nil {
		bailOnWallet()
		return fmt.Errorf("unable to create wallet db at %q: %v", neutrinoDBPath, err)
	}

	bailOnWalletAndDB := func() {
		if err := w.neutrinoDB.Close(); err != nil {
			w.log.Errorf("Error closing neutrino database: %v", err)
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
	switch w.chainParams.Net {
	case wire.MainNet:
		addPeers = []string{"cfilters.ssgen.io"}
	case wire.TestNet3:
		addPeers = []string{"dex-test.ssgen.io"}
	case wire.TestNet, wire.SimNet: // plain "wire.TestNet" is regnet!
		connectPeers = []string{"localhost:20575"}
	}
	w.log.Debug("Starting neutrino chain service...")
	chainService, err := neutrino.NewChainService(neutrino.Config{
		DataDir:       w.dataDir,
		Database:      w.neutrinoDB,
		ChainParams:   *w.chainParams,
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
			w.log.Errorf("Error closing neutrino chain service: %v", err)
		}
		bailOnWalletAndDB()
	}

	w.cl = chainService
	w.chainClient = chain.NewNeutrinoClient(w.chainParams, chainService)
	// w.wallet = &walletExtender{btcw, w.chainParams}

	// oldBday := btcw.Manager.Birthday()
	// wdb := btcw.Database()

	// performRescan := w.birthday.Before(oldBday)
	// if performRescan && !w.allowAutomaticRescan {
	// 	bailOnWalletAndDB()
	// 	return errors.New("cannot set earlier birthday while there are active deals")
	// }

	// if !oldBday.Equal(w.birthday) {
	// 	err = walletdb.Update(wdb, func(dbtx walletdb.ReadWriteTx) error {
	// 		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
	// 		return btcw.Manager.SetBirthday(ns, w.birthday)
	// 	})
	// 	if err != nil {
	// 		w.log.Errorf("Failed to reset wallet manager birthday: %v", err)
	// 		performRescan = false
	// 	}
	// }

	// if performRescan {
	// 	w.forceRescan()
	// }

	if err = w.chainClient.Start(); err != nil { // lazily starts connmgr
		bailOnEverything()
		return fmt.Errorf("couldn't start Neutrino client: %v", err)
	}

	w.log.Info("Synchronizing wallet with network...")
	btcw.SynchronizeRPC(w.chainClient)

	return nil
}
