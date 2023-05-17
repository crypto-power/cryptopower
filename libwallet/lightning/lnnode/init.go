package lnnode

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/breez/breez/channeldbservice"
	"github.com/breez/breez/config"
	"github.com/breez/breez/db"
	breezlog "github.com/breez/breez/log"
	"github.com/breez/breez/tor"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/breezbackuprpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnrpc/signrpc"
	"github.com/lightningnetwork/lnd/lnrpc/submarineswaprpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/lightningnetwork/lnd/subscribe"
)

// API represents the lnnode exposed functions that are accessible for
// breez services to use.
// It is mainly enable the service to subscribe to various daemon events
// and get an APIClient to query the daemon directly via RPC.
type API interface {
	SubscribeEvents() (*subscribe.Client, error)
	HasActiveChannel() bool
	IsReadyForPayment() bool
	WaitReadyForPayment(timeout time.Duration) error
	NodePubkey() string
	APIClient() lnrpc.LightningClient
	SubSwapClient() submarineswaprpc.SubmarineSwapperClient
	BreezBackupClient() breezbackuprpc.BreezBackuperClient
	RouterClient() routerrpc.RouterClient
	WalletKitClient() walletrpc.WalletKitClient
	ChainNotifierClient() chainrpc.ChainNotifierClient
	InvoicesClient() invoicesrpc.InvoicesClient
	SignerClient() signrpc.SignerClient
}

// Daemon contains data regarding the lightning daemon.
type Daemon struct {
	sync.Mutex
	cfg                 *config.Config
	breezDB             *db.DB
	started             int32
	stopped             int32
	startTime           time.Time
	daemonRunning       bool
	nodePubkey          string
	wg                  sync.WaitGroup
	log                 btclog.Logger
	lightningClient     lnrpc.LightningClient
	subswapClient       submarineswaprpc.SubmarineSwapperClient
	breezBackupClient   breezbackuprpc.BreezBackuperClient
	routerClient        routerrpc.RouterClient
	walletKitClient     walletrpc.WalletKitClient
	chainNotifierClient chainrpc.ChainNotifierClient
	invoicesClient      invoicesrpc.InvoicesClient
	signerClient        signrpc.SignerClient
	ntfnServer          *subscribe.Server
	quitChan            chan struct{}
	startBeforeSync     bool
	interceptor         signal.Interceptor

	TorConfig *tor.TorConfig
}

// NewDaemon is used to create a new daemon that wraps a lightning
// network daemon.
func NewDaemon(cfg *config.Config, db *db.DB, startBeforeSync bool) (*Daemon, error) {
	logger, err := breezlog.GetLogger(cfg.WorkingDir, "DAEM")
	if err != nil {
		return nil, err
	}

	return &Daemon{
		cfg:             cfg,
		breezDB:         db,
		ntfnServer:      subscribe.NewServer(),
		log:             logger,
		startBeforeSync: startBeforeSync,
	}, nil
}

