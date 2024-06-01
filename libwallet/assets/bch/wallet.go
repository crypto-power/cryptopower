package bch

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"decred.org/dcrwallet/v3/errors"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/internal/loader"
	"github.com/crypto-power/cryptopower/libwallet/internal/loader/bch"
	"github.com/crypto-power/cryptopower/libwallet/utils"

	// btcneutrino "github.com/lightninglabs/neutrino"
	neutrino "github.com/dcrlabs/neutrino-bch"
	// labschain "github.com/dcrlabs/neutrino-bch/chain"
	// "github.com/lightninglabs/neutrino/headerfs"
	// "github.com/bchsuite/bchd/btcec/v2/ecdsa"
	"github.com/gcash/bchd/bchec"
	// "github.com/btcsuite/btcd/btcutil"
	"github.com/dcrlabs/bchwallet/chain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"

	// btcchainhash "github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/dcrlabs/bchwallet/waddrmgr"
	// "github.com/btcsuite/btcd/btcutil"
	"github.com/gcash/bchutil"
	// "github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/dcrlabs/bchwallet/wallet"
	"github.com/gcash/bchutil/gcs"

	// btcwire "github.com/btcsuite/btcd/wire"
	_ "github.com/dcrlabs/bchwallet/walletdb/bdb" // bdb init() registers a driver
	"github.com/gcash/bchd/wire"
)

// Asset confirm that BCH implements that shared assets interface.
var _ sharedW.Asset = (*Asset)(nil)

// Asset is a wrapper around the bchWallet.Wallet struct.
// It implements the sharedW.Asset interface.
// It also implements the sharedW.AssetsManagerDB interface.
// This is done to allow the Asset to be used as a db interface
// for the AssetsManager.
type Asset struct {
	*sharedW.Wallet

	chainClient    *chain.NeutrinoClient
	chainParams    *chaincfg.Params
	TxAuthoredInfo *TxAuthor

	cancelSync context.CancelFunc
	syncCtx    context.Context

	// variables help manage node level tcp connections.
	dailerCtx    context.Context
	dailerCancel context.CancelFunc

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
	txAndBlockNotificationListeners map[string]*sharedW.TxAndBlockNotificationListener
	blocksRescanProgressListener    *sharedW.BlocksRescanProgressListener
}

const (
	recoverWindow    = 200 // If recoveryWindow is set to 0, there will be invalid block filter error.
	defaultDBTimeout = time.Duration(100)
)

// neutrinoService is satisfied by *neutrino.ChainService.
type neutrinoService interface {
	GetBlockHash(int64) (*chainhash.Hash, error)
	BestBlock() (*waddrmgr.BlockStamp, error)
	Peers() []*neutrino.ServerPeer
	GetBlockHeight(hash *chainhash.Hash) (int32, error)
	GetBlockHeader(*chainhash.Hash) (*wire.BlockHeader, error)
	GetCFilter(blockHash chainhash.Hash, filterType wire.FilterType, options ...neutrino.QueryOption) (*gcs.Filter, error)
	GetBlock(blockHash chainhash.Hash, options ...neutrino.QueryOption) (*bchutil.Block, error)
	Stop() error
}

var _ neutrinoService = (*neutrino.ChainService)(nil)

// CreateNewWallet creates a new wallet for the BCH asset.
func CreateNewWallet(pass *sharedW.AuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BCHChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.CreateNewWallet(pass, ldr, params, utils.BCHWalletAsset)
	if err != nil {
		return nil, err
	}

	bchWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]*sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]*sharedW.TxAndBlockNotificationListener),
	}

	if err := bchWallet.prepareChain(); err != nil {
		return nil, err
	}

	bchWallet.SetNetworkCancelCallback(bchWallet.SafelyCancelSync)

	return bchWallet, nil
}

func initWalletLoader(chainParams *chaincfg.Params, dbDirPath string) loader.AssetLoader {
	dirName := ""
	// testnet datadir takes a special structure to differentiate "testnet4" and "testnet3"
	// data directory.
	if utils.ToNetworkType(chainParams.Net.String()) == utils.Testnet {
		dirName = utils.NetDir(utils.BCHWalletAsset, utils.Testnet)
	}

	conf := &bch.LoaderConf{
		ChainParams:      walletParams(chainParams),
		DBDirPath:        filepath.Join(dbDirPath, dirName),
		DefaultDBTimeout: defaultDBTimeout,
		RecoveryWin:      recoverWindow,
	}

	return bch.NewLoader(conf)
}

