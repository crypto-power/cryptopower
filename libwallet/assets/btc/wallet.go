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
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader/btc"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// BTCAsset confirm that BTC implements that shared assets interface.
var _ sharedW.Asset = (*BTCAsset)(nil)

type BTCAsset struct {
	*sharedW.Wallet

	cl          neutrinoService
	neutrinoDB  walletdb.DB
	chainClient *chain.NeutrinoClient

	cancelFuncs []context.CancelFunc
	ctx         context.Context

	Synced         bool
	TxAuthoredInfo *TxAuthor

	chainParams *chaincfg.Params
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
func CreateNewWallet(pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.CreateNewWallet(pass, ldr, params, utils.BTCWalletAsset)
	if err != nil {
		return nil, err
	}

	btcWallet := &BTCAsset{
		Wallet:      w,
		chainParams: chainParams,
	}

	btcWallet.SetNetworkCancelCallback(btcWallet.SafelyCancelSync)

	return btcWallet, nil
}

func initWalletLoader(chainParams *chaincfg.Params, dbDirPath string) loader.AssetLoader {
	conf := &btc.LoaderConf{
		ChainParams:      chainParams,
		DBDirPath:        dbDirPath,
		DefaultDBTimeout: defaultDBTimeout,
		RecoveryWin:      recoverWindow,
	}

	return btc.NewLoader(conf)
}

// CreateWatchOnlyWallet accepts the wallet name, extended public key and the
// init parameters to create a watch only wallet for the BTC asset.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BTC asset. It then generates the BTC loader interface
// that is passed to be used upstream while creating the watch only wallet in the
// shared wallet implemenation.
// Immediately a watch only wallet is created, the function to safely cancel network sync
// is set. There after returning the watch only wallet's interface.
func CreateWatchOnlyWallet(walletName, extendedPublicKey string, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.CreateWatchOnlyWallet(walletName, extendedPublicKey,
		ldr, params, utils.BTCWalletAsset)
	if err != nil {
		return nil, err
	}

	btcWallet := &BTCAsset{
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
func RestoreWallet(seedMnemonic string, pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.RestoreWallet(seedMnemonic, pass, ldr, params, utils.BTCWalletAsset)
	if err != nil {
		return nil, err
	}

	btcWallet := &BTCAsset{
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
func LoadExisting(w *sharedW.Wallet, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	btcWallet := &BTCAsset{
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

func (asset *BTCAsset) ConnectSPVWallet(wg *sync.WaitGroup) (err error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	return asset.connect(ctx, wg)
}

// connect will start the wallet and begin syncing.
func (asset *BTCAsset) connect(ctx context.Context, wg *sync.WaitGroup) error {
	err := asset.startWallet()
	if err != nil {
		return err
	}

	// Nanny for the caches checkpoints and txBlocks caches.
	wg.Add(1)

	return nil
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (asset *BTCAsset) startWallet() error {
	// timeout and recoverWindow arguments borrowed from btcwallet directly.

	exists, err := asset.WalletExists()
	if err != nil {
		return fmt.Errorf("error verifying wallet existence: %v", err)
	}
	if !exists {
		return errors.New("wallet not found")
	}

	log.Debug("Starting native BTC wallet...")

	// Depending on the network, we add some addpeers or a connect peer. On
	// regtest, if the peers haven't been explicitly set, add the simnet harness
	// alpha node as an additional peer so we don't have to type it in. On
	// mainet and testnet3, add a known reliable persistent peer to be used in
	// addition to normal DNS seed-based peer discovery.
	var addPeers []string
	var connectPeers []string
	switch asset.chainParams.Net {
	case wire.MainNet:
		addPeers = []string{"cfilters.ssgen.io"}
	case wire.TestNet3:
		addPeers = []string{"dex-test.ssgen.io"}
	case wire.TestNet, wire.SimNet: // plain "wire.TestNet" is regnet!
		connectPeers = []string{"localhost:20575"}
	}
	log.Debug("Starting neutrino chain service...")
	chainService, err := neutrino.NewChainService(neutrino.Config{
		DataDir:       asset.DataDir(),
		Database:      asset.GetWalletDataDb(),
		ChainParams:   *asset.chainParams,
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
		log.Error(err)
		return fmt.Errorf("couldn't create Neutrino ChainService: %v", err)
	}

	bailOnEverything := func() {
		if err := chainService.Stop(); err != nil {
			log.Errorf("Error closing neutrino chain service: %v", err)
		}
	}

	asset.cl = chainService
	asset.chainClient = chain.NewNeutrinoClient(asset.chainParams, chainService)

	if err = asset.chainClient.Start(); err != nil { // lazily starts connmgr
		bailOnEverything()
		return fmt.Errorf("couldn't start Neutrino client: %v", err)
	}

	log.Info("Synchronizing wallet with network...")
	asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	return nil
}

func (asset *BTCAsset) SafelyCancelSync() {
	//TODO: use a proper logger
	fmt.Println(utils.ErrBTCMethodNotImplemented("SafelyCancelSync"))
}

// Methods added below satisfy the shared asset interface. Each should be
// implemented fully to avoid panic if invoked.
func (asset *BTCAsset) IsSynced() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsSynced"))
	return false
}
func (asset *BTCAsset) IsWaiting() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsWaiting"))
	return false
}
func (asset *BTCAsset) IsSyncing() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsSyncing"))
	return false
}
func (asset *BTCAsset) SpvSync() error {
	wg := new(sync.WaitGroup)
	err := asset.ConnectSPVWallet(wg)
	if err != nil {
		log.Warn("error occured when starting BTC sync: ", err)
	}
	return err
}
func (asset *BTCAsset) CancelRescan() {
	log.Warn(utils.ErrBTCMethodNotImplemented("CancelRescan"))
}
func (asset *BTCAsset) CancelSync() {
	log.Warn(utils.ErrBTCMethodNotImplemented("CancelSync"))
}
func (asset *BTCAsset) IsRescanning() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsRescanning"))
	return false
}
func (asset *BTCAsset) RescanBlocks() error {
	err := utils.ErrBTCMethodNotImplemented("RescanBlocks")
	return err
}
func (asset *BTCAsset) ConnectedPeers() int32 {
	log.Warn(utils.ErrBTCMethodNotImplemented("ConnectedPeers"))
	return -1
}
func (asset *BTCAsset) IsConnectedToNetwork() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsConnectedToNetwork"))
	return false
}
func (asset *BTCAsset) PublishUnminedTransactions() error {
	err := utils.ErrBTCMethodNotImplemented("PublishUnminedTransactions")
	return err
}
func (asset *BTCAsset) CountTransactions(txFilter int32) (int, error) {
	err := utils.ErrBTCMethodNotImplemented("CountTransactions")
	return -1, err
}
func (asset *BTCAsset) GetTransactionRaw(txHash string) (*sharedW.Transaction, error) {
	err := utils.ErrBTCMethodNotImplemented("GetTransactionRaw")
	return nil, err
}
func (asset *BTCAsset) TxMatchesFilter(tx *sharedW.Transaction, txFilter int32) bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("TxMatchesFilter"))
	return false
}
func (asset *BTCAsset) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool) ([]sharedW.Transaction, error) {
	err := utils.ErrBTCMethodNotImplemented("GetTransactionsRaw")
	return nil, err
}
func (asset *BTCAsset) GetBestBlock() *sharedW.BlockInfo {
	log.Warn(utils.ErrBTCMethodNotImplemented("GetBestBlock"))
	return nil
}
func (asset *BTCAsset) GetBestBlockHeight() int32 {
	log.Warn(utils.ErrBTCMethodNotImplemented("GetBestBlockHeight"))
	return -1
}
func (asset *BTCAsset) GetBestBlockTimeStamp() int64 {
	log.Warn(utils.ErrBTCMethodNotImplemented("GetBestBlockTimeStamp"))
	return -1
}
func (asset *BTCAsset) SignMessage(passphrase, address, message string) ([]byte, error) {
	err := utils.ErrBTCMethodNotImplemented("SignMessage")
	return nil, err
}
func (asset *BTCAsset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	err := utils.ErrBTCMethodNotImplemented("VerifyMessage")
	return false, err
}
func (asset *BTCAsset) RemoveSpecificPeer() {
	log.Warn(utils.ErrBTCMethodNotImplemented("RemoveSpecificPeer"))
}
func (asset *BTCAsset) SetSpecificPeer(address string) {
	log.Warn(utils.ErrBTCMethodNotImplemented("SetSpecificPeer"))
}
func (asset *BTCAsset) GetExtendedPubKey(account int32) (string, error) {
	err := utils.ErrBTCMethodNotImplemented("GetExtendedPubKey")
	return "", err
}
