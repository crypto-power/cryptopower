package account

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	breezservice "github.com/breez/breez/breez"
	"github.com/breez/breez/channeldbservice"
	"github.com/breez/breez/data"
	"github.com/breez/breez/db"

	"github.com/breez/lspd/btceclegacy"
	lspd "github.com/breez/lspd/rpc"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/aliasmgr"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/record"
	"github.com/lightningnetwork/lnd/routing"
	"google.golang.org/grpc/status"
)

const (
	defaultInvoiceExpiry       int64 = 3600
	invoiceCustomPartDelimiter       = " |\n"
	transferFundsRequest             = "Bitcoin Transfer"
)

// PaymentResponse is the response of a payment attempt.
type PaymentResponse struct {
	PaymentError string
	TraceReport  string
}

/*
GetPayments is responsible for retrieving the payment were made in this account
*/
func (a *Service) GetPayments() (*data.PaymentsList, error) {
	rawPayments, err := a.breezDB.FetchAllAccountPayments()
	if err != nil {
		return nil, err
	}

	pendingPayments, err := a.getPendingPayments(true)
	if err != nil {
		return nil, err
	}
	rawPayments = append(rawPayments, pendingPayments...)

	var paymentsList []*data.Payment
	for _, payment := range rawPayments {
		paymentItem := &data.Payment{
			Amount:                     payment.Amount,
			Fee:                        payment.Fee,
			CreationTimestamp:          payment.CreationTimestamp,
			RedeemTxID:                 payment.RedeemTxID,
			PaymentHash:                payment.PaymentHash,
			Destination:                payment.Destination,
			PendingExpirationHeight:    payment.PendingExpirationHeight,
			PendingExpirationTimestamp: payment.PendingExpirationTimestamp,
			PendingFull:                payment.PendingFull,
			Preimage:                   payment.Preimage,
			ClosedChannelPoint:         payment.ClosedChannelPoint,
			IsChannelPending:           payment.Type == db.ClosedChannelPayment && payment.ClosedChannelStatus != db.ConfirmedClose,
			ClosedChannelTxID:          payment.ClosedChannelTxID,
			ClosedChannelRemoteTxID:    payment.ClosedChannelRemoteTxID,
			ClosedChannelSweepTxID:     payment.ClosedChannelSweepTxID,
			IsKeySend:                  payment.IsKeySend,
			GroupKey:                   payment.GroupKey,
			GroupName:                  payment.GroupName,
		}
		if payment.Type != db.ClosedChannelPayment {
			paymentItem.InvoiceMemo = &data.InvoiceMemo{
				Description:     payment.Description,
				Amount:          payment.Amount,
				PayeeImageURL:   payment.PayeeImageURL,
				PayeeName:       payment.PayeeName,
				PayerImageURL:   payment.PayerImageURL,
				PayerName:       payment.PayerName,
				TransferRequest: payment.TransferRequest,
			}
		}
		switch payment.Type {
		case db.SentPayment:
			paymentItem.Type = data.Payment_SENT
			if paymentItem.LnurlPayInfo, err = a.breezDB.FetchLNUrlPayInfo(payment.PaymentHash); err != nil {
				return nil, err
			}

			// Give old reverse swap payments the correct type if necessary.
			if swap, _ := a.breezDB.FetchReverseSwap(paymentItem.PaymentHash); swap != nil {
				paymentItem.Type = data.Payment_WITHDRAWAL
			}
		case db.ReceivedPayment:
			paymentItem.Type = data.Payment_RECEIVED
		case db.DepositPayment:
			paymentItem.Type = data.Payment_DEPOSIT
		case db.WithdrawalPayment:
			paymentItem.Type = data.Payment_WITHDRAWAL
		case db.ClosedChannelPayment:
			paymentItem.Type = data.Payment_CLOSED_CHANNEL
		}

		paymentsList = append(paymentsList, paymentItem)
	}

	sort.Slice(paymentsList, func(i, j int) bool {
		return paymentsList[i].CreationTimestamp > paymentsList[j].CreationTimestamp
	})

	resultPayments := &data.PaymentsList{PaymentsList: paymentsList}
	return resultPayments, nil
}

func (a *Service) LSPActivity(lspList *data.LSPList) (*data.LSPActivity, error) {

	lnclient := a.daemonAPI.APIClient()
	if lnclient == nil {
		return nil, errors.New("daemon is not ready")
	}

	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		return nil, err
	}
	lspPubkey := make(map[string]string)
	connectedLsps := make(map[string]struct{})
	var exists = struct{}{}
	for _, lsp := range lspList.Lsps {
		lspPubkey[lsp.Pubkey] = lsp.Id
	}

	chanidChannel := make(map[uint64]*lnrpc.Channel)
	chanidLSP := make(map[uint64]string)
	for _, channel := range channels.Channels {
		chanidChannel[channel.ChanId] = channel
		if ID, ok := lspPubkey[channel.RemotePubkey]; ok {
			chanidLSP[channel.ChanId] = ID
			connectedLsps[ID] = exists
		}
	}

	lastPayments := make(map[string]int64)

	invoices, err := lnclient.ListInvoices(context.Background(), &lnrpc.ListInvoiceRequest{NumMaxInvoices: 100000})
	if err != nil {
		return nil, err
	}
	for _, invoice := range invoices.Invoices {
		if invoice.State != lnrpc.Invoice_SETTLED {
			continue
		}
		for _, htlc := range invoice.Htlcs {
			lsp := chanidLSP[htlc.ChanId]
			c, ok := chanidChannel[htlc.ChanId]
			if ok {
				if lsp == "" && c.ZeroConf && c.ZeroConfConfirmedScid == 0 &&
					len(invoice.RouteHints) > 0 && len(invoice.RouteHints[0].HopHints) > 0 {

					htlcLSP := lspPubkey[invoice.RouteHints[0].HopHints[0].NodeId]
					if _, ok := connectedLsps[htlcLSP]; ok {
						lsp = htlcLSP
						chanidLSP[htlc.ChanId] = lsp
					}
				}
			}
			if lsp != "" && lastPayments[lsp] < invoice.SettleDate {
				lastPayments[lsp] = invoice.SettleDate
			}
		}
	}

	payments, err := lnclient.ListPayments(context.Background(), &lnrpc.ListPaymentsRequest{MaxPayments: 100000})
	if err != nil {
		return nil, err
	}
	for _, payment := range payments.Payments {
		if payment.Status != lnrpc.Payment_SUCCEEDED {
			continue
		}
		for _, htlc := range payment.Htlcs {
			lsp := chanidLSP[htlc.Route.Hops[0].ChanId]
			htlcDate := time.Unix(0, htlc.ResolveTimeNs).Unix()
			if lsp != "" && lastPayments[lsp] < htlcDate {
				lastPayments[lsp] = htlcDate
			}
		}
	}

	return &data.LSPActivity{Activity: lastPayments}, nil
}

