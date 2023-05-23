package lightning

import (
	"context"
	"time"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc/verrpc"
)

// Client models a lightning service.
type LightningClient struct {
	*lndclient.GrpcLndServices
}

// Type config models a lightning configuration.
type ClientConfig struct {
	// LndAddress is the network address (host:port) of the lnd node to
	// connect to.
	LndAddress string

	// Network is the bitcoin network we expect the lnd node to operate on.
	Network lndclient.Network

	// MacaroonDir is the directory where all lnd macaroons can be found.
	// Either this, CustomMacaroonPath, or CustomMacaroonHex should be set,
	// but only one of them, depending on macaroon preferences.
	MacaroonDir string

	// CustomMacaroonPath is the full path to a custom macaroon file. Either
	// this, MacaroonDir, or CustomMacaroonHex should be set, but only one
	// of them.
	CustomMacaroonPath string

	// CustomMacaroonHex is a hexadecimal encoded macaroon string. Either
	// this, MacaroonDir, or CustomMacaroonPath should be set, but only
	// one of them.
	CustomMacaroonHex string

	// TLSPath is the path to lnd's TLS certificate file. Only this or
	// TLSData can be set, not both.
	TLSPath string

	// TLSData holds the TLS certificate data. Only this or TLSPath can be
	// set, not both.
	TLSData string

	// Insecure can be checked if we don't need to use tls, such as if
	// we're connecting to lnd via a bufconn, then we'll skip verification.
	Insecure bool

	// SystemCert specifies whether we'll fallback to a system cert pool
	// for tls.
	SystemCert bool

	// CheckVersion is the minimum version the connected lnd node needs to
	// be in order to be compatible. The node will be checked against this
	// when connecting. If no version is supplied, the default minimum
	// version will be used.
	CheckVersion *verrpc.Version

	// Dialer is an optional dial function that can be passed in if the
	// default lncfg.ClientAddressDialer should not be used.
	Dialer lndclient.DialerFunc

	// BlockUntilChainSynced denotes that the NewLndServices function should
	// block until the lnd node is fully synced to its chain backend. This
	// can take a long time if lnd was offline for a while or if the initial
	// block download is still in progress.
	BlockUntilChainSynced bool

	// BlockUntilUnlocked denotes that the NewLndServices function should
	// block until lnd is unlocked.
	BlockUntilUnlocked bool

	// CallerCtx is an optional context that can be passed if the caller
	// would like to be able to cancel the long waits involved in starting
	// up the client, such as waiting for chain sync to complete when
	// BlockUntilChainSynced is set to true, or waiting for lnd to be
	// unlocked when BlockUntilUnlocked is set to true. If a context is
	// passed in and its Done() channel sends a message, these waits will
	// be aborted. This allows a client to still be shut down properly.
	CallerCtx context.Context

	// RPCTimeout is an optional custom timeout that will be used for rpc
	// calls to lnd. If this value is not set, it will default to 30
	// seconds.
	RPCTimeout time.Duration
}

// NewService creates a new lightning service. Returns an error if
// failed initialization.
func NewClient(config *ClientConfig) (*LightningClient, error) {

	serviceConfgig := lndclient.LndServicesConfig{
		LndAddress:            config.LndAddress,
		Network:               config.Network,
		MacaroonDir:           config.MacaroonDir,
		CustomMacaroonPath:    config.CustomMacaroonPath,
		CustomMacaroonHex:     config.CustomMacaroonHex,
		TLSPath:               config.TLSPath,
		TLSData:               config.TLSData,
		Insecure:              config.Insecure,
		SystemCert:            config.SystemCert,
		CheckVersion:          config.CheckVersion,
		Dialer:                config.Dialer,
		BlockUntilChainSynced: config.BlockUntilChainSynced,
		BlockUntilUnlocked:    config.BlockUntilUnlocked,
		CallerCtx:             config.CallerCtx,
		RPCTimeout:            config.RPCTimeout,
	}

	lndServices, err := lndclient.NewLndServices(&serviceConfgig)
	if err != nil {
		return nil, err
	}

	return &LightningClient{
		GrpcLndServices: lndServices,
	}, nil
}
