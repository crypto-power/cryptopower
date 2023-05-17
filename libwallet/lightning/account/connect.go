package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"

	breezservice "github.com/breez/breez/breez"
	"github.com/breez/breez/data"
)

var (
	waitConnectTimeout = time.Second * 30
)

// ConnectChannelsPeers connects to all peers associated with a non active channel.
func (a *Service) ConnectChannelsPeers() error {
	var nodesToConnect []*lnrpc.NodeInfo

	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{
		InactiveOnly: true,
	})
	if err != nil {
		return err
	}
	for _, c := range channels.Channels {
		nodeInfo, err := lnclient.GetNodeInfo(context.Background(), &lnrpc.NodeInfoRequest{PubKey: c.RemotePubkey})
		if err != nil {
			a.log.Infof("ConnectChannelsPeers got error trying to fetch node %v", nodeInfo)
			continue
		}
		nodesToConnect = append(nodesToConnect, nodeInfo)
	}

	pendingChannels, err := lnclient.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return err
	}
	for _, c := range pendingChannels.PendingOpenChannels {
		nodeInfo, err := lnclient.GetNodeInfo(context.Background(), &lnrpc.NodeInfoRequest{PubKey: c.Channel.RemoteNodePub})
		if err != nil {
			a.log.Infof("ConnectChannelsPeers got error trying to fetch node %v", nodeInfo)
			continue
		}
		nodesToConnect = append(nodesToConnect, nodeInfo)
	}

	for _, n := range nodesToConnect {
		if len(n.GetNode().Addresses) > 0 {
			address := n.GetNode().Addresses[0]
			a.log.Infof("Connecting to peer %v with address= %v", n.GetNode().PubKey, address.Addr)
			lnclient.ConnectPeer(context.Background(), &lnrpc.ConnectPeerRequest{
				Addr: &lnrpc.LightningAddress{
					Pubkey: n.GetNode().PubKey,
					Host:   address.Addr,
				},
				Perm: true,
			})
		}
	}

	return nil
}

type regularLSP struct {
	lspID string
}

func NewRegularLSP(lspID string) *regularLSP {
	return &regularLSP{lspID: lspID}
}

func (lsp *regularLSP) Connect(a *Service) error {
	c, ctx, cancel := a.breezAPI.NewChannelOpenerClient()
	defer cancel()
	r, err := c.LSPList(ctx, &breezservice.LSPListRequest{Pubkey: a.daemonAPI.NodePubkey()})
	if err != nil {
		a.log.Infof("LSPList returned an error: %v", err)
		return err
	}
	l, ok := r.Lsps[lsp.lspID]
	if !ok {
		a.log.Infof("The LSP ID is not in the LSPList: %v", lsp.lspID)
		return fmt.Errorf("The LSP ID is not in the LSPList: %v", lsp.lspID)
	}

	return a.ConnectPeer(l.Pubkey, l.Host)
}

func (lsp *regularLSP) OpenChannel(a *Service, pubkey string) error {
	c, ctx, cancel := a.breezAPI.NewChannelOpenerClient()
	defer cancel()
	_, err := c.OpenLSPChannel(ctx, &breezservice.OpenLSPChannelRequest{LspId: lsp.lspID, Pubkey: pubkey})
	if err != nil {
		return err
	}
	return nil
}

