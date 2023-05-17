package swapfunds

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/breez/boltz"
	breezservice "github.com/breez/breez/breez"
	"github.com/breez/breez/channeldbservice"
	"github.com/breez/breez/data"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/zpay32"
)

func (s *Service) lockupOutScript(lockupAddress string, rawTx []byte) ([]byte, error) {
	tx, err := btcutil.NewTxFromBytes(rawTx)
	if err != nil {
		return nil, fmt.Errorf("btcutil.NewTxFromBytes(%x): %w", rawTx, err)
	}
	//var out *wire.OutPoint
	//var amt btcutil.Amount
	var script []byte
	for _, txout := range tx.MsgTx().TxOut {
		class, addresses, requiredsigs, err := txscript.ExtractPkScriptAddrs(txout.PkScript, s.chainParams)
		if err != nil {
			return nil, fmt.Errorf("txscript.ExtractPkScriptAddrs(%x) %w", txout.PkScript, err)
		}
		if class == txscript.WitnessV0ScriptHashTy && requiredsigs == 1 &&
			len(addresses) == 1 && addresses[0].EncodeAddress() == lockupAddress {
			script = txout.PkScript
		}
	}
	return script, nil
}

func (s *Service) claimReverseSwap(rs *data.ReverseSwap, rawTx []byte) error {
	lnClient := s.daemonAPI.APIClient()
	if lnClient == nil {
		s.log.Errorf("daemon is not ready")
		return fmt.Errorf("daemon is not ready")
	}
	info, err := lnClient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		s.log.Errorf("lnClient.GetInfo: %v", err)
		return fmt.Errorf("lnClient.GetInfo: %w", err)
	}
	if rs.TimeoutBlockHeight <= int64(info.BlockHeight) {
		s.log.Errorf("too late for the claim transaction: TimeoutBlockHeight=%v <= BlockHeight=%v", rs.TimeoutBlockHeight, info.BlockHeight)
		return fmt.Errorf("too late for the claim transaction: TimeoutBlockHeight=%v <= BlockHeight=%v", rs.TimeoutBlockHeight, info.BlockHeight)
	}

	_, err = boltz.CheckTransaction(hex.EncodeToString(rawTx), rs.LockupAddress, rs.OnchainAmount)
	if err != nil {
		s.log.Errorf("boltz.CheckTransaction(%x, %v, %v): %v", rawTx, rs.LockupAddress, rs.OnchainAmount, err)
		return fmt.Errorf("boltz.CheckTransaction(%x, %v, %v): %w", rawTx, rs.LockupAddress, rs.OnchainAmount, err)
	}
	claimTx, err := boltz.ClaimTransaction(rs.Script, hex.EncodeToString(rawTx), rs.ClaimAddress, rs.Preimage, rs.Key, rs.ClaimFee)
	if err != nil {
		s.log.Errorf("boltz.ClaimTransaction(%v, %x, %v, %v, %v): %v", rawTx, rs.ClaimAddress, rs.Preimage, rs.Key, rs.ClaimFee, err)
		return fmt.Errorf("walletKitClient.EstimateFee(%v, %x, %v, %v, %v): %w", rawTx, rs.ClaimAddress, rs.Preimage, rs.Key, rs.ClaimFee, err)
	}
	txHex, err := hex.DecodeString(claimTx)
	if err != nil {
		s.log.Errorf("hex.DecodeString(%v): %v", claimTx, err)
		return fmt.Errorf("hex.DecodeString(%v): %w", claimTx, err)
	}
	tx, err := btcutil.NewTxFromBytes(txHex)
	if err != nil {
		s.log.Errorf("btcutil.NewTxFromBytes(%v): %v", txHex, err)
		return fmt.Errorf("btcutil.NewTxFromBytes(%v): %w", txHex, err)
	}

	txid := tx.Hash().CloneBytes()
	rs.ClaimTxid = tx.Hash().String()
	_, err = s.breezDB.SaveReverseSwap(rs)
	if err != nil {
		s.log.Errorf("breezDB.SaveReverseSwap(%#v): %v", rs, err)
	}

	var unspendLockupInformation *data.UnspendLockupInformation
	unspendLockupInformation, err = s.breezDB.FetchUnspendLockupInformation()
	if err != nil || unspendLockupInformation == nil || !bytes.Equal(unspendLockupInformation.ClaimTxHash, txid) {
		outScript, err := s.lockupOutScript(rs.LockupAddress, rawTx)
		if err != nil {
			s.log.Errorf("lockupOutScript(%v, %x): %v", rs.LockupAddress, rawTx, err)
		} else {
			unspendLockupInformation = &data.UnspendLockupInformation{
				LockupScript: outScript,
				ClaimTxHash:  txid,
				HeightHint:   info.BlockHeight,
			}
			confRequest, err := s.breezDB.FetchUnconfirmedClaimTransaction()
			if err == nil && confRequest != nil && bytes.Equal(confRequest.Txid, txid) {
				unspendLockupInformation.HeightHint = confRequest.HeightHint
			}
			err = s.breezDB.SaveUnspendLockupInformation(unspendLockupInformation)
			if err != nil {
				s.log.Errorf("s.breezDB.SaveUnspendLockupInformation(%#v): %v", unspendLockupInformation, err)
			}
		}
	}

	spendRequest := &chainrpc.SpendRequest{
		Script:     unspendLockupInformation.LockupScript,
		HeightHint: unspendLockupInformation.HeightHint,
	}
	err = s.subscribeSpendTransaction(spendRequest, txid)
	if err != nil {
		s.log.Errorf("s.subscribeSpendTransaction(%#v, %x): %v", spendRequest, txid, err)
	}

	walletKitClient := s.daemonAPI.WalletKitClient()
	pr, err := walletKitClient.PublishTransaction(context.Background(), &walletrpc.Transaction{TxHex: txHex})
	if err != nil {
		s.log.Errorf("walletKitClient.PublishTransaction(%x): %v", txHex, err)
		return fmt.Errorf("walletKitClient.PublishTransaction(%x): %w", txHex, err)
	}
	if pr.PublishError != "" {
		s.log.Errorf("walletKitClient.PublishTransaction(%x): %v", txHex, pr.PublishError)
		return fmt.Errorf("walletKitClient.PublishTransaction(%x): %v", txHex, pr.PublishError)
	}
	return nil
}

