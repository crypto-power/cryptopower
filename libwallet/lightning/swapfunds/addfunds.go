package swapfunds

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/breez/breez/data"
	"github.com/breez/breez/db"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/submarineswaprpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"golang.org/x/sync/singleflight"

	breezservice "github.com/breez/breez/breez"
)

const (
	transferFundsRequest = "Bitcoin Transfer"
	maxPaymentAllowedSat = math.MaxUint32 / 1000
	maxDepositAmount     = 4_000_000
)

var (
	getPaymentGroup singleflight.Group
)

/*
AddFundsInit is responsible for topping up an existing channel
*/
func (s *Service) AddFundsInit(notificationToken, lspID string) (*data.AddFundInitReply, error) {
	accountID := s.daemonAPI.NodePubkey()
	if accountID == "" {
		return nil, fmt.Errorf("Account is not ready")
	}

	lnclient := s.daemonAPI.SubSwapClient()
	swap, err := lnclient.SubSwapClientInit(context.Background(), &submarineswaprpc.SubSwapClientInitRequest{})
	if err != nil {
		s.log.Errorf("Failed to call SubSwapClientInit %v", err)
		return nil, err
	}

	c, ctx, cancel := s.breezAPI.NewSwapper(0)
	defer cancel()

	r, err := c.AddFundInit(ctx, &breezservice.AddFundInitRequest{NodeID: accountID, NotificationToken: notificationToken, Pubkey: swap.Pubkey, Hash: swap.Hash})
	if err != nil {
		s.log.Errorf("Error in AddFundInit: %v", err)
		return nil, err
	}

	s.log.Infof("AddFundInit response = %v, notification token=%v", r, notificationToken)

	if r.ErrorMessage != "" {
		return &data.AddFundInitReply{MaxAllowedDeposit: r.MaxAllowedDeposit, MinAllowedDeposit: r.MinAllowedDeposit, ErrorMessage: r.ErrorMessage}, nil
	}

	client, err := lnclient.SubSwapClientWatch(context.Background(), &submarineswaprpc.SubSwapClientWatchRequest{Preimage: swap.Preimage, Key: swap.Key, ServicePubkey: r.Pubkey, LockHeight: r.LockHeight})
	if err != nil {
		s.log.Errorf("Failed to call SubSwapClientWatch %v", err)
		return nil, err
	}

	s.log.Infof("Finished watch: %v, %v", hex.EncodeToString(r.Pubkey), r.LockHeight)

	// Verify we are on the same page
	if client.Address != r.Address {
		return nil, errors.New("address mismatch")
	}

	swapInfo := &db.SwapAddressInfo{
		LspID:            lspID,
		Address:          r.Address,
		CreatedTimestamp: time.Now().Unix(),
		PaymentHash:      swap.Hash,
		Preimage:         swap.Preimage,
		PrivateKey:       swap.Key,
		PublicKey:        swap.Pubkey,
		Script:           client.Script,
	}
	s.log.Infof("Saving new swap info %v", swapInfo)
	s.breezDB.SaveSwapAddressInfo(swapInfo)

	// Create JSON with the script and our private key (in case user wants to do the refund by himself)
	type ScriptBackup struct {
		Script     string
		PrivateKey string
	}
	backup := ScriptBackup{Script: hex.EncodeToString(client.Script), PrivateKey: hex.EncodeToString(swap.Key)}
	jsonBytes, err := json.Marshal(backup)
	if err != nil {
		return nil, err
	}
	s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_FUND_ADDRESS_CREATED})
	s.log.Infof("r.RequiredReserve = %v ", r.RequiredReserve)
	return &data.AddFundInitReply{
		Address:           r.Address,
		MaxAllowedDeposit: r.MaxAllowedDeposit,
		ErrorMessage:      r.ErrorMessage,
		BackupJson:        string(jsonBytes[:]),
		RequiredReserve:   r.RequiredReserve,
		MinAllowedDeposit: r.MinAllowedDeposit,
	}, nil
}

