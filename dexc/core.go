package dexc

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"decred.org/dcrdex/client/asset"
	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/dex"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/values/localizable"
)

const (
	// CustomDexWalletType is a keyword that identifies a custom Cryptopower
	// wallet used by the DEX client.
	CustomDexWalletType = "cryptopowerwallet"
	// WalletIDConfigKey is the key that holds the wallet ID value in the
	// settings map used to connect an existing Cryptopower wallet to the DEX
	// client.
	WalletIDConfigKey = "walletid"
	// WalletAccountNumberConfigKey is the key that holds the wallet account
	// number in the settings map used to connect an existing Cryptopower wallet
	// to the DEX client.
	WalletAccountNumberConfigKey = "accountnumber"
)

// DEXClient represents the Decred DEX client and embeds *core.Core.
type DEXClient struct {
	ctx      context.Context
	cancelFn context.CancelFunc
	dbPath   string
	*core.Core

	loggedIn        bool
	shutdownChan    <-chan struct{}
	bondBufferCache sync.Map
	log             dex.Logger
}

func (dc *DEXClient) InitWithPassword(pw []byte, seed []byte) error {
	return dc.InitializeClient(pw, seed)
}

func (dc *DEXClient) IsInitialized() bool {
	return dc != nil && dc.Core != nil
}

func (dc *DEXClient) InitializedWithPassword() bool {
	return dc.IsInitialized() && dc.Core.IsInitialized()
}

func (dc *DEXClient) IsLoggedIn() bool {
	return dc.loggedIn
}

func (dc *DEXClient) Login(pw []byte) error {
	err := dc.Core.Login(pw)
	if err != nil {
		return err
	}
	dc.loggedIn = true
	return nil
}

func (dc *DEXClient) DBPath() string {
	return dc.dbPath
}

// WaitForShutdown returns a chan that will be closed if core exits.
func (dc *DEXClient) WaitForShutdown() <-chan struct{} {
	return dc.shutdownChan
}

func (dc *DEXClient) Shutdown() {
	dc.cancelFn()
}

// WalletIDForAsset returns the wallet ID for the provided assetID. It'll return
// nil, nil if the asset does not exist in the dex client.
func (dc *DEXClient) WalletIDForAsset(assetID uint32) (*int, error) {
	if !dc.HasWallet(int32(assetID)) {
		return nil, nil
	}

	settings, err := dc.WalletSettings(assetID)
	if err != nil {
		return nil, err
	}

	walletIDStr := settings[WalletIDConfigKey]
	walletID, err := strconv.Atoi(walletIDStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing wallet ID: %w", err)
	}

	return &walletID, nil
}

type valStamp struct {
	val   uint64
	stamp time.Time
}

// BondsFeeBuffer is a caching helper for the bonds fee buffer to assist the
// frontend by stabilizing this value for up to 45 minutes from the last request
// for a given asset and because (*Core).BondsFeeBuffer returns a fresh fee
// buffer based on a current (but padded) fee rate estimate. Values for a given
// asset are cached for 45 minutes. These values are meant to provide a sensible
// but well-padded fee buffer for bond transactions now and well into the
// future, so a long expiry is appropriate.
func (dc *DEXClient) BondsFeeBuffer(assetID uint32) uint64 {
	const expiry = 45 * time.Minute

	if !dc.HasWallet(int32(assetID)) { // no configured wallet
		return 0
	}

	buf, ok := dc.bondBufferCache.Load(assetID)
	var cachedFeeBuffer valStamp
	if ok {
		cachedFeeBuffer = buf.(valStamp)
	}

	if ok && time.Since(cachedFeeBuffer.stamp) > expiry {
		dc.log.Tracef("Using cached bond fee buffer (%v old): %d", time.Since(cachedFeeBuffer.stamp), cachedFeeBuffer.val)
		return cachedFeeBuffer.val
	}

	// BondsFeeBuffer might block if we are not yet connected to the wallet that
	// provides the new fee rate. Run in a goroutine and return 0.
	go func() {
		select {
		case <-dc.ctx.Done():
			return
		default:
			feeBuffer, err := dc.Core.BondsFeeBuffer(assetID)
			if err != nil {
				dc.log.Errorf("Error fetching bond fee buffer: %v", err)
			} else {
				dc.log.Tracef("Obtained fresh bond fee buffer: %d", feeBuffer)
				dc.bondBufferCache.Store(assetID, valStamp{feeBuffer, time.Now()})
			}
		}
	}()

	return 0
}