// walletParams works around a bug in bchWallet that doesn't recognize
// wire.TestNet4 in (*ScopedKeyManager).cloneKeyWithVersion which is called from
// AccountProperties. Only do this for the *wallet.Wallet, not the
// *neutrino.ChainService.
func walletParams(chainParams *chaincfg.Params) *chaincfg.Params {
	if chainParams.Name != chaincfg.TestNet4Params.Name {
		return chainParams
	}
	spoofParams := *chainParams
	spoofParams.Net = wire.TestNet3
	return &spoofParams
}

// CreateWatchOnlyWallet accepts the wallet name, extended public key and the
// init parameters to create a watch only wallet for the BCH asset.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BCH asset. It then generates the BCH loader interface
// that is passed to be used upstream while creating the watch only wallet in the
// shared wallet implemenation.
// Immediately a watch only wallet is created, the function to safely cancel network sync
// is set. There after returning the watch only wallet's interface.
func CreateWatchOnlyWallet(walletName, extendedPublicKey string, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BCHChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.CreateWatchOnlyWallet(walletName, extendedPublicKey,
		ldr, params, utils.BCHWalletAsset)
	if err != nil {
		return nil, err
	}

	bchWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]*sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]*sharedW.TxAndBlockNotificationListener),
	}

	if err := bchWallet.prepareChain(); err != nil {
		return nil, err
	}

	bchWallet.SetNetworkCancelCallback(bchWallet.SafelyCancelSync)

	return bchWallet, nil
}

// RestoreWallet accepts the seed, wallet pass information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BCH asset. It then generates the BCH loader interface
// that is passed to be used upstream while restoring the wallet in the
// shared wallet implemenation.
// Immediately wallet restore is complete, the function to safely cancel network sync
// is set. There after returning the restored wallet's interface.
func RestoreWallet(seedMnemonic string, pass *sharedW.AuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BCHChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir)
	w, err := sharedW.RestoreWallet(seedMnemonic, pass, ldr, params, utils.BCHWalletAsset)
	if err != nil {
		return nil, err
	}

	bchWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]*sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]*sharedW.TxAndBlockNotificationListener),
	}

	if err := bchWallet.prepareChain(); err != nil {
		return nil, err
	}

	bchWallet.SetNetworkCancelCallback(bchWallet.SafelyCancelSync)

	return bchWallet, nil
}

// LoadExisting accepts the stored shared wallet information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the BCH asset. It then generates the BCH loader interface
// that is passed to be used upstream while loading the existing the wallet in the
// shared wallet implemenation.
// Immediately loading the existing wallet is complete, the function to safely
// cancel network sync is set. There after returning the loaded wallet's interface.
func LoadExisting(w *sharedW.Wallet, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.BCHChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	// If a wallet doesn't contain discovered accounts, its previous recovery wasn't
	// successful and therefore it should try the recovery again till it successfully
	// completes.
	ldr := initWalletLoader(chainParams, params.RootDir)
	bchWallet := &Asset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]*sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]*sharedW.TxAndBlockNotificationListener),
	}

	err = bchWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	if err := bchWallet.prepareChain(); err != nil {
		return nil, err
	}

	bchWallet.SetNetworkCancelCallback(bchWallet.SafelyCancelSync)

	return bchWallet, nil
}

// SafelyCancelSync shuts down all the upstream processes. If not explicitly
// deleting a wallet use asset.CancelSync() instead.
func (asset *Asset) SafelyCancelSync() {
	if asset.IsConnectedToNetwork() {
		// Chain is either syncing or is synced.
		asset.CancelSync()
	}

	loadWallet := asset.Internal().BCH
	if asset.WalletOpened() && loadWallet.Database() != nil {
		// Close the upstream loader database connection.
		if err := loadWallet.Database().Close(); err != nil {
			log.Errorf("closing upstream db failed: %v", err)
		}
	}

	asset.syncData.wg.Wait()

	// Stop the goroutines left active to manage the wallet functionalities that
	// don't require activation of sync i.e. wallet rename, password update etc.
	if asset.WalletOpened() {
		if loadWallet.ShuttingDown() {
			return
		}

		loadWallet.Stop()
		loadWallet.WaitForShutdown()
	}
}