func (a *Daemon) PopulateChannelsGraph() error {
	a.log.Infof("PopulateChannelsGraph started")
	chandb, cleanup, err := channeldbservice.Get(a.cfg.WorkingDir)
	if err != nil {
		return fmt.Errorf("failed to populate channels graph %w", err)
	}
	defer cleanup()
	closedChannels, err := chandb.ChannelStateDB().FetchClosedChannels(false)
	if err != nil {
		return fmt.Errorf("failed to fetch closed graph %w", err)
	}

	graph := chandb.ChannelGraph()
	for _, c := range closedChannels {
		if !c.IsPending {
			if err := graph.DeleteChannelEdges(true, true, c.ShortChanID.ToUint64()); err != nil {
				a.log.Infof("failed to delete channel edge %v", err)
			}
		}
	}

	channels, err := chandb.ChannelStateDB().FetchAllOpenChannels()
	if err != nil {
		return fmt.Errorf("failed to fetch channels graph %w", err)
	}
	pubkeyBytes, err := hex.DecodeString(a.NodePubkey())
	if err != nil {
		return fmt.Errorf("failed to decode pubkey %w", err)
	}

	localPubkey, err := btcec.ParsePubKey(pubkeyBytes)
	if err != nil {
		return fmt.Errorf("failed to fetch and parse node pubkey %w", err)
	}

	for _, c := range channels {
		remotePubkey := c.IdentityPub
		var featureBuf bytes.Buffer
		if err := lnwire.NewRawFeatureVector().Encode(&featureBuf); err != nil {
			return fmt.Errorf("unable to encode features: %v", err)
		}
		a.log.Infof("PopulateChannelsGraph: populating edge %v", c.ShortChanID().ToUint64())

		edge := &channeldb.ChannelEdgeInfo{
			ChannelID:    c.ShortChanID().ToUint64(),
			ChainHash:    c.ChainHash,
			Features:     featureBuf.Bytes(),
			Capacity:     c.Capacity,
			ChannelPoint: c.FundingOutpoint,
		}

		selfBytes := localPubkey.SerializeCompressed()
		remoteBytes := remotePubkey.SerializeCompressed()
		var chanFlags lnwire.ChanUpdateChanFlags
		if bytes.Compare(selfBytes, remoteBytes) == -1 {
			copy(edge.NodeKey1Bytes[:], localPubkey.SerializeCompressed())
			copy(edge.NodeKey2Bytes[:], remotePubkey.SerializeCompressed())
			copy(edge.BitcoinKey1Bytes[:], c.LocalChanCfg.MultiSigKey.PubKey.SerializeCompressed())
			copy(edge.BitcoinKey2Bytes[:], c.RemoteChanCfg.MultiSigKey.PubKey.SerializeCompressed())

			// If we're the first node then update the chanFlags to
			// indicate the "direction" of the update.
			chanFlags = 0
		} else {
			copy(edge.NodeKey1Bytes[:], remotePubkey.SerializeCompressed())
			copy(edge.NodeKey2Bytes[:], localPubkey.SerializeCompressed())
			copy(edge.BitcoinKey1Bytes[:], c.RemoteChanCfg.MultiSigKey.PubKey.SerializeCompressed())
			copy(edge.BitcoinKey2Bytes[:], c.LocalChanCfg.MultiSigKey.PubKey.SerializeCompressed())

			// If we're the second node then update the chanFlags to
			// indicate the "direction" of the update.
			chanFlags = 1
		}

		policy1 := &channeldb.ChannelEdgePolicy{
			// SigBytes:                  msg.Signature.ToSignatureBytes(),
			ChannelID:                 c.ShortChanID().ToUint64(),
			LastUpdate:                time.Now(),
			MessageFlags:              lnwire.ChanUpdateOptionMaxHtlc,
			ChannelFlags:              chanFlags,
			TimeLockDelta:             uint16(144),
			MinHTLC:                   c.LocalChanCfg.MinHTLC,
			MaxHTLC:                   c.LocalChanCfg.MaxPendingAmount,
			FeeBaseMSat:               lnwire.MilliSatoshi(1000),
			FeeProportionalMillionths: lnwire.MilliSatoshi(1),
		}

		if chanFlags == 1 {
			chanFlags = 0
		} else {
			chanFlags = 1
		}

		policy2 := &channeldb.ChannelEdgePolicy{
			// SigBytes:                  msg.Signature.ToSignatureBytes(),
			ChannelID:                 c.ShortChanID().ToUint64(),
			LastUpdate:                time.Now(),
			MessageFlags:              lnwire.ChanUpdateOptionMaxHtlc,
			ChannelFlags:              chanFlags,
			TimeLockDelta:             uint16(144),
			MinHTLC:                   c.LocalChanCfg.MinHTLC,
			MaxHTLC:                   c.LocalChanCfg.MaxPendingAmount,
			FeeBaseMSat:               lnwire.MilliSatoshi(1000),
			FeeProportionalMillionths: lnwire.MilliSatoshi(1),
		}

		if err := chandb.ChannelGraph().AddChannelEdge(edge); err != nil {
			a.log.Errorf("failed to add channel edge %w", err)
		}
		if err := chandb.ChannelGraph().UpdateEdgePolicy(policy1); err != nil {
			a.log.Errorf("failed to add channel edge policy 1 %v", err)
		}
		if err := chandb.ChannelGraph().UpdateEdgePolicy(policy2); err != nil {
			a.log.Errorf("failed to add channel edge policy 2 %v", err)
		}
	}
	a.log.Infof("PopulateChannelsGraph completed")
	return nil
}
