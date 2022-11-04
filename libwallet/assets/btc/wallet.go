package btc

import (
	"context"
	"sync"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/gcs"
	"github.com/btcsuite/btcwallet/chain"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
)

// BTCAsset confirm that BTC implements that shared assets interface.
var _ sharedW.Asset = (*BTCAsset)(nil)

type BTCAsset struct {
	*sharedW.Wallet

	chainService neutrinoService
	chainClient  *chain.NeutrinoClient

	TxAuthoredInfo *TxAuthor

	chainParams *chaincfg.Params

	syncInfo   *SyncData
	cancelSync context.CancelFunc
	syncCtx    context.Context

	mu sync.RWMutex
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
		syncInfo: &SyncData{
			syncProgressListeners:           make(map[string]sharedW.SyncProgressListener),
			txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
		},
	}

	if err := btcWallet.prepareChain(); err != nil {
		return nil, err
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
		syncInfo: &SyncData{
			syncProgressListeners:           make(map[string]sharedW.SyncProgressListener),
			txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
		},
	}

	if err := btcWallet.prepareChain(); err != nil {
		return nil, err
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
		syncInfo: &SyncData{
			syncProgressListeners:           make(map[string]sharedW.SyncProgressListener),
			txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
		},
	}

	if err := btcWallet.prepareChain(); err != nil {
		return nil, err
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
		syncInfo: &SyncData{
			syncProgressListeners:           make(map[string]sharedW.SyncProgressListener),
			txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
		},
	}

	err = btcWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	if err := btcWallet.prepareChain(); err != nil {
		return nil, err
	}

	btcWallet.SetNetworkCancelCallback(btcWallet.SafelyCancelSync)

	return btcWallet, nil
}

func (asset *BTCAsset) SafelyCancelSync() {
	if asset.IsConnectedToNetwork() {
		asset.cancelSync()
	}
}

// Methods added below satisfy the shared asset interface. Each should be
// implemented fully to avoid panic if invoked.
func (asset *BTCAsset) IsSynced() bool {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	return asset.syncInfo.synced
}
func (asset *BTCAsset) IsWaiting() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsWaiting"))
	return false
}
func (asset *BTCAsset) IsSyncing() bool {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	return asset.syncInfo.syncing
}

func (asset *BTCAsset) ConnectedPeers() int32 {
	return asset.chainClient.CS.ConnectedCount()
}
func (asset *BTCAsset) IsConnectedToNetwork() bool {
	return asset.IsConnectedToBitcoinNetwork()
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
	block, err := asset.chainClient.CS.BestBlock()
	if err != nil {
		log.Error("GetBestBlock hash for BTC failed, Err: ", err)
		return nil
	}

	return &sharedW.BlockInfo{
		Height:    block.Height,
		Timestamp: block.Timestamp.Unix(),
	}
}

func (asset *BTCAsset) GetBestBlockHeight() int32 {
	return asset.GetBestBlock().Height
}

func (asset *BTCAsset) GetBestBlockTimeStamp() int64 {
	return asset.GetBestBlock().Timestamp
}

func (asset *BTCAsset) GetBlockHeight(hash chainhash.Hash) (int32, error) {
	height, err := asset.chainClient.GetBlockHeight(&hash)
	if err != nil {
		log.Warn("GetBlockHeight for BTC failed, Err: %v", err)
		return -1, err
	}
	return height, nil
}

func (asset *BTCAsset) GetBlockHash(height int64) (*chainhash.Hash, error) {
	blockhash, err := asset.chainClient.GetBlockHash(height)
	if err != nil {
		log.Warn("GetBlockHash for BTC failed, Err: %v", err)
		return nil, err
	}

	return blockhash, nil
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

// GetExtendedPubkey returns the extended public key of the given account,
// to do that it calls btcwallet's AccountProperties method, using KeyScopeBIP0084
// and the account number. On failure it returns error.
func (asset *BTCAsset) GetExtendedPubKey(account int32) (string, error) {
	loadedAsset := asset.Internal().BTC
	if loadedAsset == nil {
		return "", utils.ErrBTCNotInitialized
	}

	extendedPublicKey, err := loadedAsset.AccountProperties(asset.GetScope(), uint32(account))
	if err != nil {
		return "", err
	}
	return extendedPublicKey.AccountPubKey.String(), nil
}

// AccountXPubMatches checks if the xpub of the provided account matches the
// provided xpub.
func (asset *BTCAsset) AccountXPubMatches(account uint32, xPub string) (bool, error) {
	acctXPubKey, err := asset.Internal().BTC.AccountProperties(asset.GetScope(), account)
	if err != nil {
		return false, err
	}

	return acctXPubKey.AccountPubKey.String() == xPub, nil
}