func (s *Service) subscribeSpendTransaction(spendRequest *chainrpc.SpendRequest, txid []byte) error {
	client := s.daemonAPI.ChainNotifierClient()
	ctx, cancel := context.WithCancel(context.Background())
	s.log.Infof("Registering spend notification %x", spendRequest.Script)
	s.log.Infof("Chain Notifier Client is %v", client)
	stream, err := client.RegisterSpendNtfn(ctx, spendRequest)
	s.log.Infof("After Register spend %v", spendRequest)
	if err != nil {
		s.log.Errorf("client.RegisterSpendNtfn(%#v): %v", spendRequest, err)
		cancel()
		return fmt.Errorf("client.RegisterSpendNtfn(%#v): %w", spendRequest, err)
	}
	go func() {
		for {
			SpendEvent, err := stream.Recv()
			if err != nil {
				s.log.Errorf("Failed to receive an event : %v", err)
				time.AfterFunc(time.Second*10, func() {
					s.subscribeSpendTransaction(spendRequest, txid)
				})
				return
			}
			s.log.Infof("Got spend event %v", SpendEvent)
			s.log.Infof("spendEvent: %#v; rawTX:%x", SpendEvent.GetSpend(), SpendEvent.GetSpend().RawSpendingTx)
			err = s.breezDB.SaveUnspendLockupInformation(nil)
			err = s.breezDB.SaveUnconfirmedClaimTransaction(nil)
			if bytes.Equal(SpendEvent.GetSpend().SpendingTxHash, txid) {
				s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_REVERSE_SWAP_CLAIM_CONFIRMED,
					Data: []string{hex.EncodeToString(SpendEvent.GetSpend().RawSpendingTx)}})
			}
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			s.log.Infof("Cancelling subscribeSpendTransaction")
			cancel()
		case <-s.quitChan:
			cancel()
		}
	}()
	return nil
}

