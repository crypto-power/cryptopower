package account

import (
	"context"
	"math"
	"time"

	"github.com/breez/breez/db"

	"github.com/breez/breez/data"
	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/lnrpc"
	"golang.org/x/sync/singleflight"
)

const (
	maxPaymentAllowedSat  = math.MaxUint32 / 1000
	endpointTimeout       = 5
	maxGlobalReceiveLimit = 4_000_000
)

var (
	createChannelGroup singleflight.Group
)

/*
GetAccountInfo is responsible for retrieving some general account details such as balance, status, etc...
*/
func (a *Service) GetAccountInfo() (*data.Account, error) {
	accBuf, err := a.breezDB.FetchAccount()
	if err != nil {
		return nil, err
	}
	account := &data.Account{}
	if accBuf != nil {
		err = proto.Unmarshal(accBuf, account)
	}
	account.ReadyForPayments = a.daemonAPI.HasActiveChannel()
	return account, err
}

/*
GetAccountLimits returns the account limits.
*/
func (a *Service) GetAccountLimits() (maxReceive, maxPay, maxReserve int64, err error) {
	return a.getReceivePayLimit()
}

/*
GetGlobalMaxReceiveLimit returns the account global max receive limit.
*/
func (a *Service) GetGlobalMaxReceiveLimit() (maxReceive int64, err error) {
	receive, _, _, err := a.getReceivePayLimit()
	return receive, err
}

// EnableAccount controls whether the account will be enabled or disabled.
// When disbled, no attempt will be made to open a channel with breez node.
func (a *Service) EnableAccount(enabled bool) error {
	if err := a.breezDB.EnableAccount(enabled); err != nil {
		a.log.Infof("Error in enabling account (enabled = %v) %v", enabled, err)
		return err
	}
	a.onAccountChanged()
	return nil
}

func (a *Service) getAccountStatus(walletBalance *lnrpc.WalletBalanceResponse) (data.Account_AccountStatus, string, error) {
	_, channelPoints, err := a.getOpenChannels()
	if err != nil {
		return -1, "", err
	}
	if len(channelPoints) > 0 {
		return data.Account_CONNECTED, channelPoints[0], nil
	}

	lnclient := a.daemonAPI.APIClient()
	pendingChannels, err := lnclient.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return -1, "", err
	}
	if len(pendingChannels.PendingOpenChannels) > 0 {
		chanPoint := pendingChannels.PendingOpenChannels[0].Channel.ChannelPoint
		return data.Account_PROCESSING_CONNECTION, chanPoint, nil
	}
	if len(pendingChannels.PendingClosingChannels) > 0 || len(pendingChannels.PendingForceClosingChannels) > 0 {
		return data.Account_CLOSING_CONNECTION, "", nil
	}

	return data.Account_DISCONNECTED, "", nil
}

func (a *Service) getConnectedPeers() (peers []string, err error) {
	lnclient := a.daemonAPI.APIClient()
	response, err := lnclient.ListPeers(context.Background(), &lnrpc.ListPeersRequest{})
	if err != nil {
		return nil, err
	}
	connectedPeers := make([]string, len(response.Peers))
	for _, p := range response.Peers {
		connectedPeers = append(connectedPeers, p.PubKey)
	}
	return connectedPeers, nil
}

func (a *Service) unconfirmedChannels() ([]string, error) {
	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		return nil, err
	}
	var unconfirmedChannels []string
	for _, c := range channels.Channels {
		if c.ZeroConf && c.ZeroConfConfirmedScid == 0 {
			unconfirmedChannels = append(unconfirmedChannels, c.ChannelPoint)
		}
	}
	return unconfirmedChannels, nil
}

func (a *Service) getMaxReceiveSingleChannel() (maxPay int64, err error) {
	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		return 0, err
	}

	var maxAllowedReceive int64
	for _, b := range channels.Channels {
		thisChannelCanReceive := b.RemoteBalance - b.RemoteChanReserveSat
		if !b.Initiator {
			// In case this is a remote initated channel we will take a buffer of half commit fee size
			// to ensure the remote balance won't get too close to the channel reserve.
			thisChannelCanReceive -= b.CommitFee / 2
		}
		if maxAllowedReceive < thisChannelCanReceive {
			maxAllowedReceive = thisChannelCanReceive
		}
	}
	return maxAllowedReceive, nil
}

