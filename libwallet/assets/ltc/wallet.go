package ltc

import (
	"context"
	"fmt"
	"sync"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/ltc"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"

	// "decred.org/dcrwallet/v2/errors"
	// "github.com/LTCsuite/LTCd/LTCec/v2/ecdsa"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/ltcsuite/ltcd/ltcutil/gcs"

	// "github.com/LTCsuite/LTCd/LTCutil/gcs"
	// "github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/wire"

	// "github.com/ltcsuite/ltcwallet/chain"
	labschain "github.com/dcrlabs/neutrino-ltc/chain"
	ltcchaincfg "github.com/ltcsuite/ltcd/chaincfg"
	_ "github.com/ltcsuite/ltcwallet/walletdb/bdb" // bdb init() registers a driver

	// "github.com/lightninglabs/neutrino"
	neutrino "github.com/dcrlabs/neutrino-ltc"
	"github.com/dcrlabs/neutrino-ltc/headerfs"
	// LTCneutrino "github.com/lightninglabs/neutrino"
	// "github.com/lightninglabs/neutrino/headerfs"
)

// Asset confirm that LTC implements that shared assets interface.
var _ sharedW.Asset = (*Asset)(nil)

// Asset is a wrapper around the LTCwallet.Wallet struct.
// It implements the sharedW.Asset interface.
// It also implements the sharedW.AssetsManagerDB interface.
// This is done to allow the Asset to be used as a db interface
// for the AssetsManager.
type Asset struct {
	*sharedW.Wallet

	chainClient *labschain.NeutrinoClient
	chainParams *ltcchaincfg.Params
	// TxAuthoredInfo *TxAuthor

	cancelSync context.CancelFunc
	syncCtx    context.Context

	// This field has been added to cache the expensive call to GetTransactions.
	// If the best block height hasn't changed there is no need to make another
	// expensive GetTransactions call.
	// txs txCache

	// This fields helps to prevent unnecessary API calls if a new block hasn't
	// been introduced.
	// fees feeEstimateCache

	// rescanStarting is set while reloading the wallet and dropping
	// transactions from the wallet db.
	rescanStarting uint32 // atomic

	notificationListenersMu sync.RWMutex

	syncData                        *SyncData
	txAndBlockNotificationListeners map[string]sharedW.TxAndBlockNotificationListener
	blocksRescanProgressListener    sharedW.BlocksRescanProgressListener
}

const (
	recoverWindow    = 200 // If recoveryWindow is set to 0, there will be invalid block filter error.
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
	GetBlock(blockHash chainhash.Hash, options ...neutrino.QueryOption) (*ltcutil.Block, error)
	Stop() error
}

var _ neutrinoService = (*neutrino.ChainService)(nil)

// CreateNewWallet creates a new wallet for the LTC asset.
func CreateNewWallet(pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.LTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.CreateNewWallet(pass, ldr, params, utils.LTCWalletAsset)
	if err != nil {
		return nil, err
	}

	fmt.Printf("wallet created LTC %v \n", w)

	ltcWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	if err := ltcWallet.prepareChain(); err != nil {
		return nil, err
	}

	ltcWallet.SetNetworkCancelCallback(ltcWallet.SafelyCancelSync)

	return ltcWallet, nil
}

func initWalletLoader(chainParams *ltcchaincfg.Params, dbDirPath string) loader.AssetLoader {
	conf := &ltc.LoaderConf{
		ChainParams:      chainParams,
		DBDirPath:        dbDirPath,
		DefaultDBTimeout: defaultDBTimeout,
		RecoveryWin:      recoverWindow,
	}

	return ltc.NewLoader(conf)
}

