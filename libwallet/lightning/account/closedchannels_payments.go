package account

import (
	"bytes"
	"context"
	"encoding/hex"
	"time"

	"github.com/breez/breez/chainservice"
	"github.com/breez/breez/db"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
)

func (a *Service) syncClosedChannels() error {
	a.log.Infof("syncClosedChannels")
	lnclient := a.daemonAPI.APIClient()
	closedChannels, err := lnclient.ClosedChannels(context.Background(), &lnrpc.ClosedChannelsRequest{})
	if err != nil {
		return err
	}
	walletClient := a.daemonAPI.WalletKitClient()
	sweepsResponse, err := walletClient.ListSweeps(context.Background(),
		&walletrpc.ListSweepsRequest{Verbose: true})
	if err != nil {
		a.log.Errorf("failed to list sweeps %v", err)
	}

	closingToSweeps := make(map[string]string)
	if sweepsResponse != nil {
		txDetails := sweepsResponse.Sweeps.(*walletrpc.ListSweepsResponse_TransactionDetails)
		if txDetails != nil {
			for _, s := range txDetails.TransactionDetails.Transactions {
				var tx wire.MsgTx
				rawTx, _ := hex.DecodeString(s.RawTxHex)
				if err = tx.Deserialize(bytes.NewReader(rawTx)); err != nil {
					return err
				}
				for _, txIn := range tx.TxIn {
					a.log.Infof("syncClosedChannels: got sweep tx: %v previousOutpoint: ",
						tx.TxHash().String(), tx.TxHash().String())
					closingToSweeps[txIn.PreviousOutPoint.Hash.String()] = tx.TxHash().String()
				}
			}
		}
	}

	for _, c := range closedChannels.Channels {
		if err := a.onClosedChannel(c, closingToSweeps[c.ClosingTxHash]); err != nil {
			return err
		}
	}
	pendingChannels, err := lnclient.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return err
	}
	for _, c := range pendingChannels.WaitingCloseChannels {
		if err := a.onWaitingClosedChannel(c); err != nil {
			return err
		}
	}
	for _, c := range pendingChannels.PendingClosingChannels {
		if err := a.onPendingClosedChannel(c, closingToSweeps[c.ClosingTxid]); err != nil {
			return err
		}
	}
	for _, c := range pendingChannels.PendingForceClosingChannels {
		if err := a.onPendingForceClosedChannel(c, closingToSweeps[c.ClosingTxid]); err != nil {
			return err
		}
	}
	return nil
}

func (a *Service) onWaitingClosedChannel(waitingClose *lnrpc.PendingChannelsResponse_WaitingCloseChannel) error {
	a.log.Infof("onWaitingClosedChannel %v", waitingClose.Channel.ChannelPoint)
	if waitingClose.Channel.LocalBalance == 0 {
		a.log.Infof("closed channel skipped due to zero amount")
		return nil
	}
	paymentData := &db.PaymentInfo{
		Type:                    db.ClosedChannelPayment,
		ClosedChannelPoint:      waitingClose.Channel.ChannelPoint,
		ClosedChannelStatus:     db.WaitingClose,
		ClosedChannelTxID:       waitingClose.Commitments.LocalTxid,
		ClosedChannelRemoteTxID: waitingClose.Commitments.RemoteTxid,
		Amount:                  waitingClose.Channel.LocalBalance,
		Fee:                     0,
		CreationTimestamp:       time.Now().Unix(),
		Description:             "Closed Channel",
	}
	return a.breezDB.AddChannelClosedPayment(paymentData)
}

func (a *Service) onPendingClosedChannel(
	pendingChannel *lnrpc.PendingChannelsResponse_ClosedChannel, sweepTxID string) error {

	a.log.Infof("onPendingClosedChannel %v", pendingChannel.Channel.ChannelPoint)
	if pendingChannel.Channel.LocalBalance == 0 {
		a.log.Infof("closed channel skipped due to zero amount")
		return nil
	}
	paymentData := &db.PaymentInfo{
		Type:                    db.ClosedChannelPayment,
		ClosedChannelPoint:      pendingChannel.Channel.ChannelPoint,
		ClosedChannelStatus:     db.PendingClose,
		ClosedChannelTxID:       pendingChannel.ClosingTxid,
		ClosedChannelRemoteTxID: "",
		ClosedChannelSweepTxID:  sweepTxID,
		Amount:                  pendingChannel.Channel.LocalBalance,
		Fee:                     0,
		CreationTimestamp:       time.Now().Unix(),
		Description:             "Closed Channel",
	}
	return a.breezDB.AddChannelClosedPayment(paymentData)
}

func (a *Service) onPendingForceClosedChannel(
	forceClosed *lnrpc.PendingChannelsResponse_ForceClosedChannel, sweepTxID string) error {

	a.log.Infof("onPendingForceClosedChannel %v swweep: %v", forceClosed.Channel.ChannelPoint, sweepTxID)
	if forceClosed.Channel.LocalBalance == 0 {
		a.log.Infof("closed channel skipped due to zero amount")
		return nil
	}
	paymentData := &db.PaymentInfo{
		Type:                    db.ClosedChannelPayment,
		ClosedChannelPoint:      forceClosed.Channel.ChannelPoint,
		ClosedChannelStatus:     db.PendingClose,
		ClosedChannelTxID:       forceClosed.ClosingTxid,
		ClosedChannelRemoteTxID: "",
		ClosedChannelSweepTxID:  sweepTxID,
		PendingExpirationHeight: forceClosed.MaturityHeight,
		Amount:                  forceClosed.Channel.LocalBalance,
		Fee:                     0,
		CreationTimestamp:       time.Now().Unix(),
		Description:             "Closed Channel",
	}
	return a.breezDB.AddChannelClosedPayment(paymentData)
}

func (a *Service) onClosedChannel(closeSummary *lnrpc.ChannelCloseSummary, sweepTxID string) error {
	a.log.Infof("onClosedChannel %v sweepcloseid: %v", closeSummary.ChannelPoint, sweepTxID)
	if closeSummary.SettledBalance == 0 {
		a.log.Infof("closed channel skipped due to zero amount")
		return nil
	}
	closeTime, err := a.getBlockTime(int64(closeSummary.CloseHeight))
	if err != nil {
		return err
	}
	paymentData := &db.PaymentInfo{
		Type:                   db.ClosedChannelPayment,
		ClosedChannelPoint:     closeSummary.ChannelPoint,
		ClosedChannelStatus:    db.ConfirmedClose,
		ClosedChannelTxID:      closeSummary.ClosingTxHash,
		ClosedChannelSweepTxID: sweepTxID,
		Amount:                 closeSummary.SettledBalance,
		Fee:                    0,
		CreationTimestamp:      closeTime,
		Description:            "Closed Channel",
	}
	return a.breezDB.AddChannelClosedPayment(paymentData)
}

func (a *Service) getBlockTime(height int64) (int64, error) {
	cs, cleanup, err := chainservice.Get(a.cfg.WorkingDir, a.breezDB)
	if err != nil {
		return 0, err
	}
	defer cleanup()
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return 0, err
	}
	header, err := cs.GetBlockHeader(hash)
	if err != nil {
		return 0, err
	}
	return header.Timestamp.Unix(), nil
}
