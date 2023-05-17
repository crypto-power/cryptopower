package lightning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/bootstrap"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/chainservice"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/channeldbservice"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/data"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/db"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/doubleratchet"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/lnnode"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightninglabs/neutrino/filterdb"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/breezbackuprpc"
)

// Service is the interface to be implemeted by all associated services
type Service interface {
	Start() error
	Stop() error
}

/*
Start is responsible for starting the lightning client and some go routines to track and notify for account changes
*/
func (client *Client) Start() error {
	if atomic.SwapInt32(&client.started, 1) == 1 {
		return errors.New("Lightning client already started")
	}

	client.log.Info("client.start before bootstrap")
	if err := chainservice.Bootstrap(client.cfg.WorkingDir); err != nil {
		client.log.Infof("app.start bootstrap error %v", err)
		return err
	}

	client.log.Info("app.start: starting services.")
	services := []Service{
		client.lnDaemon,
		client.ServicesClient,
		client.SwapService,
		client.AccountService,
		client.BackupManager,
	}

	if err := client.lspChanStateSyncer.recordChannelsStatus(); err != nil {
		client.log.Errorf("failed to collect channels state %v", err)
	}

	for _, s := range services {
		if err := s.Start(); err != nil {
			return err
		}
	}

	client.wg.Add(1)
	go client.watchDaemonEvents()

	return nil
}

/*
Stop is responsible for stopping the ligtning daemon.
*/
func (client *Client) Stop() error {
	if atomic.SwapInt32(&client.stopped, 1) == 1 {
		return nil
	}

	close(client.quitChan)
	client.BackupManager.Stop()
	client.SwapService.Stop()
	client.AccountService.Stop()
	client.ServicesClient.Stop()
	client.lnDaemon.Stop()
	doubleratchet.Stop()
	client.releaseBreezDB()

	client.wg.Wait()
	client.log.Infof("BreezApp shutdown successfully")
	return nil
}

/*
DaemonReady return the status of the lightningLib daemon
*/
func (client *Client) DaemonReady() bool {
	return atomic.LoadInt32(&client.isReady) == 1
}

// NotificationChan returns client channel that receives notification events
func (client *Client) NotificationChan() chan data.NotificationEvent {
	return client.notificationsChan
}

/*
OnResume recalculate things we might missed when we were idle.
*/
func (client *Client) OnResume() {
	if atomic.LoadInt32(&client.isReady) == 1 {
		client.AccountService.OnResume()
		client.SwapService.SettlePendingTransfers()
	}
}

func (client *Client) RestartDaemon() error {
	return client.lnDaemon.RestartDaemon()
}

// Restore is the breez API for restoring client specific nodeID using the configured
// backup backend provider.
func (client *Client) Restore(nodeID string, key []byte) error {
	client.log.Infof("Restore nodeID = %v", nodeID)
	if err := client.releaseBreezDB(); err != nil {
		return err
	}
	defer func() {
		client.breezDB, client.releaseBreezDB, _ = db.Get(client.cfg.WorkingDir)
	}()
	_, err := client.BackupManager.Restore(nodeID, key)
	return err
}

/*
GetLogPath returns the log file path.
*/
func (client *Client) GetLogPath() string {
	return client.cfg.WorkingDir + "/logs/bitcoin/" + client.cfg.Network + "/lnd.log"
}

// GetWorkingDir returns the working dir.
func (client *Client) GetWorkingDir() string {
	return client.cfg.WorkingDir
}

func (client *Client) startAppServices() error {
	if err := client.AccountService.Start(); err != nil {
		return err
	}
	return nil
}

func (client *Client) watchDaemonEvents() error {
	defer client.wg.Done()

	evt, err := client.lnDaemon.SubscribeEvents()
	defer evt.Cancel()

	if err != nil {
		return err
	}
	for {
		select {
		case u := <-evt.Updates():
			switch u.(type) {
			case lnnode.DaemonReadyEvent:
				atomic.StoreInt32(&client.isReady, 1)
				go client.ensureSafeToRunNode()
				go client.notify(data.NotificationEvent{Type: data.NotificationEvent_READY})
			case lnnode.DaemonDownEvent:
				atomic.StoreInt32(&client.isReady, 0)
				go client.notify(data.NotificationEvent{Type: data.NotificationEvent_LIGHTNING_SERVICE_DOWN})
			case lnnode.BackupNeededEvent:
				client.BackupManager.RequestCommitmentChangedBackup()
			case lnnode.ChannelEvent:
				if client.lnDaemon.HasActiveChannel() {
					go client.ensureSafeToRunNode()
				}
			case lnnode.ChainSyncedEvent:
				chainService, cleanupFn, err := chainservice.Get(client.cfg.WorkingDir, client.breezDB)
				if err != nil {
					client.log.Errorf("failed to get chain service on sync event")
					break
				}
				err = chainService.FilterDB.PurgeFilters(filterdb.RegularFilter)
				if err != nil {
					client.log.Errorf("purge compact filters finished error = %v", err)
				}
				cleanupFn()
			}
		case <-evt.Quit():
			return nil
		}
	}
}