/*
GetFundStatus gets a notification token and does two things:
1. Register for notifications on all saved addresses
2. Fetch the current status for the saved addresses from the server
*/
func (s *Service) GetFundStatus(notificationToken string) (*data.FundStatusReply, error) {
	lnclient := s.daemonAPI.APIClient()
	if lnclient == nil {
		return nil, errors.New("Daemon is not ready")
	}
	info, err := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return nil, err
	}

	var statusReply data.FundStatusReply

	createRPCSwapAddressInfo := func(a *db.SwapAddressInfo) *data.SwapAddressInfo {
		var hoursToUnlock float32
		if a.Confirmed() {
			blocksToUnlock := int32(a.LockHeight) + 1 - int32(info.BlockHeight)
			hoursToUnlock = float32(blocksToUnlock) / 6
		}
		return &data.SwapAddressInfo{
			Address:                 a.Address,
			PaymentHash:             hex.EncodeToString(a.PaymentHash),
			ConfirmedAmount:         a.ConfirmedAmount,
			ConfirmedTransactionIds: a.ConfirmedTransactionIds,
			PaidAmount:              a.PaidAmount,
			LockHeight:              a.LockHeight,
			ErrorMessage:            a.ErrorMessage,
			LastRefundTxID:          a.LastRefundTxID,
			SwapError:               data.SwapError(a.SwapErrorReason),
			FundingTxID:             a.FundingTxID,
			HoursToUnlock:           hoursToUnlock,
			NonBlocking:             a.NonBlocking,
		}
	}

	var nonMempoolAddresses []string
	_, err = s.breezDB.FetchSwapAddresses(func(addr *db.SwapAddressInfo) bool {
		if addr.PaidAmount == 0 && addr.LastRefundTxID == "" &&
			!addr.EnteredMempool && time.Now().Sub(time.Unix(addr.CreatedTimestamp, 0)) < time.Hour*24 {
			nonMempoolAddresses = append(nonMempoolAddresses, addr.Address)
		}
		return false
	})
	if err != nil {
		return nil, err
	}

	s.log.Infof("GetFundStatus got %v non mempool addresses to query", len(nonMempoolAddresses))
	if len(nonMempoolAddresses) > 0 {
		c, ctx, cancel := s.breezAPI.NewSwapper(0)
		defer cancel()

		statusesMap, err := c.AddFundStatus(ctx, &breezservice.AddFundStatusRequest{NotificationToken: notificationToken, Addresses: nonMempoolAddresses})
		if err != nil {
			return nil, err
		}

		for addr, status := range statusesMap.Statuses {
			s.log.Infof("GetFundStatus - got status for address %v", status)
			if status.Tx != "" {
				s.breezDB.UpdateSwapAddress(addr, func(swapInfo *db.SwapAddressInfo) error {
					swapInfo.EnteredMempool = true
					swapInfo.FundingTxID = status.Tx
					return nil
				})
			}
		}
	}

	addresses, err := s.breezDB.FetchAllSwapAddresses()
	if err != nil {
		return nil, err
	}
	s.log.Infof("GetFundStatus got %v non paid addresses", len(addresses))

	for _, a := range addresses {

		// If swap transaction is not confirmed, check to see if in mempool
		if !a.Confirmed() {
			if a.EnteredMempool {
				statusReply.UnConfirmedAddresses = append(statusReply.UnConfirmedAddresses, createRPCSwapAddressInfo(a))
				s.log.Infof("Adding unconfirmed address: %v", a.Address)
			}
			continue
		}

		// In case the transactino is confirmed and has positive output, we will check
		// if it is ready for completing the process by the client or it has error.
		if a.ConfirmedAmount > 0 {
			paid := a.PaidAmount > 0

			// in case user got paid we don't need to do anything.
			if paid && len(a.ConfirmedTransactionIds) <= 1 {
				continue
			}

			// in case we didn't get paid and there was either an error in payment or passed timeout we add this
			// address to the refundable ones.
			if data.SwapError(a.SwapErrorReason) != data.SwapError_NO_ERROR || a.LockHeight <= info.BlockHeight {
				s.log.Infof("Adding refundable address: %v", a.Address)
				statusReply.RefundableAddresses = append(statusReply.RefundableAddresses, createRPCSwapAddressInfo(a))
				continue
			}

			// in case no error and still in timeout window, add this address to the confirmed addresses
			statusReply.ConfirmedAddresses = append(statusReply.ConfirmedAddresses, createRPCSwapAddressInfo(a))
			s.log.Infof("Adding confirmed address: %v", a.Address)

		}
	}

	s.log.Infof("GetFundStatus return %v", statusReply)

	return &statusReply, nil
}