/*
SendPaymentForRequestV2 send the payment according to the details specified in the bolt 11 payment request.
If the payment was failed an error is returned
*/
func (a *Service) SendPaymentForRequestV2(paymentRequest string, amountSatoshi int64, lastHopPubkey []byte, fee int64) (string, error) {
	return a.sendPaymentForRequest(paymentRequest, amountSatoshi, lastHopPubkey, fee)
}

/*
SendPaymentForRequest send the payment according to the details specified in the bolt 11 payment request.
If the payment was failed an error is returned
*/
func (a *Service) SendPaymentForRequest(paymentRequest string, amountSatoshi int64, fee int64) (string, error) {
	return a.sendPaymentForRequest(paymentRequest, amountSatoshi, nil, fee)
}

func (a *Service) sendPaymentForRequest(paymentRequest string, amountSatoshi int64, lastHopPubkey []byte, fee int64) (string, error) {
	a.log.Infof("sendPaymentForRequest: amount = %v", amountSatoshi)
	routing.DefaultShardMinAmt = 5000
	lnclient := a.daemonAPI.APIClient()
	decodedReq, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		return "", err
	}
	if decodedReq.NumSatoshis == amountSatoshi {
		amountSatoshi = 0
	}
	if err := a.breezDB.SavePaymentRequest(decodedReq.PaymentHash, []byte(paymentRequest)); err != nil {
		return "", err
	}
	a.log.Infof("sendPaymentForRequest: before sending payment...")

	maxParts := uint32(20)
	if decodedReq.Features[uint32(lnwire.MPPOptional)] == nil &&
		decodedReq.Features[uint32(lnwire.MPPRequired)] == nil {
		maxParts = 1
	}

	feeLimit := fee
	if feeLimit < 0 {
		feeLimit = math.MaxInt64
	}
	// At this stage we are ready to send asynchronously the payment through the daemon.
	var timeoutSeconds int32 = 60
	if useTor, _ := a.breezDB.GetTorActive(); useTor {
		/* If Tor is active, extend the timeout to avoid
		frequent payment timeout failures observed in testing.
		*/
		timeoutSeconds *= 2
	}

	return a.sendPayment(decodedReq.PaymentHash, decodedReq, &routerrpc.SendPaymentRequest{
		PaymentRequest: paymentRequest,
		TimeoutSeconds: timeoutSeconds,
		FeeLimitSat:    feeLimit,
		MaxParts:       maxParts,
		Amt:            amountSatoshi,
		LastHopPubkey:  lastHopPubkey,
	})
}

// SendSpontaneousPayment send a payment without a payment request.
func (a *Service) SendSpontaneousPayment(destNode string,
	description string, amount int64, feeLimitMSat int64,
	groupKey, groupName string, tlv map[int64]string) (string, error) {

	destBytes, err := hex.DecodeString(destNode)
	if err != nil {
		return "", err
	}
	feeLimit := feeLimitMSat
	if feeLimit == 0 {
		feeLimit = math.MaxInt64
	}
	req := &routerrpc.SendPaymentRequest{
		Dest:              destBytes,
		Amt:               amount,
		TimeoutSeconds:    60,
		FeeLimitMsat:      feeLimit,
		MaxParts:          20,
		DestCustomRecords: make(map[uint64][]byte),
	}

	// Since this is a spontaneous payment we need to generate the pre-image and hash by ourselves.
	var preimage lntypes.Preimage
	if _, err := rand.Read(preimage[:]); err != nil {
		return "", err
	}
	req.DestCustomRecords[record.KeySendType] = preimage[:]
	hash := preimage.Hash()
	req.PaymentHash = hash[:]
	features := []lnrpc.FeatureBit{
		lnrpc.FeatureBit_TLV_ONION_OPT,
		lnrpc.FeatureBit_PAYMENT_ADDR_REQ,
		lnrpc.FeatureBit_MPP_OPT,
	}
	req.DestFeatures = features

	// Also use the 'tip' key to set the description.
	req.DestCustomRecords[7629171] = []byte(description)
	hashStr := hex.EncodeToString(hash[:])
	if err := a.breezDB.SaveTipMessage(hashStr, []byte(description)); err != nil {
		return "", err
	}

	for k, v := range tlv {
		a.log.Infof("adding custom record %v, value: %v", uint64(k), v)
		req.DestCustomRecords[uint64(k)] = []byte(v)
	}

	if groupKey != "" {
		if err := a.breezDB.SavePaymentGroup(hashStr, []byte(groupKey), []byte(groupName)); err != nil {
			return "", err
		}
	}

	return a.sendPayment(hashStr, nil, req)
}

func (a *Service) GetMaxAmount(destination string, routeHints []*lnrpc.RouteHint, lastHopPubkey []byte) (uint64, error) {
	a.waitReadyForPayment()
	return a.getMaxAmount(destination, routeHints, lastHopPubkey)
}

func (a *Service) getMaxAmount(destination string, routeHints []*lnrpc.RouteHint, lastHopPubkey []byte) (uint64, error) {
	a.log.Infof("destination: %v, routeHints: %#v (len: %v), lastHopPubkey: %x", destination, routeHints, len(routeHints), lastHopPubkey)
	lnclient := a.daemonAPI.APIClient()
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		a.log.Errorf("lnclient.ListChannels error: %v", err)
		return 0, fmt.Errorf("lnclient.ListChannels error: %w", err)
	}
	var totalMax uint64
	for _, c := range channels.Channels {
		if c.LocalBalance == 0 {
			continue
		}
		a.log.Infof("cid: %v, active: %v, balance: %v, channnel reserve: %v", c.ChanId, c.Active, c.LocalBalance, c.LocalConstraints.ChanReserveSat)
		_, err := lnclient.QueryRoutes(context.Background(), &lnrpc.QueryRoutesRequest{
			PubKey:         destination,
			Amt:            c.LocalBalance - int64(c.LocalConstraints.ChanReserveSat) + 1,
			OutgoingChanId: c.ChanId,
			RouteHints:     routeHints,
			LastHopPubkey:  lastHopPubkey,
		})
		if err != nil {
			errStatus, _ := status.FromError(err)
			a.log.Infof("message: %v", errStatus.Message())
			var max uint64
			_, _ = fmt.Sscanf(errStatus.Message(), "insufficient local balance. Try to lower the amount to: %d mSAT", &max)
			a.log.Infof("max: %v", max)
			if max/1000 <= uint64(c.LocalBalance) {
				a.log.Infof("Adding: %v+%v -> %v", totalMax, max, totalMax+max)
				totalMax += max
			} else {
				a.log.Infof("Not adding: %v to totalMax!!!", max)
			}
		}
	}
	return totalMax, nil
}