func (client *Client) ensureSafeToRunNode() bool {
	lnclient := client.lnDaemon.APIClient()
	info, err := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		client.log.Errorf("ensureSafeToRunNode failed, continue anyway %v", err)
		return true
	}
	safe, err := client.BackupManager.IsSafeToRunNode(info.IdentityPubkey)
	if err != nil {
		client.log.Errorf("ensureSafeToRunNode failed, continue anyway %v", err)
		return true
	}
	if !safe {
		client.log.Errorf("ensureSafeToRunNode detected remote restore! stopping breez since it is not safe to run")
		go client.notify(data.NotificationEvent{Type: data.NotificationEvent_BACKUP_NODE_CONFLICT})
		client.lnDaemon.Stop()
		return false
	}
	client.log.Infof("ensureSafeToRunNode succeed, safe to run node: %v", info.IdentityPubkey)
	return true
}

func (client *Client) onServiceEvent(event data.NotificationEvent) {
	client.notify(event)
	if event.Type == data.NotificationEvent_FUND_ADDRESS_CREATED ||
		event.Type == data.NotificationEvent_LSP_CHANNEL_OPENED {
		client.BackupManager.RequestNodeBackup()
	}
}

func (client *Client) RequestBackup() {
	client.BackupManager.RequestFullBackup()
}

func (client *Client) notify(event data.NotificationEvent) {
	client.notificationsChan <- event
}

func (client *Client) SetPeers(peers []string) error {
	return client.breezDB.SetPeers(peers)
}

func (client *Client) TestPeer(peer string) error {
	if peer == "" {
		if len(client.cfg.JobCfg.ConnectedPeers) == 0 {
			return errors.New("no default peer")
		}
		peer = client.cfg.JobCfg.ConnectedPeers[0]
	}
	return chainservice.TestPeer(peer)
}

func (client *Client) GetPeers() (peers []string, isDefault bool, err error) {
	return client.breezDB.GetPeers(client.cfg.JobCfg.ConnectedPeers)
}

func (client *Client) SetTxSpentURL(txSpentURL string) error {
	return client.breezDB.SetTxSpentURL(txSpentURL)
}

func (client *Client) GetTxSpentURL() (txSpentURL string, isDefault bool, err error) {
	return client.breezDB.GetTxSpentURL(client.cfg.TxSpentURL)
}

func (client *Client) ClosedChannels() (int, error) {
	return client.lnDaemon.ClosedChannels()
}

func (client *Client) LastSyncedHeaderTimestamp() (int64, error) {
	return client.breezDB.FetchLastSyncedHeaderTimestamp()
}

