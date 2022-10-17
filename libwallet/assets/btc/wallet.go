package btc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/gcs"
	"github.com/btcsuite/btcwallet/chain"
	w "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/decred/slog"
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader/btc"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

type Wallet struct {
	*mainW.Wallet

	cl          neutrinoService
	neutrinoDB  walletdb.DB
	chainClient *chain.NeutrinoClient

	cancelFuncs []context.CancelFunc
	ctx         context.Context

	Synced bool

	chainParams *chaincfg.Params
	log         slog.Logger
}

const (
	recoverWindow    = 200
	defaultDBTimeout = time.Duration(100)
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

// CreateWatchOnlyWallet accepts the wallet name, extended public key and the
// init parameters to create a watch only wallet for the BTC asset.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BTC asset. It then generates the BTC loader interface
// that is passed to be used upstream while creating the watch only wallet in the
// shared wallet implemenation.
// Immediately a watch only wallet is created, the function to safely cancel network sync
// is set. There after returning the watch only wallet's interface.
func CreateNewWallet(pass *mainW.WalletPassInfo, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := btc.NewLoader(chainParams, params.RootDir, defaultDBTimeout, recoverWindow)
	w, err := mainW.CreateNewWallet(pass, utils.BTCWalletAsset, ldr, params)
	if err != nil {
		return nil, err
	}

	btcWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
	}

	btcWallet.SetNetworkCancelCallback(btcWallet.SafelyCancelSync)

	return btcWallet, nil
}

// CreateWatchOnlyWallet accepts the wallet name, extended public key and the
// init parameters to create a watch only wallet for the BTC asset.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BTC asset. It then generates the BTC loader interface
// that is passed to be used upstream while creating the watch only wallet in the
// shared wallet implemenation.
// Immediately a watch only wallet is created, the function to safely cancel network sync
// is set. There after returning the watch only wallet's interface.
func CreateWatchOnlyWallet(walletName, extendedPublicKey string, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := btc.NewLoader(chainParams, params.RootDir, defaultDBTimeout, recoverWindow)
	w, err := mainW.CreateWatchOnlyWallet(walletName, extendedPublicKey,
		utils.BTCWalletAsset, ldr, params)
	if err != nil {
		return nil, err
	}

	btcWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
	}

	btcWallet.SetNetworkCancelCallback(btcWallet.SafelyCancelSync)

	return btcWallet, nil
}

// RestoreWallet accepts the seed, wallet pass information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BTC asset. It then generates the BTC loader interface
// that is passed to be used upstream while restoring the wallet in the
// shared wallet implemenation.
// Immediately wallet restore is complete, the function to safely cancel network sync
// is set. There after returning the restored wallet's interface.
func RestoreWallet(seedMnemonic string, pass *mainW.WalletPassInfo, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := btc.NewLoader(chainParams, params.RootDir, defaultDBTimeout, recoverWindow)
	w, err := mainW.RestoreWallet(seedMnemonic, pass, utils.BTCWalletAsset, ldr, params)
	if err != nil {
		return nil, err
	}

	btcWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
	}

	btcWallet.SetNetworkCancelCallback(btcWallet.SafelyCancelSync)

	return btcWallet, nil
}

// LoadExisting accepts the stored shared wallet information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BTC asset. It then generates the BTC loader interface
// that is passed to be used upstream while loading the existing the wallet in the
// shared wallet implemenation.
// Immediately loading the existing wallet is complete, the function to safely
// cancel network sync is set. There after returning the loaded wallet's interface.
func LoadExisting(w *mainW.Wallet, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := btc.NewLoader(chainParams, params.RootDir, defaultDBTimeout, recoverWindow)
	btcWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
	}

	err = btcWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	btcWallet.SetNetworkCancelCallback(btcWallet.SafelyCancelSync)

	return btcWallet, nil
}

//TODO: NOT USED.
// connect will start the wallet and begin syncing.
func (wallet *Wallet) connect(ctx context.Context, wg *sync.WaitGroup) error {
	if err := logNeutrino(wallet.DataDir()); err != nil {
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

//TODO: NOT USED.
// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (wallet *Wallet) startWallet() error {
	// timeout and recoverWindow arguments borrowed from btcwallet directly.

	exists, err := wallet.WalletExists()
	if err != nil {
		return fmt.Errorf("error verifying wallet existence: %v", err)
	}
	if !exists {
		return errors.New("wallet not found")
	}

	wallet.log.Debug("Starting native BTC wallet...")
	err = wallet.OpenWallet()
	if err != nil {
		return fmt.Errorf("couldn't load wallet: %w", err)
	}

	// https://pkg.go.dev/github.com/btcsuite/btcwallet/walletdb@v1.4.0#DB
	// For neutrino to be completely compatible with the walletDbData implementation
	// in gitlab.com/raedah/cryptopower/libwallet/assets/wallet/walletdata the above
	// interface needs to be fully implemented.
	neutrinoDBPath := wallet.GetWalletDataDb().Path
	wallet.neutrinoDB, err = walletdb.Open("bdb", neutrinoDBPath, true, w.DefaultDBTimeout)
	if err != nil {
		return fmt.Errorf("unable to open wallet db at %q: %v", neutrinoDBPath, err)
	}

	bailOnWalletAndDB := func() {
		if err := wallet.neutrinoDB.Close(); err != nil {
			wallet.log.Errorf("Error closing neutrino database: %v", err)
		}
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
		DataDir:       wallet.DataDir(),
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
	wallet.Internal().BTC.SynchronizeRPC(wallet.chainClient)

	return nil
}

func (wallet *Wallet) SafelyCancelSync() {
	//TODO: use a proper logger
	fmt.Println("Safe sync shutdown not implemented")
}