func (a *Service) checkAmount(payReq *lnrpc.PayReq, sendRequest *routerrpc.SendPaymentRequest) error {
	var amt lnwire.MilliSatoshi
	var destination string
	var routeHints []*lnrpc.RouteHint
	if payReq != nil {
		destination = payReq.Destination
		routeHints = payReq.RouteHints
		sat := payReq.NumSatoshis
		if payReq.NumMsat != 0 {
			sat = 0
		}
		amt, _ = lnrpc.UnmarshallAmt(sat, payReq.NumMsat)
		if amt == 0 {
			amt, _ = lnrpc.UnmarshallAmt(sendRequest.Amt, sendRequest.AmtMsat)
		}
	} else {
		destination = hex.EncodeToString(sendRequest.Dest)
		amt, _ = lnrpc.UnmarshallAmt(sendRequest.Amt, sendRequest.AmtMsat)
	}
	max, err := a.getMaxAmount(destination, routeHints, sendRequest.LastHopPubkey)
	if err != nil {
		a.log.Errorf("a.getMaxAmount error: %v", err)
		return fmt.Errorf("a.getMaxAmount error: %w", err)
	}
	if max == 0 {
		return errors.New(lnrpc.PaymentFailureReason_FAILURE_REASON_NO_ROUTE.String())
	}

	if uint64(amt) > max {
		a.log.Errorf("insufficient balance: %v < %v", max, amt)
		return fmt.Errorf("insufficient balance:%v", max/1000)
	}
	return nil
}

func (a *Service) sendPayment(paymentHash string, payReq *lnrpc.PayReq, sendRequest *routerrpc.SendPaymentRequest) (string, error) {

	lnclient := a.daemonAPI.RouterClient()
	if err := a.waitReadyForPayment(); err != nil {
		a.log.Infof("sendPaymentAsync: error sending payment %v", err)
		return "", err
	}

	if err := a.checkAmount(payReq, sendRequest); err != nil {
		a.log.Infof("sendPaymentAsync: error sending payment %v", err)
		return "", err
	}

	if payReq != nil && len(payReq.RouteHints) == 1 && len(payReq.RouteHints[0].HopHints) == 1 {
		lnclient.XImportMissionControl(context.Background(), &routerrpc.XImportMissionControlRequest{
			Pairs: []*routerrpc.PairHistory{{
				NodeFrom: []byte(payReq.RouteHints[0].HopHints[0].NodeId),
				NodeTo:   []byte(payReq.Destination),
				History: &routerrpc.PairData{
					SuccessTime:    time.Now().UnixNano(),
					SuccessAmtMsat: payReq.NumMsat,
				},
			}}})
	}
	a.log.Infof("sending payment with max fee = %v msat", sendRequest.FeeLimitMsat)
	response, err := lnclient.SendPaymentV2(context.Background(), sendRequest)
	if err != nil {
		a.log.Infof("sendPaymentForRequest: error sending payment %v", err)
		return "", err
	}

	failureReason := lnrpc.PaymentFailureReason_FAILURE_REASON_NONE
	for {
		payment, err := response.Recv()
		if err != nil {
			a.log.Infof("Payment event error received %v", err)
			return "", err
		}
		a.log.Infof("Payment event received %v", payment.Status)
		if payment.Status == lnrpc.Payment_IN_FLIGHT {
			continue
		}
		if payment.Status != lnrpc.Payment_SUCCEEDED {
			failureReason = payment.FailureReason
		}
		break
	}

	if failureReason != lnrpc.PaymentFailureReason_FAILURE_REASON_NONE {
		a.log.Infof("sendPaymentForRequest finished with error, %v", failureReason.String())
		traceReport, err := a.createPaymentTraceReport(sendRequest.PaymentRequest, sendRequest.AmtMsat, failureReason.String())
		if err != nil {
			a.log.Errorf("failed to create trace report for failed payment %v", err)
		}
		errorMsg := failureReason.String()
		if failureReason == lnrpc.PaymentFailureReason_FAILURE_REASON_NO_ROUTE {
			_, maxPay, _, err := a.getReceivePayLimit()
			if err == nil && maxPay-sendRequest.Amt < 50 {
				errorMsg += ". Try sending a smaller amount to keep the required minimum balance."
			}
		}
		return traceReport, errors.New(errorMsg)
	}
	a.log.Infof("sendPaymentForRequest finished successfully")
	a.syncSentPayments()
	// a.notifyPaymentResult(true, sendRequest.PaymentRequest, paymentHash, "", "")
	return "", nil
}

