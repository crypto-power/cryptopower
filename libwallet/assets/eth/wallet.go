package eth

import (
	"context"
	"path/filepath"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/eth"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	// executionClient defines the name of ethereum client that manages RPC API.
	executionClient = "geth"
)

// Asset confirmation that ETH implements that shared assets interface.
var _ sharedW.Asset = (*Asset)(nil)

// Asset implements the sharedW.Asset interface. It also implements the
// sharedW.AssetsManagerDB interface. This is done to allow the Asset to be
// used as a db interface for the AssetsManager.
type Asset struct {
	*sharedW.Wallet

	chainParams *params.ChainConfig
	stack       *node.Node

	cancelSync context.CancelFunc
	syncCtx    context.Context
	clientCtx  context.Context

	backend *les.LesApiBackend

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

func initWalletLoader(chainParams *params.ChainConfig, dbDirPath string) loader.AssetLoader {
	dirName := ""
	// testnet datadir takes a special structure differenting "sepolia" , "rinkeby"
	// and "georli" data directory.
	if utils.ToNetworkType(params.NetworkNames[chainParams.ChainID.String()]) != utils.Mainnet {
		dirName = utils.NetDir(utils.ETHWalletAsset, utils.Testnet)
	}

	conf := &eth.LoaderConf{
		DBDirPath: filepath.Join(dbDirPath, dirName),
	}

	return eth.NewLoader(conf)
}

// CreateNewWallet creates a new wallet for the ETH asset. Ethereum uses an
// account based funds management method, therefore a wallet in this context
// represent an single account on ethereum.
func CreateNewWallet(pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.ETHChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)

	w, err := sharedW.CreateNewWallet(pass, ldr, params, utils.ETHWalletAsset)
	if err != nil {
		return nil, err
	}

	ctx, _ := w.ShutdownContextWithCancel()

	ethWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		clientCtx:   ctx,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	ethWallet.SetNetworkCancelCallback(ethWallet.SafelyCancelSync)

	return ethWallet, nil
}

// RestoreWallet accepts the seed, wallet pass information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the ETH asset. It then generates the ETH loader interface
// that is passed to be used upstream while restoring the wallet in the
// shared wallet implemenation.
// Immediately wallet restore is complete, the function to safely cancel network sync
// is set. There after returning the restored wallet's interface.
func RestoreWallet(seedMnemonic string, pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	return nil, utils.ErrETHMethodNotImplemented("RestoreWallet")
}

// LoadExisting accepts the stored shared wallet information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the ETH asset. It then generates the ETH loader interface
// that is passed to be used upstream while loading the existing the wallet in the
// shared wallet implemenation.
// Immediately loading the existing wallet is complete, the function to safely
// cancel network sync is set. There after returning the loaded wallet's interface.
func LoadExisting(w *sharedW.Wallet, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.ETHChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	ctx, _ := w.ShutdownContextWithCancel()
	ethWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		clientCtx:   ctx,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	err = ethWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	ethWallet.SetNetworkCancelCallback(ethWallet.SafelyCancelSync)

	return ethWallet, nil
}

// SafelyCancelSync is used to controllably disable network activity.
func (asset *Asset) SafelyCancelSync() {
	if asset.IsConnectedToEthereumNetwork() {
		asset.CancelSync()
	}
}

func (asset *Asset) IsWaiting() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsWaiting"))
	return false
}

// IsConnectedToNetwork returns true if the wallet is connected to the network.
func (asset *Asset) IsConnectedToNetwork() bool {
	return asset.IsConnectedToEthereumNetwork()
}

// ToAmount returns the AssetAmount interface implementation using the provided
// amount parameter.
func (asset *Asset) ToAmount(v int64) sharedW.AssetAmount {
	return Amount(v)
}

func (asset *Asset) GetBestBlock() *sharedW.BlockInfo {
	if asset.backend == nil {
		return sharedW.InvalidBlock
	}

	blockNumber := rpc.BlockNumber(asset.GetBestBlockHeight())
	block, err := asset.backend.BlockByNumber(asset.clientCtx, blockNumber)
	if err != nil {
		log.Errorf("invalid best block found: %v", err)
		return sharedW.InvalidBlock
	}

	return &sharedW.BlockInfo{
		Height:    int32(block.NumberU64()),
		Timestamp: int64(block.Header().Time),
	}
}

func (asset *Asset) GetBestBlockHeight() int32 {
	if asset.backend == nil {
		return sharedW.InvalidBlock.Height
	}

	header := asset.backend.CurrentHeader()
	return int32(header.Number.Uint64())
}

func (asset *Asset) GetBestBlockTimeStamp() int64 {
	bestblock := asset.GetBestBlock()
	if bestblock == nil {
		return sharedW.InvalidBlock.Timestamp
	}
	return bestblock.Timestamp
}

func (asset *Asset) SignMessage(passphrase, address, message string) ([]byte, error) {
	return nil, utils.ErrETHMethodNotImplemented("SignMessage")
}

func (asset *Asset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	return false, utils.ErrETHMethodNotImplemented("VerifyMessage")
}
