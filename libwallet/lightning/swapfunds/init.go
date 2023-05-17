package swapfunds

import (
	"sync"

	"github.com/breez/breez/config"
	"github.com/breez/breez/data"
	"github.com/breez/breez/db"
	"github.com/breez/breez/lnnode"
	breezlog "github.com/breez/breez/log"
	"github.com/breez/breez/services"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
)

type Service struct {
	started               int32
	stopped               int32
	wg                    sync.WaitGroup
	mu                    sync.Mutex
	cfg                   *config.Config
	log                   btclog.Logger
	breezDB               *db.DB
	daemonAPI             lnnode.API
	breezAPI              services.API
	chainParams           *chaincfg.Params
	reverseRoutingNode    []byte
	sendPayment           func(payreq string, amount int64, lastHopPubkey []byte, fee int64) (string, error)
	addInvoice            func(invoiceRequest *data.AddInvoiceRequest) (paymentRequest string, lspFee int64, err error)
	lspList               func() (*data.LSPList, error)
	getGlobalReceiveLimit func() (maxReceive int64, err error)
	onServiceEvent        func(data.NotificationEvent)
	quitChan              chan struct{}
}

func NewService(
	cfg *config.Config,
	breezDB *db.DB,
	breezAPI services.API,
	daemonAPI lnnode.API,
	sendPayment func(payreq string, amount int64, lastHopPubkey []byte, fee int64) (string, error),
	addInvoice func(invoiceRequest *data.AddInvoiceRequest) (paymentRequest string, lspFee int64, err error),
	lspList func() (*data.LSPList, error),
	getGlobalReceiveLimit func() (maxReceive int64, err error),
	onServiceEvent func(data.NotificationEvent)) (*Service, error) {

	logger, err := breezlog.GetLogger(cfg.WorkingDir, "FUNDS")
	if err != nil {
		return nil, err
	}
	var chainParams *chaincfg.Params
	switch cfg.Network {
	case "testnet":
		chainParams = &chaincfg.TestNet3Params
	case "simnet":
		chainParams = &chaincfg.SimNetParams
	case "mainnet":
		chainParams = &chaincfg.MainNetParams
	}

	return &Service{
		cfg:                   cfg,
		chainParams:           chainParams,
		breezDB:               breezDB,
		breezAPI:              breezAPI,
		sendPayment:           sendPayment,
		addInvoice:            addInvoice,
		lspList:               lspList,
		getGlobalReceiveLimit: getGlobalReceiveLimit,
		onServiceEvent:        onServiceEvent,
		log:                   logger,
		daemonAPI:             daemonAPI,
		quitChan:              make(chan struct{}),
	}, nil
}