// RefundFees return the fees of a refund transaction for a sub swap address.
func (s *Service) RefundFees(address, refundAddress string, targetConf int32, satPerByte int64) (int64, error) {
	s.log.Infof("Starting refund fees flow...")
	lnclient := s.daemonAPI.SubSwapClient()
	if lnclient == nil {
		s.log.Error("unable to execute Refund: Daemon is not ready")
	}
	res, err := lnclient.SubSwapClientRefund(context.Background(), &submarineswaprpc.SubSwapClientRefundRequest{
		Address:       address,
		RefundAddress: refundAddress,
		TargetConf:    targetConf,
		SatPerByte:    satPerByte,
	})
	if err != nil {
		s.log.Errorf("unable to execute SubSwapClientRefund: %v", err)
		return 0, err
	}
	return res.Fees, nil
}

// Refund broadcast a refund transaction for a sub swap address.
func (s *Service) Refund(address, refundAddress string, targetConf int32, satPerByte int64) (string, error) {
	s.log.Infof("Starting refund flow...")
	lnclient := s.daemonAPI.SubSwapClient()
	if lnclient == nil {
		s.log.Error("unable to execute Refund: Daemon is not ready")
	}

	res, err := lnclient.SubSwapClientRefund(context.Background(), &submarineswaprpc.SubSwapClientRefundRequest{
		Address:       address,
		RefundAddress: refundAddress,
		TargetConf:    targetConf,
		SatPerByte:    satPerByte,
	})
	if err != nil {
		s.log.Errorf("unable to execute SubSwapClientRefund: %v", err)
		return "", err
	}
	tx := &wire.MsgTx{}
	txReader := bytes.NewReader(res.Tx)
	if err := tx.Deserialize(txReader); err != nil {
		s.log.Errorf("unable to deserialize the refund tx: %v", err)
		return "", fmt.Errorf("unable to deserialize the refund tx: %w", err)
	}
	txHash := tx.TxHash().String()
	wkClient := s.daemonAPI.WalletKitClient()
	if wkClient == nil {
		s.log.Error("unable to execute Refund: wkClient Daemon is not ready")
		return "", fmt.Errorf("unable to execute Refund: wkClient Daemon is not ready")
	}
	pResp, err := wkClient.PublishTransaction(context.Background(), &walletrpc.Transaction{
		TxHex: res.Tx,
		Label: "SubSwap Refund",
	})
	if err != nil {
		s.log.Error("unable to publish refund transaction: %v", err)
		return "", fmt.Errorf("unable to publish refund transaction: %w", err)
	}
	if pResp.PublishError != "" {
		s.log.Error("unable to publish refund transaction: %v", pResp.PublishError)
		return "", fmt.Errorf("unable to publish refund transaction: %v", pResp.PublishError)
	}
	s.log.Infof("refund executed, res: %v", res)
	_, err = s.breezDB.UpdateSwapAddress(address, func(a *db.SwapAddressInfo) error {
		a.LastRefundTxID = txHash
		return nil
	})
	if err != nil {
		s.log.Errorf("unable to update swap address after refund: %v", err)
		return "", err
	}
	s.log.Infof("refund executed, triggerring unspendChangd event")
	s.onUnspentChanged()
	return txHash, nil
}