/*
AddInvoice encapsulate a given amount and description in a payment request
*/
func (a *Service) AddInvoice(invoiceRequest *data.AddInvoiceRequest) (paymentRequest string, lspFee int64, err error) {
	lnclient := a.daemonAPI.APIClient()

	// Format the standard invoice memo
	invoice := invoiceRequest.InvoiceDetails
	memo := formatTextMemo(invoice)

	if invoice.Expiry <= 0 {
		invoice.Expiry = defaultInvoiceExpiry
	}

	maxReceive, err := a.getMaxReceiveSingleChannel()
	if err != nil {
		a.log.Infof("failed to get account limits %v", err)
		return "", 0, err
	}

	// in case we don't need a new channel, we make sure the
	// existing channels are active.
	if maxReceive >= invoice.Amount {
		if err := a.waitReadyForPayment(); err != nil {
			return "", 0, err
		}
	}

	lspInfo := invoiceRequest.LspInfo
	if lspInfo == nil {
		return "", 0, errors.New("missing LSP information")
	}

	maxReceiveMsat := maxReceive * 1000
	amountMsat := invoice.Amount * 1000
	smallAmountMsat := amountMsat
	needOpenChannel := maxReceiveMsat < amountMsat
	var routingHints []*lnrpc.RouteHint

	// We need the LSP to open a channel.
	if needOpenChannel {

		fakeHints, err := a.getFakeChannelRoutingHint(lspInfo)
		if err != nil {
			return "", 0, err
		}
		routingHints = []*lnrpc.RouteHint{fakeHints}
		a.log.Infof("Generated zero-conf invoice for amount: %v", amountMsat)

		// Calculate the channel fee such that it's an integral number of sat.
		channelFeesMsat := amountMsat * lspInfo.ChannelFeePermyriad / 10_000 / 1_000 * 1_000
		if channelFeesMsat < lspInfo.ChannelMinimumFeeMsat {
			channelFeesMsat = lspInfo.ChannelMinimumFeeMsat
		}
		a.log.Infof("zero-conf fee calculation: lsp fee rate (permyriad): %v (minimum %v), total fees for channel: %v",
			lspInfo.ChannelFeePermyriad, lspInfo.ChannelMinimumFeeMsat, channelFeesMsat)
		if amountMsat < channelFeesMsat+1000 {
			return "", 0, fmt.Errorf("amount %v should be more than the minimum fees (%v sats)", amountMsat, lspInfo.ChannelMinimumFeeMsat/1000)
		}

		smallAmountMsat = amountMsat - channelFeesMsat
	} else {
		if routingHints, err = a.getLSPRoutingHints(lspInfo); err != nil {
			return "", 0, fmt.Errorf("failed to get LSP routing hints %w", err)
		}
	}

	if len(routingHints) == 0 {
		return "", 0, errors.New("no routing information")
	}

	var payeeInvoice string
	var payeeInvoiceHash []byte
	// check if inner invoice exists
	if invoice.Preimage != nil {
		preImage, err := lntypes.MakePreimage(invoice.Preimage)
		if err != nil {
			return "", 0, fmt.Errorf("failed to create preimage %w", err)
		}

		hash := preImage.Hash()
		payeeInvoiceHash = hash[:]
		existingInvoice, _ := lnclient.LookupInvoice(context.Background(), &lnrpc.PaymentHash{RHash: payeeInvoiceHash})
		if existingInvoice != nil {
			payeeInvoice = existingInvoice.PaymentRequest
			a.log.Infof("found and reusing existing invoice with given hash")
		}
	}

	if payeeInvoice == "" {
		// create invoice with the lower amount.
		response, err := lnclient.AddInvoice(context.Background(), &lnrpc.Invoice{
			RPreimage: invoice.Preimage,
			Memo:      memo, ValueMsat: smallAmountMsat,
			Expiry: invoice.Expiry, RouteHints: routingHints,
		})
		if err != nil {
			return "", 0, err
		}
		payeeInvoice = response.PaymentRequest
		payeeInvoiceHash = response.RHash
		if err := a.breezDB.AddZeroConfHash(payeeInvoiceHash, []byte(response.PaymentRequest)); err != nil {
			return "", 0, fmt.Errorf("failed to add zero-conf invoice %w", err)
		}
		a.log.Infof("Tracking invoice amount=%v, hash=%v", smallAmountMsat, payeeInvoiceHash)
	}

	// create invoice with the larger amount and send to LSP the details.
	if needOpenChannel {
		var paymentAddress []byte
		payeeInvoice, paymentAddress, err = a.generateInvoiceWithNewAmount(payeeInvoice, amountMsat)
		if err != nil {
			return "", 0, fmt.Errorf("failed to generate LSP invoice %w", err)
		}
		a.log.Infof("Generated payee invoice: %v", payeeInvoice)
		lspInfo := invoiceRequest.LspInfo
		pubKey := lspInfo.LspPubkey

		existingZeroInvoice, err := a.breezDB.FetchZeroConfInvoice(payeeInvoiceHash)
		if err != nil {
			return "", 0, fmt.Errorf("failed to fetch zero-conf invoice %w", err)
		}
		if existingZeroInvoice == nil || string(existingZeroInvoice) != payeeInvoice {
			if err := a.registerPayment(payeeInvoiceHash, paymentAddress, amountMsat, smallAmountMsat, pubKey, lspInfo.Id); err != nil {
				return "", 0, fmt.Errorf("failed to register payment with LSP %w", err)
			}
			if err := a.breezDB.AddZeroConfHash(payeeInvoiceHash, []byte(payeeInvoice)); err != nil {
				return "", 0, fmt.Errorf("failed to add zero-conf invoice %w", err)
			}
		}

		a.log.Infof("Zero-conf payment registered: %v", string(payeeInvoiceHash))
	}

	a.log.Infof("Generated Invoice: %v", payeeInvoice)
	return payeeInvoice, (amountMsat - smallAmountMsat) / 1_000, nil
}

func (a *Service) getFakeChannelRoutingHint(lspInfo *data.LSPInformation) (*lnrpc.RouteHint, error) {
	fakeChanID := &lnwire.ShortChannelID{BlockHeight: 1, TxIndex: 0, TxPosition: 0}
	return &lnrpc.RouteHint{
		HopHints: []*lnrpc.HopHint{
			{
				NodeId:                    lspInfo.Pubkey,
				ChanId:                    fakeChanID.ToUint64(),
				FeeBaseMsat:               uint32(lspInfo.BaseFeeMsat),
				FeeProportionalMillionths: uint32(lspInfo.FeeRate * 1000000),
				CltvExpiryDelta:           lspInfo.TimeLockDelta,
			},
		},
	}, nil
}

func (a *Service) getLSPRoutingHints(lspInfo *data.LSPInformation) ([]*lnrpc.RouteHint, error) {

	chanDB, chanDBCleanUp, err := channeldbservice.Get(a.cfg.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("channeldbservice.Get(%v): %w", a.cfg.WorkingDir, err)
	}
	defer chanDBCleanUp()

	openChannels, err := chanDB.ChannelStateDB().FetchAllOpenChannels()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all opened channels %v", err)
	}

	aliasManager, err := aliasmgr.NewManager(chanDB.Backend)
	if err != nil {
		return nil, fmt.Errorf("error in aliasmgr.NewManager: %w", err)
	}

	lnclient := a.daemonAPI.APIClient()
	channelsRes, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{
		PrivateOnly: true,
	})
	if err != nil {
		return nil, err
	}

	var hints []*lnrpc.RouteHint
	usedPeers := make(map[string]struct{})
	for _, h := range channelsRes.Channels {
		if _, ok := usedPeers[h.RemotePubkey]; ok {
			continue
		}

		ci, err := lnclient.GetChanInfo(context.Background(), &lnrpc.ChanInfoRequest{
			ChanId: h.ChanId,
		})
		if err != nil {
			a.log.Errorf("Unable to add routing hint for channel %v error=%v", h.ChanId, err)
			continue
		}

		remotePolicy := ci.Node1Policy
		if h.RemotePubkey == lspInfo.Pubkey && ci.Node2Pub == h.RemotePubkey {
			remotePolicy = ci.Node2Policy
		}

		// skip non lsp channels without remote policy
		if remotePolicy == nil && h.RemotePubkey != lspInfo.Pubkey {
			continue
		}

		feeBaseMsat := uint32(lspInfo.BaseFeeMsat)
		proportionalFee := uint32(lspInfo.FeeRate * 1000000)
		cltvExpiryDelta := lspInfo.TimeLockDelta
		if remotePolicy != nil {
			feeBaseMsat = uint32(remotePolicy.FeeBaseMsat)
			proportionalFee = uint32(remotePolicy.FeeRateMilliMsat)
			cltvExpiryDelta = remotePolicy.TimeLockDelta
		}
		a.log.Infof("adding routing hint = %v", h.RemotePubkey)
		hintID, err := a.getChannelLSPHint(openChannels, aliasManager, h)
		if err != nil {
			return nil, fmt.Errorf("failed to get lsp route hint for channel %v: %v", h.ChanId, err)
		}

		hints = append(hints, &lnrpc.RouteHint{
			HopHints: []*lnrpc.HopHint{
				{
					NodeId:                    h.RemotePubkey,
					ChanId:                    hintID,
					FeeBaseMsat:               feeBaseMsat,
					FeeProportionalMillionths: proportionalFee,
					CltvExpiryDelta:           cltvExpiryDelta,
				},
			},
		})
		usedPeers[h.RemotePubkey] = struct{}{}
	}
	return hints, nil
}