func (s *Service) handleClaimTransaction() error {
	unspendLockupInformation, err := s.breezDB.FetchUnspendLockupInformation()
	if err != nil {
		s.log.Errorf("s.breezDB.FetchUnspendLockupInformation(): %v", err)
		return fmt.Errorf("s.breezDB.FetchUnspendLockupInformation(): %w", err)
	}
	if unspendLockupInformation == nil {
		return nil
	}
	spendRequest := &chainrpc.SpendRequest{
		Script:     unspendLockupInformation.LockupScript,
		HeightHint: unspendLockupInformation.HeightHint,
	}
	err = s.subscribeSpendTransaction(spendRequest, unspendLockupInformation.ClaimTxHash)
	if err != nil {
		s.log.Errorf("s.subscribeSpendTransaction(%v, %x): %v", spendRequest, unspendLockupInformation.ClaimTxHash, err)
		return fmt.Errorf("s.subscribeSpendTransaction(%v, %x): %w", spendRequest, unspendLockupInformation.ClaimTxHash, err)
	}
	return nil
}

func (s *Service) ClaimFeeEstimates(claimAddress string) (map[int32]int64, error) {
	blockRange := []int32{2, 6, 24}
	walletKitClient := s.daemonAPI.WalletKitClient()
	fees := make(map[int32]int64)
	for _, b := range blockRange {
		f, err := walletKitClient.EstimateFee(context.Background(), &walletrpc.EstimateFeeRequest{ConfTarget: b})
		if err != nil {
			s.log.Errorf("walletKitClient.EstimateFee(%v): %v", b, err)
			return nil, fmt.Errorf("walletKitClient.EstimateFee(%v): %w", b, err)
		}
		fAmt, err := boltz.ClaimFee(claimAddress, f.SatPerKw)
		if err != nil {
			s.log.Errorf("boltz.ClaimFee(%v, %v): %v", claimAddress, f.SatPerKw, err)
			return nil, fmt.Errorf("boltz.ClaimFee(%v, %v): %w", claimAddress, f.SatPerKw, err)
		}
		fees[b] = fAmt
	}
	return fees, nil
}

func (s *Service) remoteSubscribeLockupNotification(deviceID, title, body, boltzID string, lockupAddress string, heightHint int64, timeoutBlockHeight int64) error {
	a, err := btcutil.DecodeAddress(lockupAddress, s.chainParams)
	if err != nil {
		return fmt.Errorf("btcutil.DecodeAddress(%v) %w", lockupAddress, err)
	}
	script, err := txscript.PayToAddrScript(a)
	if err != nil {
		return fmt.Errorf("txscript.PayToAddrScript(%v) %w", a, err)
	}

	c, ctx, cancel := s.breezAPI.NewPushTxNotifierClient()
	defer cancel()
	_, err = c.RegisterTxNotification(ctx, &breezservice.PushTxNotificationRequest{
		DeviceId:        deviceID,
		Title:           title,
		Body:            body,
		TxHash:          lntypes.ZeroHash[:],
		Script:          script,
		BlockHeightHint: uint32(heightHint),
		Info: &breezservice.PushTxNotificationRequest_BoltzReverseSwapLockupTxInfo{
			BoltzReverseSwapLockupTxInfo: &breezservice.BoltzReverseSwapLockupTx{
				BoltzId:            boltzID,
				TimeoutBlockHeight: uint32(timeoutBlockHeight),
			},
		},
	})
	return err
}

