package libwallet

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"decred.org/dcrwallet/v4/errors"
	api "github.com/crypto-power/instantswap/instantswap"

	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/crypto-power/instantswap/blockexplorer"
	_ "github.com/crypto-power/instantswap/blockexplorer/blockcypher" //nolint:revive
	_ "github.com/crypto-power/instantswap/blockexplorer/btcexplorer" //nolint:revive
	_ "github.com/crypto-power/instantswap/blockexplorer/dcrexplorer"
)

const (
	// BTCBlockTime is the average time it takes to mine a block on the BTC
	// network.
	BTCBlockTime = 10 * time.Minute
	// DCRBlockTime is the average time it takes to mine a block on the DCR
	// network.
	DCRBlockTime = 5 * time.Minute
	// LTCBlockTime is approx. how long it takes to mine a block on the Litecoin
	// network.
	LTCBlockTime = 3 * time.Minute

	// DefaultMarketDeviation is the maximum deviation the server rate
	// can deviate from the market rate.
	DefaultMarketDeviation = 5 // 5%
	// DefaultConfirmations is the number of confirmations required
	DefaultConfirmations = 1
	// DefaultRateRequestAmount is the amount used to perform the rate request query.
	DefaultRateRequestAmount = 1
	DefaultRateRequestBTC    = 0.01
	DefaultRateRequestLTC    = 1
	DefaultRateRequestDCR    = 10
)

func DefaultRateRequestAmt(fromCurrency string) float64 {
	switch fromCurrency {
	case utils.BTCWalletAsset.String():
		return DefaultRateRequestBTC
	case utils.LTCWalletAsset.String():
		return DefaultRateRequestLTC
	case utils.DCRWalletAsset.String():
		return DefaultRateRequestDCR
	}
	return DefaultRateRequestAmount
}

