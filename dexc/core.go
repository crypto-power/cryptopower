package dexc

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/dex"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
)

// DEXClient represents the Decred DEX client and embeds *core.Core.
type DEXClient struct {
	*core.Core

	shutdownChan    <-chan struct{}
	bondBufMtx      sync.Mutex
	bondBufferCache map[uint32]valStamp
	log             dex.Logger
}

func (dc *DEXClient) SetDEXPassword(pw []byte, seed []byte) error {
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
func (dc *DEXClient) BondsFeeBuffer(assetID uint32) (feeBuffer uint64) {
	const expiry = 45 * time.Minute
	dc.bondBufMtx.Lock()
	defer dc.bondBufMtx.Unlock()
	if buf, ok := dc.bondBufferCache[assetID]; ok && time.Since(buf.stamp) < expiry {
		dc.log.Tracef("Using cached bond fee buffer (%v old): %d",
			time.Since(buf.stamp), feeBuffer)
		return buf.val
	}

	feeBuffer, err := dc.Core.BondsFeeBuffer(assetID)
	if err != nil {
		dc.log.Error("Error fetching bond fee buffer: %v", err)
		return 0
	}

	dc.log.Tracef("Obtained fresh bond fee buffer: %d", feeBuffer)
	dc.bondBufferCache[assetID] = valStamp{feeBuffer, time.Now()}

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
		Core:            clientCore,
		bondBufferCache: make(map[uint32]valStamp),
		shutdownChan:    shutdownChan,
		log:             logger,
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