func (s *Service) subscribeLockupScript(rs *data.ReverseSwap) error {
	a, err := btcutil.DecodeAddress(rs.LockupAddress, s.chainParams)
	if err != nil {
		return fmt.Errorf("btcutil.DecodeAddress(%v) %w", rs.LockupAddress, err)
	}
	script, err := txscript.PayToAddrScript(a)
	if err != nil {
		return fmt.Errorf("txscript.PayToAddrScript(%v) %w", a, err)
	}
	client := s.daemonAPI.ChainNotifierClient()
	ctx, cancel := context.WithCancel(context.Background())
	startHeight := uint32(rs.StartBlockHeight)
	s.log.Infof("Registering with start block = %v", startHeight)
	stream, err := client.RegisterConfirmationsNtfn(ctx, &chainrpc.ConfRequest{
		NumConfs:   1,
		HeightHint: startHeight,
		Txid:       lntypes.ZeroHash[:],
		Script:     script,
	})
	if err != nil {
		s.log.Errorf("client.RegisterConfirmationsNtfn(%v, %x): %v", startHeight, script, err)
		cancel()
		return fmt.Errorf("client.RegisterConfirmationsNtfn(%v, %x): %w", startHeight, script, err)
	}
	//fmt.Printf("Register: %#v\n", rs)
	go func() {
		for {
			confEvent, err := stream.Recv()
			if err != nil {
				s.log.Errorf("Failed to receive an event : %v", err)
				time.AfterFunc(time.Second*10, func() {
					s.subscribeLockupScript(rs)
				})
				s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_REVERSE_SWAP_CLAIM_FAILED, Data: []string{rs.Key, "internal error"}})
				return
			}
			s.log.Infof("confEvent: %#v; rawTX:%x", confEvent.GetConf(), confEvent.GetConf().GetRawTx())
			s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_REVERSE_SWAP_CLAIM_STARTED, Data: []string{rs.Key}})
			err = s.claimReverseSwap(rs, confEvent.GetConf().GetRawTx())
			if err != nil {
				s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_REVERSE_SWAP_CLAIM_FAILED, Data: []string{rs.Key, err.Error()}})
			} else {
				s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_REVERSE_SWAP_CLAIM_SUCCEEDED, Data: []string{rs.Key}})
			}
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			s.log.Infof("Cancelling subscribeLockupScript")
			cancel()
		case <-s.quitChan:
			cancel()
		}
	}()
	return nil
}

func (s *Service) ReverseRoutingNode() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reverseRoutingNode != nil {
		return s.reverseRoutingNode
	}
	c, ctx, cancel := s.breezAPI.NewSwapper(0)
	defer cancel()
	r, err := c.GetReverseRoutingNode(ctx, &breezservice.GetReverseRoutingNodeRequest{})
	if err != nil {
		s.log.Errorf("c.GetReverseRoutingNode(): %v", err)
		return nil
	}
	s.reverseRoutingNode = r.NodeId
	s.log.Errorf("c.GetReverseRoutingNode() nodeID: %x", r.NodeId)
	return s.reverseRoutingNode
}

func (s *Service) NewReverseSwap(amt int64, feesHash, claimAddress string) (string, error) {
	lnClient := s.daemonAPI.APIClient()
	if lnClient == nil {
		return "", errors.New("daemon is not ready")
	}
	info, err := lnClient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return "", fmt.Errorf("lnClient.GetInfo: %w", err)
	}
	r, err := boltz.NewReverseSwap(btcutil.Amount(amt), feesHash, s.ReverseRoutingNode())
	if err != nil {
		s.log.Errorf("boltz.NewReverseSwap(%v): %v", amt, err)
		return "", fmt.Errorf("boltz.NewReverseSwap(%v): %w", amt, err)
	}
	_, err = btcutil.DecodeAddress(claimAddress, s.chainParams)
	if err != nil {
		s.log.Errorf("btcutil.DecodeAddress(%v): %v", claimAddress, err)
		return "", fmt.Errorf("btcutil.DecodeAddress(%v): %w", claimAddress, err)
	}
	rs := &data.ReverseSwap{
		Id:                 r.ID,
		Invoice:            r.Invoice,
		Script:             r.RedeemScript,
		LockupAddress:      r.LockupAddress,
		Preimage:           r.Preimage,
		Key:                r.Key,
		ClaimAddress:       claimAddress,
		LnAmount:           amt,
		OnchainAmount:      r.OnchainAmount,
		TimeoutBlockHeight: r.TimeoutBlockHeight,
		StartBlockHeight:   int64(info.BlockHeight),
	}
	s.log.Errorf("data.ReverseSwap: %#v", rs)
	h, err := s.breezDB.SaveReverseSwap(rs)
	if err != nil {
		return "", fmt.Errorf("breezDB.SaveReverseSwap(%#v): %w", rs, err)
	}
	return h, nil
}