func (a *Service) getChannelLSPHint(openChannels []*channeldb.OpenChannel, aliasManager *aliasmgr.Manager, h *lnrpc.Channel) (uint64, error) {
	hintID := lnwire.NewShortChanIDFromInt(h.ChanId)

	var dbChannel *channeldb.OpenChannel
	for _, c := range openChannels {
		if c.ShortChannelID.ToUint64() == h.ChanId {
			dbChannel = c
			break
		}
	}

	if dbChannel != nil && dbChannel.NegotiatedAliasFeature() {
		var err error
		hintID, err = aliasManager.GetPeerAlias(lnwire.NewChanIDFromOutPoint(&dbChannel.FundingOutpoint))
		if err != nil {
			return 0, fmt.Errorf("error in aliasmgr.GetPeerAlias: %w", err)
		}
	}
	return hintID.ToUint64(), nil
}

// SendPaymentFailureBugReport is used for investigating payment failures.
// It should be used if the user agrees to send his payment details and the response of
// QueryRoutes running in his node. The information is sent to the "bugreporturl" service.
func (a *Service) SendPaymentFailureBugReport(jsonReport string) error {
	client := &http.Client{}
	req, err := http.NewRequest("POST", a.cfg.BugReportURL+"/paymentfailure", bytes.NewBuffer([]byte(jsonReport)))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("PF-Key", a.cfg.BugReportURLSecret)
	_, err = client.Do(req)
	if err != nil {
		a.log.Errorf("Error in sending bug report: ", err)
		return err
	}
	a.log.Infof(jsonReport)
	return nil
}

func (a *Service) createPaymentTraceReport(paymentRequest string, amount int64, errorMsg string) (string, error) {
	lnclient := a.daemonAPI.APIClient()

	var decodedPayReq *lnrpc.PayReq
	var err error
	if paymentRequest != "" {
		decodedPayReq, err = lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})
		if err != nil {
			a.log.Errorf("DecodePaymentRequest error: %v", err)
			return "", err
		}
	}

	lnInfo, err := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		a.log.Errorf("GetInfo error: %v", err)
		return "", err
	}

	netInfo, err := lnclient.GetNetworkInfo(context.Background(), &lnrpc.NetworkInfoRequest{})
	if err != nil {
		a.log.Errorf("GetNetworkInfo error: %v", err)
		return "", err
	}
	marshaller := jsonpb.Marshaler{}
	netInfoData, err := marshaller.MarshalToString(netInfo)
	if err != nil {
		a.log.Errorf("failed to marshal network info: %v", err)
		return "", err
	}

	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		a.log.Errorf("ListChannels error: %v", err)
		return "", err
	}
	chanData, err := marshaller.MarshalToString(channels)
	if err != nil {
		a.log.Errorf("failed to marshal channels info: %v", err)
		return "", err
	}

	if amount == 0 && decodedPayReq != nil {
		amount = decodedPayReq.NumSatoshis
	}

	responseMap := map[string]interface{}{
		"request_details": map[string]interface{}{
			"source_node":     lnInfo.IdentityPubkey,
			"amount":          amount,
			"payment_request": decodedPayReq,
			"network_info":    netInfoData,
			"channels":        chanData,
		},
	}

	responseMap["payment_error"] = errorMsg

	response, err := json.MarshalIndent(responseMap, "", "  ")
	if err != nil {
		fmt.Println("unable to marshal response to json: ", err)
		return "", err
	}

	return string(response), nil
}

/*
DecodePaymentRequest is used by the payer to decode the payment request and read the invoice details.
*/
func (a *Service) DecodePaymentRequest(paymentRequest string) (*data.InvoiceMemo, error) {
	a.log.Infof("DecodePaymentRequest %v", paymentRequest)
	lnclient := a.daemonAPI.APIClient()
	decodedPayReq, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		a.log.Errorf("DecodePaymentRequest error: %v", err)
		return nil, err
	}
	invoiceMemo := a.extractMemo(decodedPayReq)
	return invoiceMemo, nil
}

func (a *Service) GetPaymentRequestHash(paymentRequest string) (string, error) {
	a.log.Infof("GetPaymentRequestHash %v", paymentRequest)
	lnclient := a.daemonAPI.APIClient()
	if lnclient == nil {
		return "", errors.New("daemon is not ready")
	}
	decodedPayReq, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		a.log.Errorf("DecodePaymentRequest error: %v", err)
		return "", err
	}
	return decodedPayReq.PaymentHash, nil
}

/*
GetRelatedInvoice is used by the payee to fetch the related invoice of its sent payment request so he can see if it is settled.
*/
func (a *Service) GetRelatedInvoice(paymentRequest string) (*data.Invoice, error) {
	lnclient := a.daemonAPI.APIClient()
	decodedPayReq, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})

	if err != nil {
		a.log.Infof("Can't decode payment request: %v", paymentRequest)
		return nil, err
	}
	a.log.Infof("GetRelatedInvoice: %s", decodedPayReq.PaymentHash)

	invoiceMemo := a.extractMemo(decodedPayReq)

	lookup, err := lnclient.LookupInvoice(context.Background(), &lnrpc.PaymentHash{RHashStr: decodedPayReq.PaymentHash})
	if err != nil {
		a.log.Infof("GetRelatedInvoice: LookupInvoice failed.")
		return nil, err
	}

	invoice := &data.Invoice{
		Memo:    invoiceMemo,
		AmtPaid: lookup.AmtPaidSat,
		Settled: lookup.Settled,
	}

	return invoice, nil
}

func (a *Service) extractMemo(decodedPayReq *lnrpc.PayReq) *data.InvoiceMemo {
	invoiceMemo := &data.InvoiceMemo{}
	invoiceMemo.Amount = decodedPayReq.NumSatoshis
	parseTextMemo(decodedPayReq.Description, invoiceMemo)
	if invoiceMemo.Description == transferFundsRequest {
		invoiceMemo.TransferRequest = true
	}
	return invoiceMemo
}