func (a *Service) getOutgoingPendingAmount() (amt int64, err error) {
	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		return 0, err
	}
	amt = 0
	for _, c := range channels.Channels {
		for _, h := range c.PendingHtlcs {
			if !h.Incoming {
				amt += h.Amount
			}
		}
	}
	return amt, nil
}

func (a *Service) getReceivePayLimit() (maxReceive, maxPay, maxReserve int64, err error) {
	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		return 0, 0, 0, err
	}

	pendingChannels, err := lnclient.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return 0, 0, 0, err
	}

	channelBalance, err := lnclient.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
	if err != nil {
		return 0, 0, 0, err
	}

	var maxAllowedToReceive int64
	var maxAllowedToPay int64
	var maxChanReserve int64

	processChannel := func(canPay, chanReserve int64) {
		if canPay < 0 {
			canPay = 0
		}
		maxAllowedToPay += canPay
		maxChanReserve += chanReserve
	}

	for _, b := range channels.Channels {
		thisChannelCanReceive := b.RemoteBalance - b.RemoteChanReserveSat
		thisChannelCanPay := b.LocalBalance - b.LocalChanReserveSat
		if !b.Initiator {
			// In case this is a remote initated channel we will take a buffer of half commit fee size
			// to ensure the remote balance won't get too close to the channel reserve.
			thisChannelCanReceive -= b.CommitFee / 2
		} else {
			// Otherwise we need to restrict how much we can pay at the same manner
			thisChannelCanPay -= b.CommitFee / 2
		}
		processChannel(thisChannelCanPay, b.LocalChanReserveSat)
	}

	for _, b := range pendingChannels.PendingOpenChannels {
		processChannel(0, b.Channel.LocalChanReserveSat)
	}
	maxAllowedToReceive = maxGlobalReceiveLimit - channelBalance.Balance
	if maxAllowedToReceive < 0 {
		maxAllowedToReceive = 0
	}

	return maxAllowedToReceive, maxAllowedToPay, maxChanReserve, nil
}

func (a *Service) getRoutingNodeFeeRate(ourKey string) (int64, error) {
	chanIDs, _, err := a.getOpenChannels()
	if err != nil {
		a.log.Errorf("Failed to get breez channels %v", err)
		return 0, err
	}

	if len(chanIDs) == 0 {
		return 0, nil
	}

	lnclient := a.daemonAPI.APIClient()
	edge, err := lnclient.GetChanInfo(context.Background(), &lnrpc.ChanInfoRequest{ChanId: chanIDs[0]})
	if err != nil {
		a.log.Errorf("Failed to get breez channel info %v", err)
		return 0, err
	}

	if ourKey == edge.Node1Pub && edge.Node2Policy != nil {
		return edge.Node2Policy.FeeBaseMsat / 1000, nil
	} else if edge.Node1Policy != nil {
		return edge.Node1Policy.FeeBaseMsat / 1000, nil
	}
	return 0, nil
}

func (a *Service) getOpenChannels() ([]uint64, []string, error) {
	var channelPoints []string
	var channelIds []uint64
	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{
		PrivateOnly: true,
	})
	if err != nil {
		return nil, nil, err
	}

	for _, c := range channels.Channels {
		channelPoints = append(channelPoints, c.ChannelPoint)
		channelIds = append(channelIds, c.ChanId)
		a.log.Infof("Channel Point with node = %v", c.ChannelPoint)
	}
	return channelIds, channelPoints, nil
}

func (a *Service) getPendingChannelPoint() (string, error) {
	lnclient := a.daemonAPI.APIClient()
	pendingChannels, err := lnclient.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return "", err
	}

	if len(pendingChannels.PendingOpenChannels) == 0 {
		return "", nil
	}

	for _, c := range pendingChannels.PendingOpenChannels {
		return c.Channel.ChannelPoint, nil
	}

	return "", nil
}

