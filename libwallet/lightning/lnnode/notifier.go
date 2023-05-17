package lnnode

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/backuprpc"
	"github.com/lightningnetwork/lnd/lnrpc/breezbackuprpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnrpc/signrpc"
	"github.com/lightningnetwork/lnd/lnrpc/submarineswaprpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/subscribe"
)

// DaemonReadyEvent is sent when the daemon is ready for RPC requests
type DaemonReadyEvent struct {
	IdentityPubkey string
}

// DaemonDownEvent is sent when the daemon stops
type DaemonDownEvent struct{}

// ChannelEvent is sent whenever a channel is created/closed or active/inactive.
type ChannelEvent struct {
	*lnrpc.ChannelEventUpdate
}

// PeerEvent is sent whenever a peer is connected/disconnected.
type PeerEvent struct {
	*lnrpc.PeerEvent
}

// TransactionEvent is sent when a new transaction is received.
type TransactionEvent struct {
	*lnrpc.Transaction
}

// InvoiceEvent is sent when a new invoice is created/settled.
type InvoiceEvent struct {
	*lnrpc.Invoice
}

// ChainSyncedEvent is sent when the chain gets into synced state.
type ChainSyncedEvent struct{}

// ResumeEvent is sent when the app resumes.
type ResumeEvent struct{}

// BackupNeededEvent is sent when the node signals backup is needed.
type BackupNeededEvent struct{}

// RoutingNodeChannelOpened is sent when a channel with the routing
// node is opened.
type RoutingNodeChannelOpened struct{}

// SubscribeEvents subscribe to various application events
func (d *Daemon) SubscribeEvents() (*subscribe.Client, error) {
	return d.ntfnServer.Subscribe()
}

func (d *Daemon) startSubscriptions() error {
	var err error
	grpcCon, err := newLightningClient(d.cfg)
	if err != nil {
		return err
	}

	d.Lock()
	d.lightningClient = lnrpc.NewLightningClient(grpcCon)
	d.subswapClient = submarineswaprpc.NewSubmarineSwapperClient(grpcCon)
	d.breezBackupClient = breezbackuprpc.NewBreezBackuperClient(grpcCon)
	d.routerClient = routerrpc.NewRouterClient(grpcCon)
	d.walletKitClient = walletrpc.NewWalletKitClient(grpcCon)
	d.chainNotifierClient = chainrpc.NewChainNotifierClient(grpcCon)
	d.signerClient = signrpc.NewSignerClient(grpcCon)
	d.invoicesClient = invoicesrpc.NewInvoicesClient(grpcCon)
	d.Unlock()

	info, chainErr := d.lightningClient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if chainErr != nil {
		d.log.Warnf("Failed get chain info", chainErr)
		return chainErr
	}

	d.Lock()
	d.nodePubkey = info.IdentityPubkey
	d.Unlock()

	backupEventClient := backuprpc.NewBackupClient(grpcCon)
	ctx, cancel := context.WithCancel(context.Background())

	d.wg.Add(7)
	go d.subscribeChannels(d.lightningClient, ctx)
	go d.subscribePeers(d.lightningClient, ctx)
	go d.subscribeTransactions(ctx)
	go d.subscribeInvoices(ctx)
	go d.subscribeChannelAcceptor(ctx, d.lightningClient)
	go d.watchBackupEvents(backupEventClient, ctx)
	go d.syncToChain(ctx)

	// cancel subscriptions on quit
	go func() {
		<-d.quitChan
		cancel()
	}()

	if err := d.ntfnServer.SendUpdate(DaemonReadyEvent{IdentityPubkey: info.IdentityPubkey}); err != nil {
		return err
	}
	d.log.Infof("Daemon ready! subscriptions started")
	return nil
}

func (d *Daemon) subscribeChannelAcceptor(ctx context.Context, client lnrpc.LightningClient) error {
	defer d.wg.Done()

	channelAcceptorClient, err := client.ChannelAcceptor(ctx)
	if err != nil {
		d.log.Errorf("Failed to get a channel acceptor %v", err)
		return err
	}
	for {
		request, err := channelAcceptorClient.Recv()
		if err == io.EOF || ctx.Err() == context.Canceled {
			d.log.Errorf("channelAcceptorClient cancelled, shutting down")
			return err
		}

		if err != nil {
			d.log.Errorf("channelAcceptorClient failed to get notification %v", err)
			// in case of unexpected error, we will wait a bit so we won't get
			// into infinite loop.
			time.Sleep(2 * time.Second)
			continue
		}

		private := request.ChannelFlags&uint32(lnwire.FFAnnounceChannel) == 0
		d.log.Infof("channel creation requested from node: %v private: %v", request.NodePubkey, private)
		resp := &lnrpc.ChannelAcceptResponse{
			PendingChanId: request.PendingChanId,
			Accept:        private,
		}
		if request.WantsZeroConf {
			d.log.Infof("channel requested wants zero conf")
			resp.MinAcceptDepth = 0
			resp.ZeroConf = true
		}

		serialized, err := json.Marshal(resp)
		if err == nil {
			d.log.Infof("sending response to channel acceptor: %v", serialized)
		}

		err = channelAcceptorClient.Send(resp)

		if err != nil {
			d.log.Errorf("Error in channelAcceptorClient.Send(%v, %v): %v", request.PendingChanId, private, err)
			return err
		}
	}
}