func formatTextMemo(invoice *data.InvoiceMemo) string {
	memo := invoice.Description
	formatPayeeData := invoice.PayeeName != "" && invoice.PayeeImageURL != ""
	if formatPayeeData {
		memo += invoiceCustomPartDelimiter
		customParts := []string{invoice.PayeeName, invoice.PayeeImageURL}
		formatPayerData := invoice.PayerName != "" && invoice.PayerImageURL != ""
		if formatPayerData {
			customParts = append(customParts, invoice.PayerName, invoice.PayerImageURL)
		}
		memo += strings.Join(customParts, " | ")
	}
	return memo
}

func parseTextMemo(memo string, invoiceMemo *data.InvoiceMemo) {
	invoiceMemo.Description = memo
	invoiceParts := strings.Split(memo, invoiceCustomPartDelimiter)
	if len(invoiceParts) == 2 {
		invoiceMemo.Description = invoiceParts[0]
		customData := invoiceParts[1]
		customDataParts := strings.Split(customData, " | ")
		invoiceMemo.PayeeName = customDataParts[0]
		invoiceMemo.PayeeImageURL = customDataParts[1]
		if len(customDataParts) == 4 {
			invoiceMemo.PayerName = customDataParts[2]
			invoiceMemo.PayerImageURL = customDataParts[3]
		}
	}
}

func (a *Service) watchPayments() {
	defer a.wg.Done()
	lnclient := a.daemonAPI.APIClient()
	if err := a.syncSentPayments(); err != nil {
		a.log.Errorf("failed to sync payments: %v", err)
	}
	go func() {
		retry := 0
		for retry < 3 {
			if err := a.syncClosedChannels(); err != nil {
				a.log.Errorf("failed to sync closed chanels retry:%v error: %v", retry, err)
				time.Sleep(4 * time.Second)
				retry++
				continue
			}
			return
		}
	}()

	_, lastInvoiceSettledIndex := a.breezDB.FetchPaymentsSyncInfo()
	a.log.Infof("last invoice settled index %v", lastInvoiceSettledIndex)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := lnclient.SubscribeInvoices(ctx, &lnrpc.InvoiceSubscription{SettleIndex: lastInvoiceSettledIndex})
	if err != nil {
		a.log.Criticalf("Failed to call SubscribeInvoices %v, %v", stream, err)
	}

	go func() {
		for {
			invoice, err := stream.Recv()
			a.log.Infof("watchPayments - Invoice received by subscription")
			if err != nil {
				a.log.Criticalf("Failed to receive an invoice : %v", err)
				return
			}
			if invoice.Settled {
				a.log.Infof("watchPayments adding a received payment")
				if err = a.onNewReceivedPayment(invoice); err != nil {
					a.log.Criticalf("Failed to update received payment : %v", err)
					return
				}
			}
		}
	}()

	go func() {
		<-a.quitChan
		a.log.Infof("Canceling subscription")
		cancel()
	}()
}

func (a *Service) syncSentPayments() error {
	a.log.Infof("syncSentPayments")
	lnclient := a.daemonAPI.APIClient()
	lightningPayments, err := lnclient.ListPayments(context.Background(), &lnrpc.ListPaymentsRequest{})
	if err != nil {
		return err
	}
	lastPaymentTime, _ := a.breezDB.FetchPaymentsSyncInfo()
	for _, paymentItem := range lightningPayments.Payments {
		// We go back up to 36 hours to make sure we sync payments that were pending
		// for a long time before we update the sync time
		if paymentItem.CreationDate <= lastPaymentTime-60*60*36 {
			continue
		}
		a.log.Infof("syncSentPayments adding an outgoing payment")
		a.onNewSentPayment(paymentItem)
	}

	return nil

	//TODO delete history of payment requests after the new payments API stablized.
}

func (a *Service) getPendingPayments(includeInflight bool) ([]*db.PaymentInfo, error) {
	var payments []*db.PaymentInfo
	lnclient := a.daemonAPI.APIClient()
	if a.daemonRPCReady() {
		channelsRes, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
		if err != nil {
			return nil, err
		}
		chainInfo, chainErr := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
		if chainErr != nil {
			a.log.Errorf("Failed get chain info", chainErr)
			return nil, chainErr
		}

		inflightPayments, err := a.getInflightPaymentsMap()
		if err != nil {
			return nil, err
		}
		pendingByHash := make(map[string]*db.PaymentInfo)
		for _, ch := range channelsRes.Channels {
			for _, htlc := range ch.PendingHtlcs {
				pendingItem, err := a.createPendingPayment(htlc, chainInfo.BlockHeight, inflightPayments)
				if err != nil {
					return nil, err
				}

				pendingSoFar, ok := pendingByHash[pendingItem.PaymentHash]
				currentInflight, hasInflight := inflightPayments[pendingItem.PaymentHash]
				if !ok {
					pendingByHash[pendingItem.PaymentHash] = pendingItem
					pendingSoFar = pendingItem

					// we have an in flight payment, let's get all the htlc and sum them.
					if hasInflight {
						pendingSoFar.Fee = 0
						pendingSoFar.Amount = 0
						for _, ht := range currentInflight.Htlcs {
							if ht.Status != lnrpc.HTLCAttempt_FAILED {
								a.log.Infof("pendingPaymets: adding fee %v and amount %v", ht.Route.TotalFees, ht.Route.TotalAmt)
								pendingSoFar.Fee += ht.Route.TotalFees
								pendingSoFar.Amount += (ht.Route.TotalAmt - ht.Route.TotalFees)
							}
						}
						a.log.Infof("pendingPaymets: hasInflight=%v, pendingSoFar.Amount=%v, currentInflight.ValueSat=%v",
							hasInflight, pendingSoFar.Amount, currentInflight.ValueSat)
						if pendingSoFar.Amount == currentInflight.ValueSat {
							pendingSoFar.PendingFull = true
						}
						pendingSoFar.Amount = currentInflight.ValueSat
					}
				}
			}
		}

		if includeInflight {
			// add pending payments that represents in flight outgoing payments
			// without any htlcs.
			for hash, inFlight := range inflightPayments {
				if _, ok := pendingByHash[hash]; !ok {
					hashBytes, err := hex.DecodeString(inFlight.PaymentHash)
					if err != nil {
						return nil, err
					}
					if time.Now().Sub(time.Unix(inFlight.CreationDate, 0)) < time.Second*60 {
						payment, err := a.createPendingPayment(&lnrpc.HTLC{
							HashLock:         hashBytes,
							Incoming:         false,
							ExpirationHeight: chainInfo.BlockHeight + 144,
							Amount:           inFlight.ValueSat},
							chainInfo.BlockHeight, inflightPayments)
						if err != nil {
							return nil, err
						}
						pendingByHash[inFlight.PaymentHash] = payment
					}
				}
			}
		}

		for h, p := range pendingByHash {
			groupKey, groupName, err := a.breezDB.FetchPaymentGroup(h)
			if err != nil {
				return nil, err
			}
			if groupKey != nil {
				p.GroupKey = string(groupKey)
			}
			if groupName != nil {
				p.GroupName = string(groupName)
			}
			payments = append(payments, p)
		}
	}

	return payments, nil
}

