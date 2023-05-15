package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/eth"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	gethutils "github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethstats"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
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
	client      *ethclient.Client

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

// prepareChain initialize the local node responsible for p2p connections.
func (asset *Asset) prepareChain() error {
	if !asset.WalletOpened() {
		return errors.New("wallet account not loaded")
	}

	ks := asset.Internal().ETH.Keystore
	if ks == nil || len(ks.Accounts()) == 0 {
		return errors.New("no existing wallet account found")
	}

	// generates a private key using the provided hashed seed. asset.EncryptedSeed has
	// a length of 64 bytes but only 32 are required to generate an ECDSA private
	// key.
	privatekey, err := crypto.ToECDSA(asset.EncryptedSeed[:32])
	if err != nil {
		return err
	}

	bootnodes, err := utils.GetBootstrapNodes(asset.chainParams)
	if err != nil {
		return fmt.Errorf("invalid bootstrap nodes: %v", err)
	}

	// Convert the bootnodes to internal enode representations
	var enodes []*enode.Node
	for _, boot := range bootnodes {
		url, err := enode.Parse(enode.ValidSchemes, boot)
		if err == nil {
			enodes = append(enodes, url)
		} else {
			log.Error("Failed to parse bootnode URL", "url", boot, " err", err)
		}
	}

	cfg := node.DefaultConfig
	cfg.DBEngine = "leveldb" // leveldb is used instead of pebble db.
	cfg.Name = executionClient
	cfg.WSModules = append(cfg.WSModules, "eth")
	cfg.DataDir = asset.DataDir()
	cfg.P2P.PrivateKey = privatekey
	cfg.P2P.BootstrapNodesV5 = enodes
	cfg.P2P.NoDiscovery = true
	cfg.P2P.DiscoveryV5 = true
	cfg.P2P.MaxPeers = 25

	stack, err := node.New(&cfg)
	if err != nil {
		return err
	}
	asset.stack = stack

	genesis, err := utils.GetGenesis(asset.chainParams)
	if err != nil {
		return fmt.Errorf("invalid genesis block: %v", err)
	}

	// Assemble the Ethereum light client protocol
	ethcfg := ethconfig.Defaults
	ethcfg.SyncMode = downloader.LightSync
	ethcfg.GPO = ethconfig.LightClientGPO
	ethcfg.NetworkId = asset.chainParams.ChainID.Uint64()
	ethcfg.Genesis = genesis
	ethcfg.Checkpoint = params.TrustedCheckpoints[genesis.ToBlock().Hash()]
	gethutils.SetDNSDiscoveryDefaults(&ethcfg, genesis.ToBlock().Hash())

	lesBackend, err := les.New(stack, &ethcfg)
	if err != nil {
		return fmt.Errorf("failed to register the Ethereum service: %w", err)
	}

	asset.backend = lesBackend.ApiBackend

	// Assemble the ethstats monitoring and reporting service'
	stats, err := utils.GetEthStatsURL(asset.chainParams)
	if err != nil {
		return fmt.Errorf("invalid ethstat URL: %v", err)
	}

	if stats != "" {
		if err := ethstats.New(stack, lesBackend.ApiBackend, lesBackend.Engine(), stats); err != nil {
			return fmt.Errorf("ethstats connection failed: %v", err)
		}
	}

	// Boot up the client and ensure it connects to bootnodes
	if err := stack.Start(); err != nil {
		return err
	}

	for _, boot := range enodes {
		old, err := enode.Parse(enode.ValidSchemes, boot.String())
		if err == nil {
			stack.Server().AddPeer(old)
		}
	}

	// Attach to the client and retrieve and interesting metadatas
	api, err := stack.Attach()
	if err != nil {
		stack.Close()
		return fmt.Errorf("attaching the client failed: %v", err)
	}

	asset.client = ethclient.NewClient(api)
	return nil
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
	if asset.client == nil {
		return sharedW.InvalidBlock
	}

	blockNumber := big.NewInt(int64(asset.GetBestBlockHeight()))
	block, err := asset.client.BlockByNumber(asset.clientCtx, blockNumber)
	if err != nil {
		log.Errorf("invalid best block found: %v", err)
		return sharedW.InvalidBlock
	}

	return &sharedW.BlockInfo{
		Height:    int32(block.NumberU64()),
		Timestamp: block.ReceivedAt.Unix(),
	}
}

func (asset *Asset) GetBestBlockHeight() int32 {
	if asset.client == nil {
		return sharedW.InvalidBlock.Height
	}

	height, err := asset.client.BlockNumber(asset.clientCtx)
	if err != nil {
		log.Errorf("invalid best block height found: %v", err)
		return sharedW.InvalidBlock.Height
	}

	return int32(height)
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