type lnurlLSP struct {
	URI      string `json:"uri"`
	CallBack string `json:"callback"`
	K1       string `json:"k1"`

	//Error fields
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func NewLnurlLSP(lnurl string) (*lnurlLSP, error) {
	hrp, data, err := decode(lnurl)
	if err != nil {
		return nil, err
	}
	if hrp != "lnurl" {
		return nil, fmt.Errorf("invalid lnurl string")
	}
	url, err := convertBits(data, 5, 8, false)
	if err != nil {
		return nil, err
	}
	res, err := http.Get(string(url))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var lnurlLSP lnurlLSP
	err = decoder.Decode(&lnurlLSP)
	if err != nil {
		return nil, err
	}
	if lnurlLSP.Status == "ERROR" {
		return nil, fmt.Errorf("Error: %v", lnurlLSP.Reason)
	}
	return &lnurlLSP, nil
}

func (lsp *lnurlLSP) Connect(a *Service) error {
	s := strings.Split(lsp.URI, "@")
	if len(s) != 2 {
		return fmt.Errorf("Malformed URI: %v", lsp.URI)
	}
	return a.ConnectPeer(s[0], s[1])
}

func (lsp *lnurlLSP) OpenChannel(a *Service, pubkey string) error {
	//<callback>?k1=<k1>&remoteid=<Local LN node ID>&private=<1/0>
	u, err := url.Parse(lsp.CallBack)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("k1", lsp.K1)
	q.Set("remoteid", pubkey)
	q.Set("private", "1")
	u.RawQuery = q.Encode()
	res, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var r struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	err = decoder.Decode(&r)
	if err != nil {
		return err
	}
	if r.Status == "ERROR" {
		return fmt.Errorf("Error: %v", r.Reason)
	}
	return nil
}

type lsp interface {
	Connect(a *Service) error
	OpenChannel(a *Service, pubkey string) error
}

func (a *Service) waitReadyForPayment() error {
	return a.daemonAPI.WaitReadyForPayment(waitConnectTimeout)
}

/*
OpenLSPChannel is responsible for creating a new channel with the LSP
*/
func (a *Service) OpenLSPChannel(lspID string) error {
	l := NewRegularLSP(lspID)
	lsp := lsp(l)
	return a.openChannel(lsp, false)
}

// ConnectLSPPeer connects to the LSP peer.
func (a *Service) ConnectLSPPeer(lspID string) error {
	return NewRegularLSP(lspID).Connect(a)
}

/*
OpenLnurlChannel is responsible for creating a new channel using a lnURL
*/
func (a *Service) OpenLnurlChannel(lnurl string) error {
	l, err := NewLnurlLSP(lnurl)
	if err != nil {
		return err
	}
	lsp := lsp(l)
	return a.openChannel(lsp, true)
}

/*
OpenDirectLnurlChannel is responsible for creating a new channel using lnURL
 data
*/
func (a *Service) OpenDirectLnurlChannel(k1, callback, URI string) error {
	l := &lnurlLSP{
		K1:       k1,
		CallBack: callback,
		URI:      URI,
	}
	lsp := lsp(l)
	return a.openChannel(lsp, true)
}

func (a *Service) openChannel(l lsp, force bool) error {
	a.log.Info("openChannel started...")
	_, err, _ := createChannelGroup.Do("createChannel", func() (interface{}, error) {
		err := a.connectAndOpenChannel(l, force)
		a.log.Info("openChannel finished, error = %v", err)
		return nil, err
	})
	return err
}

func (a *Service) connectAndOpenChannel(l lsp, force bool) error {
	err := l.Connect(a)
	if err != nil {
		return err
	}
	var dontOpen bool
	if !force {
		dontOpen, err = a.hasChannel()
		if err != nil {
			return err
		}
	}
	// open channel if no channel exists.
	if !dontOpen {
		lnclient := a.daemonAPI.APIClient()
		lnInfo, err := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
		if err != nil {
			return err
		}
		err = l.OpenChannel(a, lnInfo.IdentityPubkey)
		if err != nil {
			return err
		}
		pendingChannels, err := a.getPendingChannelPoint()
		if err != nil && len(pendingChannels) > 0 {
			a.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_LSP_CHANNEL_OPENED})
		}
		a.onRoutingNodePendingChannel()
	}
	return nil
}

func (a *Service) ConnectPeer(pubkey, host string) error {
	if a.isConnected(pubkey) {
		return nil
	}
	lnclient := a.daemonAPI.APIClient()
	a.log.Infof("Connecting to routing node host: %v, pubKey: %v", host, pubkey)
	_, err := lnclient.ConnectPeer(context.Background(), &lnrpc.ConnectPeerRequest{
		Addr: &lnrpc.LightningAddress{
			Pubkey: pubkey,
			Host:   host,
		},
		Perm: true,
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Service) hasChannel() (bool, error) {
	channelPoints, _, err := a.getOpenChannels()
	if err != nil {
		return false, fmt.Errorf("openLSPChannel got error in getBreezOpenChannels %v", err)
	}

	if len(channelPoints) > 0 {
		a.log.Infof("openLSPChannel already has a channel with breez, doing nothing")
		return true, nil
	}
	pendingChannels, err := a.getPendingChannelPoint()
	if err != nil {
		return false, fmt.Errorf("openLSPChannel got error in getPendingBreezChannelPoint %v", err)
	}

	if len(pendingChannels) > 0 {
		a.onRoutingNodePendingChannel()
		a.log.Infof("openLSPChannel already has a pending channel with breez, doing nothing")
		return true, nil
	}

	return false, nil
}

func (a *Service) isConnected(pubkey string) bool {
	lnclient := a.daemonAPI.APIClient()
	peers, err := lnclient.ListPeers(context.Background(), &lnrpc.ListPeersRequest{})
	if err != nil {
		a.log.Errorf("isConnected got error in ListPeers: %v", err)
		return false
	}
	for _, p := range peers.Peers {
		if p.PubKey == pubkey {
			return true
		}
	}
	return false
}