func (s *Service) onDaemonReady() error {
	//then initiate an update for all swap addresses in the db
	addresses, err := s.breezDB.FetchSwapAddresses(func(addr *db.SwapAddressInfo) bool {
		return addr.PaidAmount == 0
	})
	s.log.Infof("watchSettledSwapAddresses got these addresses to check: %v", addresses)
	if err != nil {
		s.log.Errorf("failed to call fetchSwapAddresses %v", err)
		return err
	}

	lnclient := s.daemonAPI.APIClient()
	for _, a := range addresses {
		invoice, err := lnclient.LookupInvoice(context.Background(), &lnrpc.PaymentHash{RHash: a.PaymentHash})
		if err != nil {
			continue
		}
		if err := s.onInvoice(invoice); err != nil {
			return err
		}
	}

	//then initiate an update for all swap addresses in the db
	addresses, err = s.breezDB.FetchAllSwapAddresses()
	s.log.Infof("watchSwapAddressConfirmations got these addresses to check: %v", addresses)
	if err != nil {
		s.log.Errorf("failed to call fetchSwapAddresses %v", err)
		return err
	}

	for _, a := range addresses {
		_, err = s.updateUnspentAmount(a.Address)
		if err != nil {
			s.log.Errorf("Failed to update unspent output for address %v", a.Address)
			return err
		}
	}
	return nil
}

// onTransaction subscribe to cofirmed transaction notifications in order
// to update the status of changed SwapAddressInfo in the db.
// On every notification if a new confirmation was detected it calls getPaymentsForConfirmedTransactions
// In order to calim the payments from the swap service.
func (s *Service) onTransaction() error {
	addresses, err := s.breezDB.FetchAllSwapAddresses()
	if err != nil {
		s.log.Errorf("watchSwapAddressConfirmations - Failed to call fetchAllSwapAddresses %v", err)
		return nil
	}
	s.log.Infof("watchSwapAddressConfirmations updating swap addresses")
	var newConfirmation bool
	for _, addr := range addresses {
		updated, err := s.updateUnspentAmount(addr.Address)
		if err != nil {
			s.log.Errorf("Unable to call updateUnspentAmount for address %v", addr.Address)
		}
		newConfirmation = newConfirmation || updated
	}

	//if we got new confirmation we will raise change event.
	if newConfirmation {
		s.onUnspentChanged()

		// if we are connected to the routing node, let's redeem our payment.
		go s.getPaymentsForConfirmedTransactions()
	}
	return nil
}

func (s *Service) onInvoice(invoice *lnrpc.Invoice) error {
	if invoice.Settled {
		s.log.Infof("watchSettledSwapAddresses - removing paid SwapAddressInfo")
		_, err := s.breezDB.UpdateSwapAddressByPaymentHash(invoice.RHash, func(addressInfo *db.SwapAddressInfo) error {
			addressInfo.PaidAmount = invoice.AmtPaidSat
			return nil
		})
		if err != nil {
			s.log.Errorf("watchSettledSwapAddresses - failed to call updateSwapAddressByPaymentHash : %v", err)
			return err
		}
	}
	return nil
}

