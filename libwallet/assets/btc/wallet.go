package btc

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/chain"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
)

// BTCAsset confirm that BTC implements that shared assets interface.
var _ sharedW.Asset = (*BTCAsset)(nil)

type BTCAsset struct {
	*sharedW.Wallet

	chainClient    *chain.NeutrinoClient
	chainParams    *chaincfg.Params
	TxAuthoredInfo *TxAuthor

	cancelSync context.CancelFunc
	syncCtx    context.Context

	// This field has been added to cache the expensive call to GetTransactions.
	// If the best block height hasn't changed there is no need to make another
	// expensive GetTransactions call.
	txs txCache

	// This fields helps to prevent unnecessary API calls if a new block hasn't
	// been introduced.
	fees feeEstimateCache

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
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
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
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
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
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
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

	// If a wallet doesn't contain discovered accounts, its previous recovery wasn't
	// successful and therefore it should try the recovery again till it successfully
	// completes.
	ldr := initWalletLoader(chainParams, params.RootDir)
	btcWallet := &BTCAsset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
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

// SafelyCancelSync shuts down all the upstream processes. If not explicity
// deleting a wallet use asset.CancelSync() instead.
func (asset *BTCAsset) SafelyCancelSync() {
	if asset.IsConnectedToNetwork() {
		// Before exiting, attempt to update the birthday block incase of a
		// premature exit. Premature exit happens when the chain is not synced
		// or chain rescan is running.
		if !asset.IsSynced() || asset.IsRescanning() {
			asset.updateAssetBirthday()
		}

		// Chain is either syncing or is synced.
		asset.CancelSync()
	}

	// Stop the goroutines left active to manage the wallet functionalities that
	// don't require activation of sync i.e. wallet rename, password update etc.
	loadWallet := asset.Internal().BTC
	if loadWallet != nil {
		if loadWallet.ShuttingDown() {
			return
		}

		loadWallet.Stop()
		loadWallet.WaitForShutdown()
	}
}

// Methods added below satisfy the shared asset interface. Each should be
// implemented fully to avoid panic if invoked.
func (asset *BTCAsset) IsSynced() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.synced
}

func (asset *BTCAsset) IsWaiting() bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("IsWaiting"))
	return false
}

func (asset *BTCAsset) IsSyncing() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.syncing
}

func (asset *BTCAsset) IsSyncShuttingDown() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.isSyncShuttingDown
}

func (asset *BTCAsset) ConnectedPeers() int32 {
	// Calling CS.ConnectedCount() before the first sync is
	// Performed will freeze the application, because the function never return.
	// Return 0 when not connected to bitcoin network as work around.
	if !asset.IsConnectedToNetwork() {
		return -1
	}
	return asset.chainClient.CS.ConnectedCount()
}

func (asset *BTCAsset) IsConnectedToNetwork() bool {
	return asset.IsConnectedToBitcoinNetwork()
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
	err := asset.UnlockWallet(passphrase)
	if err != nil {
		return nil, err
	}
	defer asset.LockWallet()

	addr, err := decodeAddress(address, asset.chainParams)
	if err != nil {
		return nil, err
	}

	privKey, err := asset.Internal().BTC.PrivKeyForAddress(addr)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
	if err != nil {
		return nil, err
	}
	err = wire.WriteVarString(&buf, 0, message)
	if err != nil {
		return nil, err
	}

	messageHash := chainhash.DoubleHashB(buf.Bytes())
	sigbytes, err := ecdsa.SignCompact(privKey, messageHash, true)
	if err != nil {
		return nil, err
	}

	return sigbytes, nil
}

func (asset *BTCAsset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	addr, err := decodeAddress(address, asset.chainParams)
	if err != nil {
		return false, err
	}

	// decode base64 signature
	sig, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, err
	}

	// Validate the signature - this just shows that it was valid at all.
	// we will compare it with the key next.
	var buf bytes.Buffer
	err = wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
	if err != nil {
		return false, nil
	}
	err = wire.WriteVarString(&buf, 0, message)
	if err != nil {
		return false, nil
	}
	expectedMessageHash := chainhash.DoubleHashB(buf.Bytes())
	pk, wasCompressed, err := ecdsa.RecoverCompact(sig, expectedMessageHash)
	if err != nil {
		return false, err
	}

	var serializedPubKey []byte
	if wasCompressed {
		serializedPubKey = pk.SerializeCompressed()
	} else {
		serializedPubKey = pk.SerializeUncompressed()
	}
	// Verify that the signed-by address matches the given address
	switch checkAddr := addr.(type) {
	case *btcutil.AddressPubKeyHash:
		return bytes.Equal(btcutil.Hash160(serializedPubKey), checkAddr.Hash160()[:]), nil
	case *btcutil.AddressPubKey:
		return string(serializedPubKey) == checkAddr.String(), nil
	case *btcutil.AddressWitnessPubKeyHash:
		byteEq := bytes.Compare(btcutil.Hash160(serializedPubKey), checkAddr.Hash160()[:])
		return byteEq == 0, nil
	default:
		return false, errors.New("address type not supported")
	}
}

func (asset *BTCAsset) RemovePeers() {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	go func() {
		err := asset.reloadChainService()
		if err != nil {
			log.Error(err)
		}
	}()
}

func (asset *BTCAsset) SetSpecificPeer(address string) {
	knownAddr := asset.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")

	// Prevent setting same address twice
	if !strings.Contains(address, ";") && !strings.Contains(knownAddr, address) {
		knownAddr += ";" + address
	}

	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, knownAddr)
	go func() {
		err := asset.reloadChainService()
		if err != nil {
			log.Error(err)
		}
	}()
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