func (asset *Asset) NeutrinoClient() *chain.NeutrinoClient {
	return asset.chainClient
}

// IsSynced returns true if the wallet is synced.
func (asset *Asset) IsSynced() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.synced
}

// IsWaiting returns true if the wallet is waiting for headers.
func (asset *Asset) IsWaiting() bool {
	log.Error(utils.ErrBCHMethodNotImplemented("IsWaiting"))
	return false
}

// IsSyncing returns true if the wallet is syncing.
func (asset *Asset) IsSyncing() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.syncing
}

// IsSyncShuttingDown returns true if the wallet is shutting down.
func (asset *Asset) IsSyncShuttingDown() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.isSyncShuttingDown
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
	return asset.IsConnectedToBitcoinCashNetwork()
}

// GetBestBlock returns the best block.
func (asset *Asset) GetBestBlock() *sharedW.BlockInfo {
	block, err := asset.chainClient.CS.BestBlock()
	if err != nil {
		log.Error("GetBestBlock hash for BCH failed, Err: ", err)
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
		log.Warn("GetBlockHeight for BCH failed, Err: %v", err)
		return -1, err
	}
	return height, nil
}

// GetBlockHash returns the block hash for the given block height.
func (asset *Asset) GetBlockHash(height int64) (*chainhash.Hash, error) {
	blockhash, err := asset.chainClient.GetBlockHash(height)
	if err != nil {
		log.Warn("GetBlockHash for BCH failed, Err: %v", err)
		return nil, err
	}

	return blockhash, nil
}

// SignMessage signs a message with the private key associated with an address.
func (asset *Asset) SignMessage(passphrase, address, message string) ([]byte, error) {
	err := asset.UnlockWallet(passphrase)
	if err != nil {
		return nil, err
	}
	defer asset.LockWallet()

	addr, err := decodeAddress(address, asset.chainParams)
	if err != nil {
		return nil, err
	}

	privKey, err := asset.Internal().BCH.PrivKeyForAddress(addr)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = wire.WriteVarString(&buf, 0, "Litecoin Signed Message:\n")
	if err != nil {
		return nil, err
	}
	err = wire.WriteVarString(&buf, 0, message)
	if err != nil {
		return nil, err
	}

	messageHash := chainhash.DoubleHashB(buf.Bytes())
	sigbytes, err := bchec.SignCompact(nil, privKey, messageHash, true) // TODO: check if nil is correct
	if err != nil {
		return nil, err
	}

	return sigbytes, nil
}

// VerifyMessage verifies a signed message.
func (asset *Asset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
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
	err = wire.WriteVarString(&buf, 0, "Bitcoin Cash Signed Message:\n")
	if err != nil {
		return false, nil
	}
	err = wire.WriteVarString(&buf, 0, message)
	if err != nil {
		return false, nil
	}
	expectedMessageHash := chainhash.DoubleHashB(buf.Bytes())
	pk, wasCompressed, err := bchec.RecoverCompact(nil, sig, expectedMessageHash) // TODO: check if nil is correct
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
	case *bchutil.AddressPubKeyHash:
		return bytes.Equal(bchutil.Hash160(serializedPubKey), checkAddr.Hash160()[:]), nil
	case *bchutil.AddressPubKey:
		return string(serializedPubKey) == checkAddr.String(), nil
	// case *bchutil.AddressWitnessPubKeyHash:
	// 	byteEq := bytes.Compare(bchutil.Hash160(serializedPubKey), checkAddr.Hash160()[:])
	// 	return byteEq == 0, nil
	default:
		return false, errors.New("address type not supported")
	}
}

// RemovePeers removes all peers from the wallet.
func (asset *Asset) RemovePeers() {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	go func() {
		err := asset.reloadChainService()
		if err != nil {
			log.Error(err)
		}
	}()
}