// CreateWatchOnlyWallet accepts the wallet name, extended public key and the
// init parameters to create a watch only wallet for the LTC asset.
// It validates the network type passed by fetching the chain parameters
// associated with it for the LTC asset. It then generates the LTC loader interface
// that is passed to be used upstream while creating the watch only wallet in the
// shared wallet implemenation.
// Immediately a watch only wallet is created, the function to safely cancel network sync
// is set. There after returning the watch only wallet's interface.
func CreateWatchOnlyWallet(walletName, extendedPublicKey string, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.LTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.CreateWatchOnlyWallet(walletName, extendedPublicKey,
		ldr, params, utils.LTCWalletAsset)
	if err != nil {
		return nil, err
	}

	ltcWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	if err := ltcWallet.prepareChain(); err != nil {
		return nil, err
	}

	ltcWallet.SetNetworkCancelCallback(ltcWallet.SafelyCancelSync)

	return ltcWallet, nil
}

// RestoreWallet accepts the seed, wallet pass information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the LTC asset. It then generates the LTC loader interface
// that is passed to be used upstream while restoring the wallet in the
// shared wallet implemenation.
// Immediately wallet restore is complete, the function to safely cancel network sync
// is set. There after returning the restored wallet's interface.
func RestoreWallet(seedMnemonic string, pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.LTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.RestoreWallet(seedMnemonic, pass, ldr, params, utils.LTCWalletAsset)
	if err != nil {
		return nil, err
	}

	ltcWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	if err := ltcWallet.prepareChain(); err != nil {
		return nil, err
	}

	ltcWallet.SetNetworkCancelCallback(ltcWallet.SafelyCancelSync)

	return ltcWallet, nil
}

// LoadExisting accepts the stored shared wallet information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the LTC asset. It then generates the LTC loader interface
// that is passed to be used upstream while loading the existing the wallet in the
// shared wallet implemenation.
// Immediately loading the existing wallet is complete, the function to safely
// cancel network sync is set. There after returning the loaded wallet's interface.
func LoadExisting(w *sharedW.Wallet, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.LTCChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	// If a wallet doesn't contain discovered accounts, its previous recovery wasn't
	// successful and therefore it should try the recovery again till it successfully
	// completes.
	ldr := initWalletLoader(chainParams, params.RootDir)
	ltcWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	err = ltcWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	if err := ltcWallet.prepareChain(); err != nil {
		return nil, err
	}

	ltcWallet.SetNetworkCancelCallback(ltcWallet.SafelyCancelSync)

	return ltcWallet, nil
}

// SafelyCancelSync shuts down all the upstream processes. If not explicity
// deleting a wallet use asset.CancelSync() instead.
func (asset *Asset) SafelyCancelSync() {
	if asset.IsConnectedToNetwork() {
		// Chain is either syncing or is synced.
		asset.CancelSync()
	}

	loadWallet := asset.Internal().LTC
	if loadWallet != nil && loadWallet.Database() != nil {
		// Close the upstream loader database connection to disable the wallet
		// recovery if it is running in the background.
		if err := loadWallet.Database().Close(); err != nil {
			log.Errorf("closing upstream db failed: %v", err)
		}
	}

	asset.syncData.wg.Wait()

	// Stop the goroutines left active to manage the wallet functionalities that
	// don't require activation of sync i.e. wallet rename, password update etc.
	if loadWallet != nil {
		if loadWallet.ShuttingDown() {
			return
		}

		loadWallet.Stop()
		loadWallet.WaitForShutdown()
	}
}

// IsSynced returns true if the wallet is synced.
func (asset *Asset) IsSynced() bool {
	log.Error(utils.ErrLTCMethodNotImplemented("IsSynced"))
	return false
}

// IsWaiting returns true if the wallet is waiting for headers.
func (asset *Asset) IsWaiting() bool {
	log.Error(utils.ErrLTCMethodNotImplemented("IsWaiting"))
	return false
}

// IsSyncing returns true if the wallet is syncing.
func (asset *Asset) IsSyncing() bool {
	log.Error(utils.ErrLTCMethodNotImplemented("IsSyncing"))
	return false
}

// IsSyncShuttingDown returns true if the wallet is shutting down.
func (asset *Asset) IsSyncShuttingDown() bool {
	log.Error(utils.ErrLTCMethodNotImplemented("IsSyncShuttingDown"))
	return false
}