func (s *Service) FetchReverseSwap(hash string) (*data.ReverseSwap, error) {
	return s.breezDB.FetchReverseSwap(hash)
}

func (s *Service) SetReverseSwapClaimFee(hash string, fee int64) error {
	rs, err := s.breezDB.FetchReverseSwap(hash)
	if err != nil {
		s.log.Errorf("s.breezDB.FetchReverseSwap(%v): %w", hash, err)
		return fmt.Errorf("s.breezDB.FetchReverseSwap(%v): %w", hash, err)
	}
	rs.ClaimFee = fee
	_, err = s.breezDB.SaveReverseSwap(rs)
	if err != nil {
		return fmt.Errorf("breezDB.SaveReverseSwap(%#v): %w", rs, err)
	}
	return nil
}

func (s *Service) PayReverseSwap(hash, deviceID, title, body string, fee int64) error {
	rs, err := s.breezDB.FetchReverseSwap(hash)
	if err != nil {
		s.log.Errorf("s.breezDB.FetchReverseSwap(%v): %w", hash, err)
		return fmt.Errorf("s.breezDB.FetchReverseSwap(%v): %w", hash, err)
	}

	if rs.ClaimFee == 0 {
		s.log.Errorf("need to set claimFee for %v", hash)
		return fmt.Errorf("need to set claimFee for %v", hash)
	}

	err = s.subscribeLockupScript(rs)
	if err != nil {
		return fmt.Errorf("s.subscribeLockupScript(%v): %w", rs, err)
	}
	err = s.remoteSubscribeLockupNotification(deviceID, title, body, rs.Id, rs.LockupAddress, rs.StartBlockHeight, rs.TimeoutBlockHeight)
	if err != nil {
		s.log.Errorf("s.remoteSubscribeLockupNotification(%v, %v, %v, %v, %v, %v): %v", deviceID, title, body, rs.Id, rs.LockupAddress, rs.StartBlockHeight, err)
	}
	var nodeID []byte
	payReq, err := zpay32.Decode(rs.Invoice, s.chainParams)
	if err == nil && payReq != nil && len(payReq.RouteHints) >= 1 && len(payReq.RouteHints[0]) == 1 {
		nodeID = payReq.RouteHints[0][0].NodeID.SerializeCompressed()
	}
	_, err = s.sendPayment(rs.Invoice, rs.LnAmount, nodeID, fee)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) UnconfirmedReverseSwapClaimTransaction() (string, error) {
	unspendLockupInformation, err := s.breezDB.FetchUnspendLockupInformation()
	if err != nil {
		s.log.Errorf("s.breezDB.FetchUnspendLockupInformation(): %v", err)
		return "", fmt.Errorf("s.breezDB.FetchUnspendLockupInformation(): %w", err)
	}
	if unspendLockupInformation == nil {
		return "", nil
	}
	h, err := chainhash.NewHash(unspendLockupInformation.ClaimTxHash)
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func (s *Service) ResetUnconfirmedReverseSwapClaimTransaction() error {
	return s.breezDB.SaveUnspendLockupInformation(nil)
}

func (s *Service) ReverseSwapPayments() (*data.ReverseSwapPaymentStatuses, error) {
	chanDB, chanDBCleanUp, err := channeldbservice.Get(s.cfg.WorkingDir)
	if err != nil {
		s.log.Errorf("channeldbservice.Get(%v): %v", s.cfg.WorkingDir, err)
		return nil, fmt.Errorf("channeldbservice.Get(%v): %w", s.cfg.WorkingDir, err)
	}
	defer chanDBCleanUp()
	paymentControl := channeldb.NewPaymentControl(chanDB)
	payments, err := paymentControl.FetchInFlightPayments()
	if err != nil {
		s.log.Errorf("paymentControl.FetchInFlightPayments(): %v", err)
		return nil, fmt.Errorf("paymentControl.FetchInFlightPayments(): %w", err)
	}
	s.log.Infof("Fetched %v in flight payments", len(payments))
	var statuses []*data.ReverseSwapPaymentStatus
	for _, p := range payments {
		s.log.Infof("Checking payment %v", p)
		hash := p.Info.PaymentIdentifier.String()
		s.log.Infof("Checking payment hash %v", hash)
		rs, err := s.breezDB.FetchReverseSwap(hash)
		if err != nil {
			s.log.Errorf("s.breezDB.FetchReverseSwap(%v): %w", hash, err)
			return nil, fmt.Errorf("s.breezDB.FetchReverseSwap(%v): %v", hash, err)
		}
		s.log.Infof("AfterFetchReverseSwap %v", rs)
		if rs == nil {
			continue
		}
		s.log.Infof("gettting info about reverse swap: hash=%v, swap=%v", hash, rs)
		status, txid, _, eta, err := boltz.GetTransaction(rs.Id, rs.LockupAddress, rs.OnchainAmount)
		if err != nil {
			s.log.Errorf("boltz.GetTransaction(%v, %v, %v): %v", rs.Id, rs.LockupAddress, rs.OnchainAmount, err)
			if errors.Is(err, boltz.ErrSwapNotFound) {
				continue
			}
			return nil, fmt.Errorf("boltz.GetTransaction(%v, %v, %v): %w", rs.Id, rs.LockupAddress, rs.OnchainAmount, err)
		}
		s.log.Infof("Reverse swap: hash=%v, status=%v, txid=%v, eta=%v", hash, status, txid, eta)
		statuses = append(statuses, &data.ReverseSwapPaymentStatus{Hash: hash, TxID: txid, Eta: int32(eta)})
	}
	return &data.ReverseSwapPaymentStatuses{PaymentsStatus: statuses}, nil
}

func (s *Service) handleReverseSwapsPayments() error {
	chanDB, chanDBCleanUp, err := channeldbservice.Get(s.cfg.WorkingDir)
	if err != nil {
		s.log.Errorf("channeldbservice.Get(%v): %v", s.cfg.WorkingDir, err)
		return fmt.Errorf("channeldbservice.Get(%v): %w", s.cfg.WorkingDir, err)
	}
	defer chanDBCleanUp()
	paymentControl := channeldb.NewPaymentControl(chanDB)
	payments, err := paymentControl.FetchInFlightPayments()
	if err != nil {
		s.log.Errorf("paymentControl.FetchInFlightPayments(): %v", err)
		return fmt.Errorf("paymentControl.FetchInFlightPayments(): %w", err)
	}
	s.log.Infof("Fetched %v in flight payments", len(payments))
	for _, p := range payments {
		hash := p.Info.PaymentIdentifier.String()
		rs, err := s.breezDB.FetchReverseSwap(hash)
		if err != nil {
			s.log.Errorf("s.breezDB.FetchReverseSwap(%v): %w", hash, err)
			continue
		}
		if rs == nil {
			continue
		}
		s.log.Infof("handling reverse swap: hash=%v, swap=%v", hash, rs)
		err = s.subscribeLockupScript(rs)
		if err != nil {
			s.log.Errorf("s.subscribeLockupScript(%v): %v", rs, err)
			return err
		}
	}
	return nil
}