func (d *Daemon) subscribePeers(client lnrpc.LightningClient, ctx context.Context) error {
	defer d.wg.Done()

	subscription, err := client.SubscribePeerEvents(ctx, &lnrpc.PeerEventSubscription{})
	if err != nil {
		d.log.Errorf("Failed to subscribe peers %v", err)
		return err
	}

	d.log.Infof("Peers subscription created")
	for {
		notification, err := subscription.Recv()
		if err == io.EOF || ctx.Err() == context.Canceled {
			d.log.Errorf("subscribePeers cancelled, shutting down")
			return err
		}

		d.log.Infof("peer event type %v received for peer = %v", notification.Type, notification.PubKey)
		if err != nil {
			d.log.Errorf("subscribe peers Failed to get notification %v", err)
			// in case of unexpected error, we will wait a bit so we won't get
			// into infinite loop.
			time.Sleep(2 * time.Second)
			continue
		}

		d.ntfnServer.SendUpdate(PeerEvent{notification})
	}
}

func (d *Daemon) subscribeChannels(client lnrpc.LightningClient, ctx context.Context) error {
	defer d.wg.Done()

	subscription, err := client.SubscribeChannelEvents(ctx, &lnrpc.ChannelEventSubscription{})
	if err != nil {
		d.log.Errorf("Failed to subscribe channels %v", err)
		return err
	}

	d.log.Infof("Channels subscription created")
	for {
		notification, err := subscription.Recv()
		if err == io.EOF || ctx.Err() == context.Canceled {
			d.log.Errorf("subscribeChannels cancelled, shutting down")
			return err
		}

		d.log.Infof("Channel event type %v received for channel = %v", notification.Type, notification.Channel)
		if err != nil {
			d.log.Errorf("subscribe channels Failed to get notification %v", err)
			// in case of unexpected error, we will wait a bit so we won't get
			// into infinite loop.
			time.Sleep(2 * time.Second)
			continue
		}

		d.ntfnServer.SendUpdate(ChannelEvent{notification})
	}
}

func (d *Daemon) subscribeTransactions(ctx context.Context) error {
	defer d.wg.Done()

	stream, err := d.lightningClient.SubscribeTransactions(ctx, &lnrpc.GetTransactionsRequest{})
	if err != nil {
		d.log.Criticalf("Failed to call SubscribeTransactions %v, %v", stream, err)
	}

	d.log.Infof("Wallet transactions subscription created")
	for {
		notification, err := stream.Recv()
		if err == io.EOF || ctx.Err() == context.Canceled {
			d.log.Errorf("subscribeTransactions cancelled, shutting down")
			return err
		}
		d.log.Infof("subscribeTransactions received new transaction")
		if err != nil {
			d.log.Errorf("Failed to receive a transaction : %v", err)
			// in case of unexpected error, we will wait a bit so we won't get
			// into infinite loop.
			time.Sleep(2 * time.Second)
		}
		d.log.Infof("watchOnChainState sending account change notification")
		d.ntfnServer.SendUpdate(TransactionEvent{notification})
	}
}

func (d *Daemon) subscribeInvoices(ctx context.Context) error {
	defer d.wg.Done()

	stream, err := d.lightningClient.SubscribeInvoices(ctx, &lnrpc.InvoiceSubscription{})
	if err != nil {
		d.log.Criticalf("Failed to call SubscribeInvoices %v, %v", stream, err)
		return err
	}

	d.log.Infof("Invoices subscription created")
	for {
		invoice, err := stream.Recv()
		if err == io.EOF || ctx.Err() == context.Canceled {
			d.log.Errorf("subscribeInvoices cancelled, shutting down")
			return err
		}
		if err != nil {
			d.log.Criticalf("Failed to receive an invoice : %v", err)
			return err
		}
		d.log.Infof("watchPayments - Invoice received by subscription")
		d.ntfnServer.SendUpdate(InvoiceEvent{invoice})
	}
}

func (d *Daemon) watchBackupEvents(client backuprpc.BackupClient, ctx context.Context) error {
	defer d.wg.Done()

	stream, err := client.SubscribeBackupEvents(ctx, &backuprpc.BackupEventSubscription{})
	if err != nil {
		d.log.Criticalf("Failed to call SubscribeBackupEvents %v, %v", stream, err)
	}

	d.log.Infof("Backup events subscription created")
	for {
		_, err := stream.Recv()
		if err == io.EOF || ctx.Err() == context.Canceled {
			d.log.Errorf("watchBackupEvents cancelled, shutting down")
			return err
		}
		d.log.Infof("watchBackupEvents received new event")
		if err != nil {
			d.log.Errorf("watchBackupEvents failed to receive a new event: %v, %v", stream, err)
			return err
		}
		d.ntfnServer.SendUpdate(BackupNeededEvent{})
	}
}

func (d *Daemon) syncToChain(ctx context.Context) error {
	defer d.wg.Done()
	for {
		chainInfo, chainErr := d.lightningClient.GetInfo(ctx, &lnrpc.GetInfoRequest{})
		if chainErr != nil {
			d.log.Warnf("Failed get chain info", chainErr)
			return chainErr
		}

		d.log.Infof("Sync to chain interval Synced=%v BlockHeight=%v", chainInfo.SyncedToChain, chainInfo.BlockHeight)

		if err := d.breezDB.SetLastSyncedHeaderTimestamp(chainInfo.BestHeaderTimestamp); err != nil {
			d.log.Errorf("Failed to set last header timestamp")
		}

		if chainInfo.SyncedToChain {
			d.log.Infof("Synchronized to chain finshed BlockHeight=%v", chainInfo.BlockHeight)
			break
		}
		time.Sleep(time.Second * 3)
	}
	d.ntfnServer.SendUpdate(ChainSyncedEvent{})
	return nil
}