func (a *Service) getInflightPaymentsMap() (map[string]*lnrpc.Payment, error) {
	lnclient := a.daemonAPI.APIClient()
	lightningPayments, err := lnclient.ListPayments(context.Background(),
		&lnrpc.ListPaymentsRequest{IncludeIncomplete: true})
	if err != nil {
		return nil, err
	}

	payments := make(map[string]*lnrpc.Payment)
	for _, pending := range lightningPayments.Payments {
		if pending.Status == lnrpc.Payment_IN_FLIGHT {
			a.log.Infof("found inflight payment %v", pending.PaymentHash)
			//mar, _ := json.Marshal(pending)
			//a.log.Infof(string(mar))
			payments[pending.PaymentHash] = pending
		}
	}
	return payments, nil
}

func (a *Service) createPendingPayment(htlc *lnrpc.HTLC, currentBlockHeight uint32,
	inflightPayments map[string]*lnrpc.Payment) (*db.PaymentInfo, error) {

	paymentType := db.SentPayment
	if htlc.Incoming {
		paymentType = db.ReceivedPayment
	}

	var paymentRequest string
	var pendingPayment *lnrpc.Payment
	lnclient := a.daemonAPI.APIClient()

	a.log.Infof("createPendingPayment")
	amount := htlc.Amount
	if htlc.Incoming {
		invoice, err := lnclient.LookupInvoice(context.Background(), &lnrpc.PaymentHash{RHash: htlc.HashLock})
		if err != nil {
			a.log.Errorf("createPendingPayment - failed to call LookupInvoice %v", err)
			return nil, err
		}
		if invoice != nil {
			paymentRequest = invoice.PaymentRequest
		}
	} else {
		payReqBytes, err := a.breezDB.FetchPaymentRequest(hex.EncodeToString(htlc.HashLock))
		if err != nil {
			a.log.Errorf("createPendingPayment - failed to call fetchPaymentRequest %v", err)
			return nil, err
		}
		paymentRequest = string(payReqBytes)
		if err != nil {
			return nil, err
		}
		hashStr := hex.EncodeToString(htlc.HashLock)
		pendingPayment, _ = inflightPayments[hashStr]
	}

	minutesToExpire := time.Duration((htlc.ExpirationHeight - currentBlockHeight) * 10)
	paymentData := &db.PaymentInfo{
		Type:                       paymentType,
		Amount:                     amount,
		CreationTimestamp:          time.Now().Unix(),
		PendingExpirationHeight:    htlc.ExpirationHeight,
		PendingExpirationTimestamp: time.Now().Add(minutesToExpire * time.Minute).Unix(),
	}

	if paymentRequest != "" {
		decodedReq, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: paymentRequest})
		if err != nil {
			return nil, err
		}

		invoiceMemo, err := a.DecodePaymentRequest(paymentRequest)
		if err != nil {
			return nil, err
		}

		paymentData.Description = invoiceMemo.Description
		paymentData.PayeeImageURL = invoiceMemo.PayeeImageURL
		paymentData.PayeeName = invoiceMemo.PayeeName
		paymentData.PayerImageURL = invoiceMemo.PayerImageURL
		paymentData.PayerName = invoiceMemo.PayerName
		paymentData.TransferRequest = invoiceMemo.TransferRequest
		paymentData.PaymentHash = decodedReq.PaymentHash
		paymentData.Destination = decodedReq.Destination
	}

	if pendingPayment != nil {
		paymentData.IsKeySend = len(pendingPayment.PaymentRequest) == 0
		paymentData.PaymentHash = pendingPayment.PaymentHash
		paymentData.CreationTimestamp = pendingPayment.CreationTimeNs / int64(time.Second)
	}

	return paymentData, nil
}

func (a *Service) onNewSentPayment(paymentItem *lnrpc.Payment) error {

	paymentData := &db.PaymentInfo{
		Type:              db.SentPayment,
		Amount:            paymentItem.Value,
		Fee:               paymentItem.Fee,
		CreationTimestamp: paymentItem.CreationDate,
		PaymentHash:       paymentItem.PaymentHash,
		Preimage:          paymentItem.PaymentPreimage,
	}

	if len(paymentItem.PaymentRequest) > 0 {
		invoiceMemo, err := a.DecodePaymentRequest(paymentItem.PaymentRequest)
		if err != nil {
			return err
		}

		paymentData.Description = invoiceMemo.Description

		/* If invoiceMemo.Description is empty check if
		   there's an LNUrlPayInfo with this paymentHash and
		   store that description. Why? Because in LNUrl-Pay,
		   an invoice will only have a descriptionHash in the `h` tag.
		   The client receives the invoice description in a separate request.
		   We save the LNUrlPayInfo as soon as we receive it so it is ok to check the db for it here.
		*/
		if info, err := a.breezDB.FetchLNUrlPayInfo(paymentItem.PaymentHash); err == nil && info != nil {
			if paymentData.Description == "" {
				paymentData.Description = info.InvoiceDescription
				a.log.Infof("onNewSentPayment: No description found in this invoice. Using :%q", paymentData.Description)
			}

			if info.SuccessAction != nil && info.SuccessAction.Tag == "aes" {
				if info.SuccessAction.Message, err = a.DecryptLNUrlPayMessage(paymentItem.PaymentHash, invoiceMemo.Preimage); err != nil {
					a.log.Errorf("onNewSentPayment: Could not decrypt 'aes' lnurl-pay message: %s", err)
				}

			}
		}

		paymentData.PayeeImageURL = invoiceMemo.PayeeImageURL
		paymentData.PayeeName = invoiceMemo.PayeeName
		paymentData.PayerImageURL = invoiceMemo.PayerImageURL
		paymentData.PayerName = invoiceMemo.PayerName
		paymentData.TransferRequest = invoiceMemo.TransferRequest

		lnclient := a.daemonAPI.APIClient()
		decodedReq, err := lnclient.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: string(paymentItem.PaymentRequest)})
		if err != nil {
			return err
		}
		paymentData.Destination = decodedReq.Destination
		if decodedReq.Destination == a.cfg.SwapperPubkey {
			paymentData.Type = db.WithdrawalPayment
		}
	} else {
		paymentData.IsKeySend = true
		message, err := a.breezDB.FetchTipMessage(paymentItem.PaymentHash)
		if err != nil {
			return err
		}
		groupKey, groupName, err := a.breezDB.FetchPaymentGroup(paymentItem.PaymentHash)
		if err != nil {
			return err
		}

		numHtlcs := len(paymentItem.Htlcs)
		if numHtlcs > 0 && paymentItem.Htlcs[0].Route != nil {
			hops := paymentItem.Htlcs[0].Route.Hops
			if len(hops) > 0 {
				lastHop := hops[len(hops)-1]
				paymentData.Destination = lastHop.PubKey
			}
		}
		if groupKey != nil {
			paymentData.GroupKey = string(groupKey)
		}
		if groupName != nil {
			paymentData.GroupName = string(groupName)
		}
		// pathLength := len(paymentItem.Path)
		// if pathLength > 0 {
		// 	paymentData.Destination = paymentItem.Path[pathLength-1]
		// }
		paymentData.Description = string(message)
	}

	swap, err := a.breezDB.FetchReverseSwap(paymentItem.PaymentHash)
	if err != nil {
		return err
	}
	if swap != nil {
		paymentData.Type = db.WithdrawalPayment
		paymentData.RedeemTxID = swap.ClaimTxid
		paymentData.Amount = swap.OnchainAmount - swap.ClaimFee
		paymentData.Fee += paymentItem.Value - swap.OnchainAmount + swap.ClaimFee
	}

	skipped, err := a.breezDB.AddAccountPayment(paymentData, 0, uint64(paymentItem.CreationDate))
	if !skipped {
		a.onServiceEvent(data.NotificationEvent{Type: data.NotificationEvent_PAYMENT_SENT})
		a.onAccountChanged()
	}
	return err
}