func (client *Client) DeleteGraph() error {
	chanDB, chanDBCleanUp, err := channeldbservice.Get(client.cfg.WorkingDir)
	if err != nil {
		client.log.Errorf("channeldbservice.Get(%v): %v", client.cfg.WorkingDir, err)
		return fmt.Errorf("channeldbservice.Get(%v): %w", client.cfg.WorkingDir, err)
	}
	defer chanDBCleanUp()
	graph := chanDB.ChannelGraph()

	cids := make(map[uint64]struct{})
	nodes := 0
	ourNode, err := graph.SourceNode()
	if err != nil {
		client.log.Errorf("graph.SourceNode() error = %v", err)
		return fmt.Errorf("graph.SourceNode(): %w", err)
	}
	ourCids := make(map[uint64]struct{})
	ourNodeKeyBytes := ourNode.PubKeyBytes
	err = chanDB.View(func(tx walletdb.ReadTx) error {
		return ourNode.ForEachChannel(tx, func(tx walletdb.ReadTx,
			channelEdgeInfo *channeldb.ChannelEdgeInfo,
			_ *channeldb.ChannelEdgePolicy,
			_ *channeldb.ChannelEdgePolicy) error {
			ourCids[channelEdgeInfo.ChannelID] = struct{}{}
			return nil
		})
	}, func() {})
	if err != nil {
		client.log.Errorf("ourNode.ForEachChannel error = %v", err)
		return fmt.Errorf("ourNode.ForEachChannel: %w", err)
	}
	err = chanDB.View(func(tx walletdb.ReadTx) error {
		return graph.ForEachNode(func(tx walletdb.ReadTx, lightningNode *channeldb.LightningNode) error {
			if bytes.Equal(lightningNode.PubKeyBytes[:], ourNodeKeyBytes[:]) {
				return nil
			}
			nodes++
			return lightningNode.ForEachChannel(tx, func(tx walletdb.ReadTx,
				channelEdgeInfo *channeldb.ChannelEdgeInfo,
				_ *channeldb.ChannelEdgePolicy,
				_ *channeldb.ChannelEdgePolicy) error {
				// Add the channel only if it's not connected to our node
				if _, ok := ourCids[channelEdgeInfo.ChannelID]; !ok {
					cids[channelEdgeInfo.ChannelID] = struct{}{}
				}
				return nil
			})
		})
	}, func() {})
	if err != nil {
		client.log.Errorf("DeleteNodeFromGraph->ForEachNodeChannel error = %v", err)
		return fmt.Errorf("ForEachNodeChannel: %w", err)
	}
	client.log.Infof("About to delete %v channels from %v nodes.", len(cids), nodes)
	var chanIDs []uint64
	for cid := range cids {
		chanIDs = append(chanIDs, cid)
	}
	err = graph.DeleteChannelEdges(true, true, chanIDs...)
	if err != nil {
		client.log.Errorf("DeleteNodeFromGraph->DeleteChannelEdges error = %v", err)
		return fmt.Errorf("DeleteChannelEdges: %w", err)
	}

	err = graph.PruneGraphNodes()
	if err != nil {
		client.log.Errorf("DeleteNodeFromGraph->PruneGraphNodes error = %v", err)
		return fmt.Errorf("PruneGraphNodes(): %w", err)
	}
	client.log.Infof("Deleted %v channels from %v nodes.", len(chanIDs), nodes)
	return nil
}

func (client *Client) GraphUrl() (string, error) {
	if client.breezDB == nil {
		return "", fmt.Errorf("breezDB still not initialized")
	}
	return bootstrap.GraphURL(client.GetWorkingDir(), client.breezDB)
}

func (client *Client) BackupFiles() (string, error) {
	res, err := client.lnDaemon.BreezBackupClient().GetBackup(context.Background(), &breezbackuprpc.GetBackupRequest{})
	if err != nil {
		return "", err
	}

	jsonRes, err := json.Marshal(res.Files)
	return string(jsonRes), err
}

func (client *Client) PopulateChannelPolicy() {
	if err := client.lnDaemon.PopulateChannelsGraph(); err != nil {
		client.log.Errorf("failed to populate graph %v", err)
	}
}

func (client *Client) ResetClosedChannelChainInfo(r *data.ResetClosedChannelChainInfoRequest) (
	*data.ResetClosedChannelChainInfoReply, error) {

	err := client.lspChanStateSyncer.resetClosedChannelChainInfo(r.ChanPoint, r.BlockHeight)
	if err != nil {
		return nil, err
	}
	return &data.ResetClosedChannelChainInfoReply{}, nil
}

func (client *Client) CheckLSPClosedChannelMismatch(
	r *data.CheckLSPClosedChannelMismatchRequest) (
	*data.CheckLSPClosedChannelMismatchResponse, error) {

	mismatch, err := client.lspChanStateSyncer.checkLSPClosedChannelMismatch(r.LspInfo.Pubkey,
		r.LspInfo.LspPubkey, r.LspInfo.Id, r.ChanPoint)
	if err != nil {
		return nil, err
	}
	return &data.CheckLSPClosedChannelMismatchResponse{Mismatch: mismatch}, nil
}

func (client *Client) SetTorActive(enable bool) error {
	client.log.Infof("setTorActive: setting enabled = %v", enable)
	return client.breezDB.SetTorActive(enable)
}

func (client *Client) GetTorActive() bool {
	client.log.Info("getTorActive")

	b, err := client.breezDB.GetTorActive()
	if err != nil {
		client.log.Infof("getTorActive: %v", err)
	}
	return b
}
