package eth

import (
	"errors"
	"fmt"
	"path/filepath"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/eth"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/node"
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

	ethWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
	}

	loadedWallet, _ := ldr.GetLoadedWallet()
	if err := ethWallet.prepareChain(loadedWallet.ETH.Keystore); err != nil {
		return nil, fmt.Errorf("preparing chain failed: %v", err)
	}

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

	ethWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
	}

	err = ethWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	return ethWallet, nil
}

// prepareChain initialize the local node responsible for p2p connections.
func (asset *Asset) prepareChain(ks *keystore.KeyStore) error {
	if ks == nil {
		return errors.New("Wallet account not loaded")
	}

	if len(ks.Accounts()) == 0 {
		return errors.New("no existing wallet account found")
	}

	// generates a private key using the provided hashed seed. asset.EncryptedSeed has
	// a length of 64 bytes but only 32 are required to generate an ECDSA private
	// key.
	privatekey, err := crypto.ToECDSA(asset.EncryptedSeed[:32])
	if err != nil {
		return err
	}

	cfg := node.DefaultConfig
	cfg.Name = executionClient
	cfg.WSModules = append(cfg.WSModules, "eth")
	cfg.DataDir = asset.DataDir()
	cfg.P2P.PrivateKey = privatekey

	stack, err := node.New(&cfg)
	if err != nil {
		return err
	}
	asset.stack = stack
	return nil
}

func (asset *Asset) IsWaiting() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsWaiting"))
	return false
}

func (asset *Asset) ChangePrivatePassphraseForWallet(oldPrivatePassphrase, newPrivatePassphrase string, privatePassphraseType int32) error {
	return utils.ErrETHMethodNotImplemented("ChangePrivatePassphraseForWallet")
}

func (asset *Asset) IsConnectedToNetwork() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsConnectedToNetwork"))
	return false
}

// ToAmount returns the AssetAmount interface implementation using the provided
// amount parameter.
func (asset *Asset) ToAmount(v int64) sharedW.AssetAmount {
	return Amount(v)
}

func (asset *Asset) GetBestBlock() *sharedW.BlockInfo {
	log.Error(utils.ErrETHMethodNotImplemented("GetBestBlock"))
	return sharedW.InvalidBlock
}

func (asset *Asset) GetBestBlockHeight() int32 {
	log.Error(utils.ErrETHMethodNotImplemented("GetBestBlockHeight"))
	return sharedW.InvalidBlock.Height
}

func (asset *Asset) GetBestBlockTimeStamp() int64 {
	log.Error(utils.ErrETHMethodNotImplemented("GetBestBlockTimeStamp"))
	return sharedW.InvalidBlock.Timestamp
}

func (asset *Asset) SignMessage(passphrase, address, message string) ([]byte, error) {
	return nil, utils.ErrETHMethodNotImplemented("SignMessage")
}

func (asset *Asset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	return false, utils.ErrETHMethodNotImplemented("VerifyMessage")
}
