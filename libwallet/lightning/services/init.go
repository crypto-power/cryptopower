package services

import (
	"context"
	"sync"
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/config"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/data"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/log"
	breezservice "github.com/breez/breez/breez"
	"github.com/btcsuite/btclog"
	"google.golang.org/grpc"
)

const (
	endpointTimeout = 30
)

// API is the interface for external breez services.
type API interface {
	NewSyncNotifierClient() (breezservice.SyncNotifierClient, context.Context, context.CancelFunc)
	NewFundManager() (breezservice.FundManagerClient, context.Context, context.CancelFunc)
	NewSwapper(timeout time.Duration) (breezservice.SwapperClient, context.Context, context.CancelFunc)
	NewChannelOpenerClient() (breezservice.ChannelOpenerClient, context.Context, context.CancelFunc)
	NewPushTxNotifierClient() (breezservice.PushTxNotifierClient, context.Context, context.CancelFunc)
	LSPList() (*data.LSPList, error)
}

// Client represents the client interface to breez services
type Client struct {
	sync.Mutex
	started    int32
	stopped    int32
	cfg        *config.Config
	log        btclog.Logger
	connection *grpc.ClientConn
	lspList    *data.LSPList
}

// NewClient creates a new service client struct
func NewClient(cfg *config.Config) (*Client, error) {
	logger, err := log.GetLogger(cfg.WorkingDir, "CLIENT")
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg: cfg,
		log: logger,
	}, nil
}