func (a *Service) calculateAccount() (*data.Account, error) {
	lnclient := a.daemonAPI.APIClient()
	lnInfo, err := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return nil, err
	}

	channelBalance, err := lnclient.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
	if err != nil {
		return nil, err
	}
	normalizedBalance := channelBalance.Balance
	if normalizedBalance > 0 {
		outgoingPending, err := a.getOutgoingPendingAmount()
		if err != nil {
			return nil, err
		}

		// add all pending htlcs to the balance.
		normalizedBalance += outgoingPending
		pendingPayments, err := a.getPendingPayments(false)
		if err != nil {
			return nil, err
		}
		for _, pending := range pendingPayments {
			if pending.Type == db.SentPayment {
				a.log.Infof("removing pending amount %v", (pending.Amount + pending.Fee))
				normalizedBalance -= pending.Amount
			}
		}
	}

	// in case we have MPP in flight, this normalization may end up with negative
	// value (the payment will fail eventually), here zero reflects better the state.
	if normalizedBalance < 0 {
		normalizedBalance = 0
	}

	walletBalance, err := lnclient.WalletBalance(context.Background(), &lnrpc.WalletBalanceRequest{})
	if err != nil {
		return nil, err
	}

	accStatus, chanPoint, err := a.getAccountStatus(walletBalance)
	if err != nil {
		return nil, err
	}

	connectedPeers, err := a.getConnectedPeers()
	if err != nil {
		return nil, err
	}

	maxAllowedToReceive, maxAllowedToPay, maxChanReserve, err := a.getReceivePayLimit()
	if err != nil {
		return nil, err
	}

	maxInboundLiquidity, err := a.getMaxReceiveSingleChannel()
	if err != nil {
		return nil, err
	}

	routingNodeFeeRate, err := a.getRoutingNodeFeeRate(lnInfo.IdentityPubkey)
	if err != nil {
		a.log.Infof("Failed to get routing node fee %v", err)
	}
	a.log.Infof("Routing node fee rate = %v", routingNodeFeeRate)

	enabled, err := a.breezDB.AccountEnabled()
	if err != nil {
		return nil, err
	}
	unconfirmedChannels, err := a.unconfirmedChannels()
	if err != nil {
		return nil, err
	}
	onChainBalance := walletBalance.ConfirmedBalance
	return &data.Account{
		Id:                  lnInfo.IdentityPubkey,
		Balance:             normalizedBalance,
		MaxAllowedToReceive: maxAllowedToReceive,
		MaxAllowedToPay:     maxAllowedToPay,
		MaxPaymentAmount:    maxPaymentAllowedSat,
		MaxChanReserve:      maxChanReserve,
		Status:              accStatus,
		WalletBalance:       onChainBalance,
		RoutingNodeFee:      routingNodeFeeRate,
		ReadyForPayments:    a.daemonAPI.HasActiveChannel(),
		Enabled:             enabled,
		ChannelPoint:        chanPoint,
		TipHeight:           int64(lnInfo.BlockHeight),
		ConnectedPeers:      connectedPeers,
		MaxInboundLiquidity: maxInboundLiquidity,
		UnconfirmedChannels: unconfirmedChannels,
	}, nil
}

// We need to put some dealy on this bacause there is a gap between transaction hit LND and the other side effects that happen
// like channel updates, balance updates etc...
func (a *Service) onAccountChanged() {
	time.Sleep(2 * time.Second)
	a.calculateAccountAndNotify()
}

func (a *Service) calculateAccountAndNotify() (*data.Account, error) {
	acc, err := a.calculateAccount()
	if err != nil {
		a.log.Errorf("Failed to calculate account %v", err)
	}
	accBuf, err := proto.Marshal(acc)
	if err != nil {
		a.log.Errorf("failed to marshal account, change event wasn't propagated")
		return nil, err
	}
	a.breezDB.SaveAccount(accBuf)
	a.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_ACCOUNT_CHANGED})
	return acc, nil
}