// StartScheduler starts the automatic order scheduler.
func (mgr *AssetsManager) StartScheduler(ctx context.Context, params instantswap.SchedulerParams) error {
	const op errors.Op = "mgr.StartScheduler"
	log.Info("Order Scheduler: started")

	log.Info("Order Scheduler: verifying source wallet")
	sourceWallet := mgr.WalletWithID(params.Order.SourceWalletID)
	if sourceWallet == nil {
		return errors.E(op, errors.Errorf("wallet with id:%d not found", params.Order.SourceWalletID))
	}

	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()

	if mgr.InstantSwap.CancelOrderScheduler != nil {
		mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
		return errors.New("scheduler already running")
	}

	mgr.InstantSwap.SchedulerStartTime = time.Now()
	mgr.InstantSwap.SchedulerCtx, mgr.InstantSwap.CancelOrderScheduler = context.WithCancel(ctx)
	mgr.InstantSwap.PublishOrderSchedulerStarted()
	defer func() {
		mgr.InstantSwap.CancelOrderScheduler = nil
		mgr.InstantSwap.SchedulerStartTime = time.Time{}
		mgr.InstantSwap.PublishOrderSchedulerEnded()
		log.Info("Order Scheduler: exited")
	}()
	mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()

	// Initialize the exchange server.
	log.Info("Order Scheduler: initializing exchange server")
	exchangeObject, err := mgr.InstantSwap.NewExchangeServer(params.Order.ExchangeServer)
	if err != nil {
		return errors.E(op, err)
	}

	var lastOrderTime time.Time
	for {
		// Check if scheduler has been shutdown and exit if true.
		if mgr.InstantSwap.SchedulerCtx.Err() != nil {
			return mgr.InstantSwap.SchedulerCtx.Err()
		}

		sourceAccountBalance, err := sourceWallet.GetAccountBalance(params.Order.SourceAccountNumber)
		if err != nil {
			log.Error("unable to get account balance")
			return err
		}

		walletBalance := sourceAccountBalance.Spendable.ToCoin()
		if walletBalance <= params.BalanceToMaintain {
			if !lastOrderTime.IsZero() { // some orders have already been concluded
				return nil
			}

			log.Error("source wallet balance is less than or equals the set balance to maintain")
			return errors.E(op, "source wallet balance is less than or equals the set balance to maintain") // stop scheduling if the source wallet balance is less than or equals the set balance to maintain
		}

		if !lastOrderTime.IsZero() {
			log.Info("Order Scheduler: creating next order based on selected frequency")

			// calculate time until the next order
			timeUntilNextOrder := params.Frequency - time.Since(lastOrderTime)
			if timeUntilNextOrder <= 0 {
				log.Info("Order Scheduler: the scheduler start time is equal to or greater than the frequency, starting next order immediately")
			} else {
				log.Infof("Order Scheduler: %s until the next order is executed", timeUntilNextOrder)
				time.Sleep(timeUntilNextOrder)
			}
		}

		fromCur := params.Order.FromCurrency
		toCur := params.Order.ToCurrency
		rateRequestParams := api.ExchangeRateRequest{
			From:        fromCur,
			To:          toCur,
			Amount:      DefaultRateRequestAmt(fromCur), // amount needs to be greater than 0 to get the exchange rate
			FromNetwork: params.Order.FromNetwork,
			ToNetwork:   params.Order.ToNetwork,
		}
		log.Info("Order Scheduler: getting exchange rate info")
		res, err := mgr.InstantSwap.GetExchangeRateInfo(exchangeObject, rateRequestParams)
		if err != nil {
			log.Error("unable to get exchange server rate info")
			return errors.E(op, err)
		}

		if params.MaxDeviationRate <= 0 {
			params.MaxDeviationRate = DefaultMarketDeviation // default 5%
		}

		market := values.NewMarket(fromCur, toCur)
		source := mgr.RateSource.Name()
		ticker := mgr.RateSource.GetTicker(market, false)
		if ticker == nil {
			log.Errorf("unable to get market(%s) rate from %s.", market, source)
			log.Infof("Proceeding without checking market rate deviation...")
		} else {
			exchangeServerRate := res.ExchangeRate // estimated receivable value for libwallet.DefaultRateRequestAmount (1)
			rateSourceRate := ticker.LastTradePrice

			// Current rate source supported Binance and Bittrex always returns
			// ticker.LastTradePrice in's the quote asset unit e.g DCR-BTC, LTC-BTC.
			// We will also do this when and if USDT is supported.
			if strings.EqualFold(fromCur, "btc") {
				rateSourceRate = 1 / ticker.LastTradePrice
			}

			serverRateStr := values.StringF(values.StrServerRate, params.Order.ExchangeServer.Server, fromCur, exchangeServerRate, toCur)
			log.Info(serverRateStr)
			binanceRateStr := values.StringF(values.StrCurrencyConverterRate, source, fromCur, rateSourceRate, toCur)
			log.Info(binanceRateStr)

			// Check if the server rate deviates from the market rate by ± 5%
			// exit if true
			percentageDiff := math.Abs((exchangeServerRate-rateSourceRate)/((exchangeServerRate+rateSourceRate)/2)) * 100
			if percentageDiff > params.MaxDeviationRate {
				errMsg := fmt.Errorf("exchange rate deviates from the market rate by (%.2f%%) more than %.2f%%", percentageDiff-params.MaxDeviationRate, params.MaxDeviationRate)
				log.Error(errMsg)
				return errors.E(op, errMsg)
			}
		}

		// set the max send amount to the max limit set by the server
		invoicedAmount := res.Max

		estimatedBalanceAfterExchange := walletBalance - invoicedAmount
		// if the max send limit is 0, then the server does not have a max limit
		// constraint so we can send the entire source wallet balance
		if res.Max == 0 || estimatedBalanceAfterExchange < params.BalanceToMaintain {
			invoicedAmount = walletBalance - params.BalanceToMaintain // deduct the balance to maintain from the source wallet balance
		}

		if invoicedAmount <= 0 {
			errMsg := fmt.Errorf("balance to maintain is the same or greater than wallet balance(Current Balance: %v, Balance to Maintain: %v)", walletBalance, params.BalanceToMaintain)
			log.Error(errMsg)
			return errors.E(op, errMsg)
		}

		if invoicedAmount == walletBalance {
			errMsg := "Specify a little balance to maintain to cover for transaction fees... e.g 0.001 for DCR to BTC or LTC swaps"
			log.Error(errMsg)
			return errors.E(op, errMsg)
		}

		log.Info("Order Scheduler: creating order")
		params.Order.InvoicedAmount = invoicedAmount
		order, err := mgr.InstantSwap.CreateOrder(exchangeObject, params.Order)
		if err != nil {
			log.Error("error creating order: ", err.Error())
			return errors.E(op, err)
		}
		lastOrderTime = time.Now()

		log.Info("Order Scheduler: creating unsigned transaction")

		// construct the transaction to send the invoiced amount to the exchange server
		err = sourceWallet.NewUnsignedTx(params.Order.SourceAccountNumber, nil)
		if err != nil {
			return errors.E(op, err)
		}

		var amount int64
		switch sourceWallet.GetAssetType() {
		case utils.BTCWalletAsset:
			amount = btc.AmountSatoshi(params.Order.InvoicedAmount)
		case utils.DCRWalletAsset:
			amount = dcr.AmountAtom(params.Order.InvoicedAmount)
		}

		log.Infof("Order Scheduler: adding send destination, address: %s, amount: %.2f", order.DepositAddress, params.Order.InvoicedAmount)
		// TODO: Broadcast will fail below if params.Order.InvoicedAmount is the
		// same as the current wallet balance. We should be able to consider
		// wallet fees for the transaction whilst constructing the transaction.
		// As a temporary band aid, a check has been added above to error if
		// swap amount does not consider tx fees.
		err = sourceWallet.AddSendDestination(0, order.DepositAddress, amount, false)
		if err != nil {
			log.Error("error adding send destination: ", err.Error())
			return errors.E(op, err)
		}

		log.Info("Order Scheduler: broadcasting tx")
		txHash, err := sourceWallet.Broadcast(params.SpendingPassphrase, "")
		if err != nil {
			log.Error("error broadcasting tx: ", err.Error())
			return errors.E(op, err)
		}

		// wait for the order to be completed before scheduling the next order
		var isRefunded bool
		for {
			// Check if scheduler has been shutdown and exit if true.
			if mgr.InstantSwap.SchedulerCtx.Err() != nil {
				return mgr.InstantSwap.SchedulerCtx.Err()
			}

			// depending on the block time for the asset, the order may take a while to complete
			// so we wait for the estimated block time before checking the order status
			switch params.Order.ToCurrency {
			case utils.BTCWalletAsset.String():
				log.Info("Order Scheduler: waiting for btc block time (10 minutes)")
				time.Sleep(BTCBlockTime)
			case utils.DCRWalletAsset.String():
				log.Info("Order Scheduler: waiting for dcr block time (5 minutes)")
				time.Sleep(DCRBlockTime)
			case utils.LTCWalletAsset.String():
				log.Info("Order Scheduler: waiting for ltc block time (~3 minutes)")
				time.Sleep(LTCBlockTime)
			}

			log.Info("Order Scheduler: get newly created order info")
			orderInfo, err := mgr.InstantSwap.GetOrderInfo(exchangeObject, order.UUID)
			if err != nil {
				return errors.E(op, err)
			}

			// If this is empty for any reason, default to the actual tx hash.
			if orderInfo.TxID == "" {
				orderInfo.TxID = txHash
			}

			if orderInfo.Status == api.OrderStatusRefunded {
				log.Info("order was refunded. verifying that the order was refunded successfully from the blockchain explorer")
				tmpFromCurrency := params.Order.FromCurrency
				tmpToCurrency := params.Order.ToCurrency

				// swap ToCurrency and FromCurrency
				params.Order.ToCurrency = tmpFromCurrency
				params.Order.FromCurrency = tmpToCurrency

				orderInfo.ReceiveAmount = params.Order.InvoicedAmount

				isRefunded = true
			}

			log.Info("Order Scheduler: instantiate block explorer")
			// verify that the order was completed successfully from the blockchain explorer
			config := blockexplorer.Config{
				EnableOutput: false,
				Symbol:       params.Order.ToCurrency,
			}
			explorer, err := blockexplorer.NewExplorer(config) // TODO: Confirm if this still works as intended
			if err != nil {
				log.Error("error instantiating block explorer: ", err.Error())
				return errors.E(op, err)
			}

			verificationInfo := blockexplorer.TxVerifyRequest{
				TxId:      orderInfo.TxID,
				Amount:    orderInfo.ReceiveAmount,
				CreatedAt: orderInfo.CreatedAt,
				Address:   orderInfo.DestinationAddress,
				Confirms:  DefaultConfirmations,
			}

			log.Infof("Order Scheduler: verifying transaction with ID: %s", orderInfo.TxID)
			verification, err := explorer.VerifyTransaction(verificationInfo)
			if err != nil {
				log.Error("error verifying transaction: ", err.Error())
				return errors.E(op, err)
			}

			if verification.Verified {
				if verification.BlockExplorerAmount.ToCoin() != orderInfo.ReceiveAmount {
					log.Infof("received amount: %f", verification.BlockExplorerAmount.ToCoin())
					log.Infof("expected amount: %f", orderInfo.ReceiveAmount)
					log.Error("received amount does not match the expected amount")
					return errors.E(op, err)
				}

				if isRefunded {
					log.Info("order was refunded successfully")
				} else {
					log.Info("order was completed successfully")
				}

				break // order is completed, break out of the loop
			}

			continue // order is not completed, check again
		}
	}
}

// StopScheduler stops the order scheduler.
func (mgr *AssetsManager) StopScheduler() {
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()
	if mgr.InstantSwap.CancelOrderScheduler != nil {
		mgr.InstantSwap.CancelOrderScheduler()
		mgr.InstantSwap.CancelOrderScheduler = nil
	}
	mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	log.Info("Order Scheduler: stopped")
}

// IsOrderSchedulerRunning returns true if the order scheduler is running.
func (mgr *AssetsManager) IsOrderSchedulerRunning() bool {
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()
	defer mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	return mgr.InstantSwap.CancelOrderScheduler != nil
}

// GetSchedulerRuntime returns the duration the order scheduler has been
// running.
func (mgr *AssetsManager) GetSchedulerRuntime() string {
	return time.Since(mgr.InstantSwap.SchedulerStartTime).Round(time.Second).String()
}