func (a *Service) onNewReceivedPayment(invoice *lnrpc.Invoice) error {
	var invoiceMemo *data.InvoiceMemo
	var err error
	if len(invoice.PaymentRequest) > 0 {
		if invoiceMemo, err = a.DecodePaymentRequest(invoice.PaymentRequest); err != nil {
			return err
		}
	}

	zeroConfPayreq, err := a.breezDB.FetchZeroConfInvoice(invoice.RHash)
	if err != nil {
		return err
	}
	a.log.Infof("got payment zero-conf payreq = %v", string(zeroConfPayreq))
	var zeroConfMemo *data.InvoiceMemo
	if len(zeroConfPayreq) > 0 {
		if zeroConfMemo, err = a.DecodePaymentRequest(string(zeroConfPayreq)); err != nil {
			return err
		}
		a.log.Infof("got payment decoded zero-conf memo amount: %v", zeroConfMemo.Amount)
	}

	paymentType := db.ReceivedPayment
	if invoiceMemo.TransferRequest {
		paymentType = db.DepositPayment
	}

	paymentData := &db.PaymentInfo{
		Type:              paymentType,
		Amount:            invoice.AmtPaidSat,
		CreationTimestamp: invoice.SettleDate,
		Description:       invoiceMemo.Description,
		PayeeImageURL:     invoiceMemo.PayeeImageURL,
		PayeeName:         invoiceMemo.PayeeName,
		PayerImageURL:     invoiceMemo.PayerImageURL,
		PayerName:         invoiceMemo.PayerName,
		TransferRequest:   invoiceMemo.TransferRequest,
		PaymentHash:       hex.EncodeToString(invoice.RHash),
		Preimage:          hex.EncodeToString(invoice.RPreimage),
	}
	if zeroConfMemo != nil {
		paymentData.Fee = zeroConfMemo.Amount - invoiceMemo.Amount
		a.log.Infof("got payment calculated fee: %v", paymentData.Fee)
		if err = a.breezDB.RemoveZeroConfHash(invoice.RHash); err != nil {
			return err
		}
	}

	_, err = a.breezDB.AddAccountPayment(paymentData, invoice.SettleIndex, 0)
	if err != nil {
		a.log.Errorf("Unable to add reveived payment : %v", err)
		return err
	}
	a.onServiceEvent(data.NotificationEvent{
		Type: data.NotificationEvent_INVOICE_PAID,
		Data: []string{invoice.PaymentRequest}})
	a.onAccountChanged()
	return nil
}

func (a *Service) registerPayment(paymentHash, paymentSecret []byte, incomingAmountMsat,
	outgoingAmountMsat int64, lspPubkey []byte, lspID string) error {

	destination, err := hex.DecodeString(a.daemonAPI.NodePubkey())
	if err != nil {
		a.log.Infof("hex.DecodeString(%v) error: %v", a.daemonAPI.NodePubkey(), err)
		return fmt.Errorf("hex.DecodeString(%v) error: %w", a.daemonAPI.NodePubkey(), err)
	}
	tag, err := a.generateTag()
	if err != nil {
		a.log.Infof("generateTag() error: %v", err)
		return fmt.Errorf("generateTag() error: %w", err)
	}

	pi := &lspd.PaymentInformation{
		PaymentHash:        paymentHash,
		PaymentSecret:      paymentSecret,
		Destination:        destination,
		IncomingAmountMsat: incomingAmountMsat,
		OutgoingAmountMsat: outgoingAmountMsat,
		Tag:                tag,
	}
	data, err := proto.Marshal(pi)

	c, ctx, cancel := a.breezAPI.NewChannelOpenerClient()
	defer cancel()

	a.log.Infof("Register Payment pubkey = %v", lspPubkey)
	pubkey, err := btcec.ParsePubKey(lspPubkey)
	if err != nil {
		a.log.Infof("btcec.ParsePubKey(%x) error: %v", lspPubkey, err)
		return fmt.Errorf("btcec.ParsePubKey(%x) error: %w", lspPubkey, err)
	}
	blob, err := btceclegacy.Encrypt(pubkey, data)
	if err != nil {
		a.log.Infof("btcec.Encrypt(%x) error: %v", data, err)
		return fmt.Errorf("btcec.Encrypt(%x) error: %w", data, err)
	}

	_, err = c.RegisterPayment(ctx, &breezservice.RegisterPaymentRequest{LspId: lspID, Blob: blob})
	if err != nil {
		a.log.Infof("RegisterPayment() error: %v", err)
		return fmt.Errorf("RegisterPayment() error: %w", err)
	}
	return nil
}

func (a *Service) generateTag() (string, error) {
	h := sha256.Sum256([]byte(a.cfg.LspToken))
	k := hex.EncodeToString(h[:])
	obj := map[string]interface{}{
		"apiKeyHash": k,
	}

	tag, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(tag), nil
}