func (s *Service) SetNonBlockingUnconfirmed() error {
	addresses, err := s.breezDB.FetchSwapAddresses(func(addr *db.SwapAddressInfo) bool {
		return !addr.Confirmed()
	})
	if err != nil {
		return err
	}
	s.log.Infof("updating non blocking addresses")
	for _, a := range addresses {
		_, err := s.breezDB.UpdateSwapAddress(a.Address, func(address *db.SwapAddressInfo) error {
			s.log.Infof("updating non blocking address")
			address.NonBlocking = true
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) lightningTransfersReady() bool {
	return s.daemonAPI.HasActiveChannel()
}

// SettlePendingTransfers watch for routing peer connection and once connected it does two things:
//  1. Ask the breez server to pay in lightning for addresses that the user has sent funds to and
//     that the funds are confirmred
//  2. Ask the breez server to pay on-chain for funds were sent to him in lightning as part of the
//     remove funds flow
func (s *Service) SettlePendingTransfers() {
	go s.getPaymentsForConfirmedTransactions()
	go s.redeemAllRemovedFunds()
}

func (s *Service) updateUnspentAmount(address string) (bool, error) {
	lnclient := s.daemonAPI.SubSwapClient()
	return s.breezDB.UpdateSwapAddress(address, func(swapInfo *db.SwapAddressInfo) error {
		unspentResponse, err := lnclient.UnspentAmount(context.Background(), &submarineswaprpc.UnspentAmountRequest{Address: address})
		if err != nil {
			return err
		}

		swapInfo.ConfirmedAmount = unspentResponse.Amount //get unsepnt amount
		if len(unspentResponse.Utxos) > 0 {
			s.log.Infof("Updating unspent amount %v for address %v", unspentResponse.Amount, address)
			for _, utxo := range unspentResponse.Utxos {
				currentUtxoLockHeight := uint32(utxo.BlockHeight + unspentResponse.LockHeight)
				if swapInfo.LockHeight < currentUtxoLockHeight {
					swapInfo.LockHeight = currentUtxoLockHeight
				}
			}
		}

		confirmedTransactionIDs := swapInfo.ConfirmedTransactionIds
		duplicates := make(map[string]struct{})

		for _, c := range confirmedTransactionIDs {
			duplicates[c] = struct{}{}
		}

		for _, tx := range unspentResponse.Utxos {
			if _, ok := duplicates[tx.Txid]; !ok {
				duplicates[tx.Txid] = struct{}{}
				confirmedTransactionIDs = append(confirmedTransactionIDs, tx.Txid)
			}
		}

		swapInfo.ConfirmedTransactionIds = confirmedTransactionIDs
		return nil
	})
}

func (s *Service) getPaymentsForConfirmedTransactions() {
	s.log.Infof("getPaymentsForConfirmedTransactions: asking for pending payments")

	// if !s.lightningTransfersReady() {
	// 	s.log.Infof("Skipping getPaymentsForConfirmedTransactions HasActiveChannel=%v",
	// 		s.daemonAPI.HasActiveChannel())
	// 	return
	// }

	confirmedAddresses, err := s.breezDB.FetchSwapAddresses(func(addr *db.SwapAddressInfo) bool {
		return addr.ConfirmedAmount > 0 && addr.PaidAmount == 0 && addr.LastRefundTxID == ""
	})
	if err != nil {
		s.log.Errorf("getPaymentsForConfirmedTransactions: failed to fetch swap addresses %v", err)
		return
	}
	s.log.Infof("getPaymentsForConfirmedTransactions: confirmedAddresses length = %v", len(confirmedAddresses))
	for _, address := range confirmedAddresses {
		getPaymentGroup.Do(fmt.Sprintf("getPayment - %v", address), func() (interface{}, error) {
			s.retryGetPayment(address, 3)
			s.onUnspentChanged()
			return nil, nil
		})
	}
}

func (s *Service) retryGetPayment(addressInfo *db.SwapAddressInfo, retries int) {
	var channelNotReadyError bool
	for i := 0; i < retries || channelNotReadyError; i++ {
		waitDuration := 5 * time.Second
		deadlineExceeded, err := s.getPayment(addressInfo)
		if err == nil {
			s.log.Infof("succeed to get payment for address %v", addressInfo.Address)
			break
		}
		s.log.Errorf("retryGetPayment - error getting payment in attempt=%v %v", i, err)

		channelNotReadyError = isTemporaryChannelError(err.Error())
		if deadlineExceeded {
			waitDuration = 30 * time.Second
		}
		time.Sleep(waitDuration)
	}
}

func isTemporaryChannelError(err string) bool {
	return strings.Contains(err, "TemporaryChannelFailure")
}

func (s *Service) getPayment(addressInfo *db.SwapAddressInfo) (bool, error) {
	lnclient := s.daemonAPI.APIClient()
	paymentRequest := addressInfo.PaymentRequest
	if paymentRequest == "" {
		payreq, errorReason, err := s.createSwapInvoice(addressInfo)
		if err != nil {
			s.breezDB.UpdateSwapAddress(addressInfo.Address, func(a *db.SwapAddressInfo) error {
				a.ErrorMessage = err.Error()
				a.SwapErrorReason = int32(errorReason)
				return nil
			})
			return false, fmt.Errorf("failed to call AddInvoice, err = %v", err)
		}
		paymentRequest = payreq
	}

	invoice, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		s.breezDB.UpdateSwapAddress(addressInfo.Address, func(a *db.SwapAddressInfo) error {
			a.ErrorMessage = err.Error()
			a.SwapErrorReason = int32(data.SwapError_NO_ERROR)
			return nil
		})
		return false, fmt.Errorf("failed to decode swap payment request %w", err)
	}

	if addressInfo.ConfirmedAmount != invoice.NumSatoshis {
		errorMsg := "Money was added after the invoice was created"
		s.breezDB.UpdateSwapAddress(addressInfo.Address, func(a *db.SwapAddressInfo) error {
			a.ErrorMessage = errorMsg
			return nil
		})
		return false, errors.New(errorMsg)
	}

	addressInfo.PaymentRequest = paymentRequest
	s.breezDB.UpdateSwapAddress(addressInfo.Address, func(a *db.SwapAddressInfo) error {
		a.PaymentRequest = paymentRequest
		return nil
	})

	c, ctx, cancel := s.breezAPI.NewSwapper(time.Second * 30)
	defer cancel()
	var paymentError string
	reply, err := c.GetSwapPayment(ctx, &breezservice.GetSwapPaymentRequest{PaymentRequest: paymentRequest})
	deadlineExceeded := ctx.Err() == context.DeadlineExceeded
	if err != nil {
		paymentError = err.Error()
	} else if reply.PaymentError != "" {
		paymentError = reply.PaymentError
	}
	if reply != nil {
		s.log.Infof("reply from getPayment: error=%v, error reason=%v", reply.PaymentError, reply.SwapError)
	}
	if paymentError != "" {
		s.breezDB.UpdateSwapAddress(addressInfo.Address, func(a *db.SwapAddressInfo) error {
			if !isTemporaryChannelError(paymentError) {
				a.ErrorMessage = paymentError
			}
			if reply != nil {
				a.SwapErrorReason = int32(reply.SwapError)
			}

			return nil
		})
		return deadlineExceeded, fmt.Errorf("failed to get payment for address %v, err = %v", addressInfo.Address, paymentError)
	}
	return deadlineExceeded, nil
}

func (s *Service) createSwapInvoice(addressInfo *db.SwapAddressInfo) (payReq string, reason data.SwapError, err error) {
	maxReceive, err := s.getGlobalReceiveLimit()
	if err != nil {
		s.log.Error("unable to get account limits")
		return "", data.SwapError_NO_ERROR, err
	}
	if maxReceive > maxDepositAmount {
		maxReceive = maxDepositAmount
	}

	if addressInfo.ConfirmedAmount > maxPaymentAllowedSat || addressInfo.ConfirmedAmount > maxReceive {
		s.log.Errorf("invoice limit exceeded amount: %v, max able to receive: %v", addressInfo.ConfirmedAmount, maxReceive)
		return "", data.SwapError_FUNDS_EXCEED_LIMIT, errors.New("invoice limit exceeded")
	}

	lsps, err := s.lspList()
	if err != nil {
		return "", data.SwapError_NO_ERROR, err
	}
	lsp, ok := lsps.Lsps[addressInfo.LspID]
	if !ok {
		return "", data.SwapError_NO_ERROR, errors.New("LSP is not selected")
	}
	addInvoice, _, err := s.addInvoice(&data.AddInvoiceRequest{
		InvoiceDetails: &data.InvoiceMemo{
			Preimage:    addressInfo.Preimage,
			Amount:      addressInfo.ConfirmedAmount,
			Description: transferFundsRequest,
			Expiry:      60 * 60 * 24 * 30,
		},
		LspInfo: lsp,
	})

	if err != nil {
		s.log.Error("unable to create invoice: %v", err)
		return "", data.SwapError_NO_ERROR, err
	}

	return addInvoice, data.SwapError_NO_ERROR, nil
}

func (s *Service) onUnspentChanged() {
	s.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_FUND_ADDRESS_UNSPENT_CHANGED})
}
