package dexc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"decred.org/dcrdex/client/asset"
	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/dex"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
)

const (
	// CustomDexDcrWalletType is a keyword that identifies a custom dcr wallet
	// used by the DEX client.
	CustomDexDcrWalletType = "cryptopowerwallet"
	// DexDcrWalletIDConfigKey is the key that holds the wallet ID value in the
	// settings map used to connect an existing dcr wallet to the DEX client.
	DexDcrWalletIDConfigKey = "walletid"
	// DexDcrWalletAccountNameConfigKey is the key that holds the wallet account values
	// in the settings map used to connect an existing dcr wallet to the DEX client.
	DexDcrWalletAccountNameConfigKey = "accountname"
)

// DEXClient represents the Decred DEX client and embeds *core.Core.
type DEXClient struct {
	*core.Core

	shutdownChan    <-chan struct{}
	bondBufferCache sync.Map
	log             dex.Logger
}

func (dc *DEXClient) InitWithPassword(pw []byte, seed []byte) error {
	return dc.InitializeClient(pw, seed)
}

func (dc *DEXClient) IsDEXPasswordSet() bool {
	return dc.IsInitialized()
}

// WaitForShutdown returns a chan that will be closed if core exits.
func (dc *DEXClient) WaitForShutdown() <-chan struct{} {
	return dc.shutdownChan
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

	buf, ok := dc.bondBufferCache.Load(assetID)
	var cachedFeeBuffer valStamp
	if ok {
		cachedFeeBuffer = buf.(valStamp)
	}

	if ok && time.Since(cachedFeeBuffer.stamp) > expiry {
		dc.log.Tracef("Using cached bond fee buffer (%v old): %d", time.Since(cachedFeeBuffer.stamp), cachedFeeBuffer.val)
		return cachedFeeBuffer.val
	}

	feeBuffer, err := dc.Core.BondsFeeBuffer(assetID)
	if err != nil {
		dc.log.Error("Error fetching bond fee buffer: %v", err)
		return 0
	}

	dc.log.Tracef("Obtained fresh bond fee buffer: %d", feeBuffer)
	dc.bondBufferCache.Store(assetID, valStamp{feeBuffer, time.Now()})
	return feeBuffer
}

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
		Language:           lang,
		UnlockCoinsOnLogin: false, // TODO: Make configurable.
	}

	clientCore, err := core.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dex core: %w", err)
	}

	shutdownChan := make(chan struct{})
	dc := &DEXClient{
		Core:         clientCore,
		shutdownChan: shutdownChan,
		log:          logger,
	}

	// Use a goroutine to start dex core as it'll block until dex core exits.
	go func() {
		dc.Run(ctx)
		close(shutdownChan)
		dc.Core = nil
		logCloser()
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
// NOTE: Before connecting a dcr wallet, first call mw.UseDcrWalletForDex to
// configure the dcr ExchangeWallet to use a custom wallet instead of the
// default rpc wallet.
func (dc *DEXClient) AddWallet(assetID uint32, settings map[string]string, appPW, walletPW []byte) error {
	assetInfo, err := asset.Info(assetID)
	if err != nil {
		return fmt.Errorf("unsupported asset %d", assetID)
	}

	validWalletType := false
	config := map[string]string{}
	for _, def := range assetInfo.AvailableWallets {
		if def.Type == CustomDexDcrWalletType {
			validWalletType = true
			// Start building the wallet config with default values.
			for _, option := range def.ConfigOpts {
				config[strings.ToLower(option.Key)] = fmt.Sprintf("%v", option.DefaultValue)
			}
			break
		}
	}

	if !validWalletType {
		return fmt.Errorf("invalid type %q for %s wallet", CustomDexDcrWalletType, assetInfo.Name)
	}

	// User-provided settings should override defaults.
	for k, v := range settings {
		config[k] = v
	}

	return dc.Core.CreateWallet(appPW, walletPW, &core.WalletForm{
		AssetID: assetID,
		Config:  config,
		Type:    CustomDexDcrWalletType,
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