// SetSpecificPeer sets a specific peer to connect to.
func (asset *Asset) SetSpecificPeer(addresses string) {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, addresses)
	go func() {
		err := asset.reloadChainService()
		if err != nil {
			log.Error(err)
		}
	}()
}

// GetExtendedPubKey returns the extended public key of the given account,
// to do that it calls bchWallet's AccountProperties method, using KeyScopeBIP0084
// and the account number. On failure it returns error.
func (asset *Asset) GetExtendedPubKey(account int32) (string, error) {
	loadedAsset := asset.Internal().BCH
	if loadedAsset == nil {
		return "", utils.ErrBCHNotInitialized
	}

	// extendedPublicKey, err := loadedAsset.AccountProperties(GetScope(), uint32(account))
	// if err != nil {
	// 	return "", err
	// }
	return /* extendedPublicKey.AccountPubKey.String() */ "", nil
}

// AccountXPubMatches checks if the xpub of the provided account matches the
// provided xpub.
func (asset *Asset) AccountXPubMatches(account uint32, xPub string) (bool, error) {
	// acctXPubKey, err := asset.Internal().BCH.AccountProperties(GetScope(), account)
	// if err != nil {
	// 	return false, err
	// }

	return /* acctXPubKey.AccountPubKey.String()*/ "" == xPub, nil
}

func decodeAddress(s string, params *chaincfg.Params) (bchutil.Address, error) {
	addr, err := bchutil.DecodeAddress(s, params)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: decode failed with %#q", s, err)
	}
	if !addr.IsForNet(params) {
		return nil, fmt.Errorf("invalid address %q: not intended for use on %s",
			addr, params.Name)
	}
	return addr, nil
}

// GetWalletBalance returns the total balance across all accounts.
func (asset *Asset) GetWalletBalance() (*sharedW.Balance, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrBCHNotInitialized
	}

	accountsResult, err := asset.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	var totalBalance, totalSpendable, totalImmatureReward, totalLocked int64
	for _, acc := range accountsResult.Accounts {
		totalBalance += acc.Balance.Total.ToInt()
		totalSpendable += acc.Balance.Spendable.ToInt()
		totalImmatureReward += acc.Balance.ImmatureReward.ToInt()
		totalLocked += acc.Balance.Locked.ToInt()
	}

	return &sharedW.Balance{
		Total:          Amount(totalBalance),
		Spendable:      Amount(totalSpendable),
		ImmatureReward: Amount(totalImmatureReward),
		Locked:         Amount(totalLocked),
	}, nil
}

// secretSource is used to locate keys and redemption scripts while signing a
// transaction. secretSource satisfies the txauthor.SecretsSource interface.
type secretSource struct {
	w           *wallet.Wallet
	chainParams *chaincfg.Params
}

// ChainParams returns the chain parameters.
func (s *secretSource) ChainParams() *chaincfg.Params {
	return s.chainParams
}

// GetKey fetches a private key for the specified address.
func (s *secretSource) GetKey(addr bchutil.Address) (*bchec.PrivateKey, bool, error) {
	ma, err := s.w.AddressInfo(addr)
	if err != nil {
		return nil, false, err
	}

	mpka, ok := ma.(waddrmgr.ManagedPubKeyAddress)
	if !ok {
		e := fmt.Errorf("managed address type for %v is `%T` but "+
			"want waddrmgr.ManagedPubKeyAddress", addr, ma)
		return nil, false, e
	}

	privKey, err := mpka.PrivKey()
	if err != nil {
		return nil, false, err
	}

	k, _ /* pub */ := bchec.PrivKeyFromBytes(bchec.S256(), privKey.Serialize())

	return k, ma.Compressed(), nil
}

// GetScript fetches the redemption script for the specified p2sh/p2wsh address.
func (s *secretSource) GetScript(addr bchutil.Address) ([]byte, error) {
	ma, err := s.w.AddressInfo(addr)
	if err != nil {
		return nil, err
	}

	msa, ok := ma.(waddrmgr.ManagedScriptAddress)
	if !ok {
		e := fmt.Errorf("managed address type for %v is `%T` but "+
			"want waddrmgr.ManagedScriptAddress", addr, ma)
		return nil, e
	}
	return msa.Script()
}