// Start prepares and starts the DEX client.
//
// NOTE: "lang" will be changed to the default language (en) if the the DEX
// client does not have support for it.
func Start(ctx context.Context, root, lang, logDir, logLvl string, net libutils.NetworkType, maxLogZips int) (*DEXClient, error) {
	dexNet, err := parseDEXNet(net)
	if err != nil {
		return nil, fmt.Errorf("error parsing network: %w", err)
	}

	logger, logCloser, err := newDexLogger(logDir, logLvl, maxLogZips)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(root, "dexc.db")
	cfg := &core.Config{
		DBPath:             dbPath,
		Net:                dexNet,
		Logger:             logger,
		Language:           validDEXLang(lang),
		NoAutoWalletLock:   true,
		UnlockCoinsOnLogin: false, // TODO: Make configurable.
	}

	clientCore, err := core.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dex core: %w", err)
	}

	shutdownChan := make(chan struct{})
	dc := &DEXClient{
		dbPath:       dbPath,
		Core:         clientCore,
		shutdownChan: shutdownChan,
		log:          logger,
	}

	dc.ctx, dc.cancelFn = context.WithCancel(ctx)
	// Use a goroutine to start dex core as it'll block until dex core exits.
	go func() {
		dc.Run(dc.ctx)
		logCloser()
		close(shutdownChan)
		dc.cancelFn() // don't leak context
		dc.Core = nil // do this after all shutdownChan listeners must've stopped waiting
	}()

	return dc, nil
}

// HasWallet is true if a wallet has been added to the DEX client for the
// specified asset.
func (dc *DEXClient) HasWallet(assetID int32) bool {
	return dc.Core.WalletState(uint32(assetID)) != nil
}

// AddWallet attempts to connect or create the wallet with the provided details
// to the DEX client.
// NOTE: Before connecting a dcr wallet, dcr ExchangeWallet must have been
// configured to use a custom wallet. See:
// libwallet.AssetManager.PrepareDexSupportForDcrWallet() and
// libwallet.AssetManager.prepareDexSupportForBTCCloneWallets.
func (dc *DEXClient) AddWallet(assetID uint32, settings map[string]string, appPW, walletPW []byte) error {
	assetInfo, err := asset.Info(assetID)
	if err != nil {
		return fmt.Errorf("unsupported asset %d", assetID)
	}

	validWalletType := false
	config := map[string]string{}
	for _, def := range assetInfo.AvailableWallets {
		if def.Type == CustomDexWalletType {
			validWalletType = true
			// Start building the wallet config with default values.
			for _, option := range def.ConfigOpts {
				config[strings.ToLower(option.Key)] = fmt.Sprintf("%v", option.DefaultValue)
			}
			break
		}
	}

	if !validWalletType {
		return fmt.Errorf("wallet type %q is not supported for %s wallet", CustomDexWalletType, assetInfo.Name)
	}

	// User-provided settings should override defaults.
	for k, v := range settings {
		config[k] = v
	}

	return dc.Core.CreateWallet(appPW, walletPW, &core.WalletForm{
		AssetID: assetID,
		Config:  config,
		Type:    CustomDexWalletType,
	})
}

func parseDEXNet(net libutils.NetworkType) (dex.Network, error) {
	switch net {
	case libutils.Mainnet:
		return dex.Mainnet, nil
	case libutils.Testnet:
		return dex.Testnet, nil
	case libutils.Regression, libutils.Simulation:
		return dex.Simnet, nil
	default:
		return 0, fmt.Errorf("unknown network %s", net)
	}
}

// validDEXLang checks that the provided lang is supported by the DEX client. An
// empty string is returned for an invalid lang to allow DEX core use it's
// default language.
func validDEXLang(lang string) string {
	switch lang {
	case localizable.ENGLISH, localizable.CHINESE:
		return lang
	}
	return ""
}