// ConnectedPeers returns the number of connected peers.
func (asset *Asset) ConnectedPeers() int32 {
	// Calling CS.ConnectedCount() before the first sync is
	// Performed will freeze the application, because the function never return.
	// Return 0 when not connected to bitcoin network as work around.
	if !asset.IsConnectedToNetwork() {
		return -1
	}
	return asset.chainClient.CS.ConnectedCount()
}

// IsConnectedToNetwork returns true if the wallet is connected to the network.
func (asset *Asset) IsConnectedToNetwork() bool {
	return asset.IsConnectedToLitecoinNetwork()
}

// GetBestBlock returns the best block.
func (asset *Asset) GetBestBlock() *sharedW.BlockInfo {
	block, err := asset.chainClient.CS.BestBlock()
	if err != nil {
		log.Error("GetBestBlock hash for LTC failed, Err: ", err)
		return sharedW.InvalidBlock
	}

	return &sharedW.BlockInfo{
		Height:    block.Height,
		Timestamp: block.Timestamp.Unix(),
	}
}

// GetBestBlockHeight returns the best block height.
func (asset *Asset) GetBestBlockHeight() int32 {
	return asset.GetBestBlock().Height
}

// GetBestBlockTimeStamp returns the best block timestamp.
func (asset *Asset) GetBestBlockTimeStamp() int64 {
	return asset.GetBestBlock().Timestamp
}

// GetBlockHeight returns the block height for the given block hash.
func (asset *Asset) GetBlockHeight(hash chainhash.Hash) (int32, error) {
	height, err := asset.chainClient.GetBlockHeight(&hash)
	if err != nil {
		log.Warn("GetBlockHeight for LTC failed, Err: %v", err)
		return -1, err
	}
	return height, nil
}

// GetBlockHash returns the block hash for the given block height.
func (asset *Asset) GetBlockHash(height int64) (*chainhash.Hash, error) {
	blockhash, err := asset.chainClient.GetBlockHash(height)
	if err != nil {
		log.Warn("GetBlockHash for LTC failed, Err: %v", err)
		return nil, err
	}

	return blockhash, nil
}

// SignMessage signs a message with the private key associated with an address.
func (asset *Asset) SignMessage(passphrase, address, message string) ([]byte, error) {
	return nil, utils.ErrLTCMethodNotImplemented("SignMessage")
}

// VerifyMessage verifies a signed message.
func (asset *Asset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	return false, utils.ErrLTCMethodNotImplemented("VerifyMessage")
}

// RemovePeers removes all peers from the wallet.
func (asset *Asset) RemovePeers() {
	log.Error(utils.ErrLTCMethodNotImplemented("RemovePeers"))
}

// SetSpecificPeer sets a specific peer to connect to.
func (asset *Asset) SetSpecificPeer(address string) {
	log.Error(utils.ErrLTCMethodNotImplemented("SetSpecificPeer"))
}

// GetExtendedPubKey returns the extended public key of the given account,
// to do that it calls LTCwallet's AccountProperties method, using KeyScopeBIP0084
// and the account number. On failure it returns error.
func (asset *Asset) GetExtendedPubKey(account int32) (string, error) {
	loadedAsset := asset.Internal().LTC
	if loadedAsset == nil {
		return "", utils.ErrLTCNotInitialized
	}

	extendedPublicKey, err := loadedAsset.AccountProperties(asset.GetScope(), uint32(account))
	if err != nil {
		return "", err
	}
	return extendedPublicKey.AccountPubKey.String(), nil
}

// AccountXPubMatches checks if the xpub of the provided account matches the
// provided xpub.
func (asset *Asset) AccountXPubMatches(account uint32, xPub string) (bool, error) {
	acctXPubKey, err := asset.Internal().LTC.AccountProperties(asset.GetScope(), account)
	if err != nil {
		return false, err
	}

	return acctXPubKey.AccountPubKey.String() == xPub, nil
}
