package libwallet

import (
	"context"
	"math"
	"time"

	"decred.org/dcrwallet/v3/errors"
	api "github.com/crypto-power/instantswap/instantswap"

	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/instantswap/blockexplorer"
	_ "github.com/crypto-power/instantswap/blockexplorer/btcexplorer" //nolint:revive
	_ "github.com/crypto-power/instantswap/blockexplorer/dcrexplorer"
)

const (
	// BTCBlockTime is the average time it takes to mine a block on the BTC network.
	BTCBlockTime = 10 * time.Minute
	// DCRBlockTime is the average time it takes to mine a block on the DCR network.
	DCRBlockTime = 5 * time.Minute

	// DefaultMarketDeviation is the maximum deviation the server rate
	// can deviate from the market rate.
	DefaultMarketDeviation = 5 // 5%
	// DefaultConfirmations is the number of confirmations required
	DefaultConfirmations = 1
	// DefaultRateRequestAmount is the amount used to perform the rate request query.
	DefaultRateRequestAmount = 1
)

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
			return errors.E(op, err)
		}

		if sourceAccountBalance.Spendable.ToCoin() <= params.BalanceToMaintain {
			log.Error("source wallet balance is less than or equals the set balance to maintain")
			return errors.E(op, "source wallet balance is less than or equals the set balance to maintain") // stop scheduling if the source wallet balance is less than or equals the set balance to maintain
		}

		rateRequestParams := api.ExchangeRateRequest{
			From:   params.Order.FromCurrency,
			To:     params.Order.ToCurrency,
			Amount: DefaultRateRequestAmount, // amount needs to be greater than 0 to get the exchange rate
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

		market := params.Order.FromCurrency + "-" + params.Order.ToCurrency
		ticker, err := mgr.ExternalService.GetTicker(ext.Binance, market)
		if err != nil {
			log.Error("unable to get market rate from binance")
			return errors.E(op, err)
		}

		exchangeServerRate := 1 / res.ExchangeRate
		binanceRate := ticker.LastTradePrice
		log.Info(params.Order.ExchangeServer.Server+" rate: ", exchangeServerRate)
		log.Info(ext.Binance+" rate: ", binanceRate)

		// check if the server rate deviates from the market rate by ± 5%
		// exit if true
		percentageDiff := math.Abs((exchangeServerRate-binanceRate)/((exchangeServerRate+binanceRate)/2)) * 100
		if percentageDiff > params.MaxDeviationRate {
			log.Error("exchange rate deviates from the market rate by more than 5%")
			return errors.E(op, "exchange rate deviates from the market rate by more than 5%")
		}

		// set the max send amount to the max limit set by the server
		invoicedAmount := res.Min

		// if the max send limit is 0, then the server does not have a max limit constraint
		// so we can send the entire source wallet balance
		// if res.Max == 0 {
		// 	invoicedAmount = sourceAccountBalance.Spendable.ToCoin() - params.BalanceToMaintain // deduct the balance to maintain from the source wallet balance
		// }

		log.Info("Order Scheduler: check balance after exchange")
		estimatedBalanceAfterExchange := sourceAccountBalance.Spendable.ToCoin() - invoicedAmount
		if estimatedBalanceAfterExchange < params.BalanceToMaintain {
			log.Error("source wallet balance after the exchange would be less than the set balance to maintain")
			return errors.E(op, "source wallet balance after the exchange would be less than the set balance to maintain") // stop scheduling if the source wallet balance after the exchange would be less than the set balance to maintain
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

		log.Infof("Order Scheduler: adding send destination, address: %s, amount: %d", order.DepositAddress, amount)
		err = sourceWallet.AddSendDestination(order.DepositAddress, amount, false)
		if err != nil {
			log.Error("error adding send destination: ", err.Error())
			return errors.E(op, err)
		}

		log.Info("Order Scheduler: broadcasting tx")
		_, err = sourceWallet.Broadcast(params.SpendingPassphrase, "")
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
			}

			log.Info("Order Scheduler: get newly created order info")
			orderInfo, err := mgr.InstantSwap.GetOrderInfo(exchangeObject, order.UUID)
			if err != nil {
				return errors.E(op, err)
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

			log.Info("Order Scheduler: order is not completed, checking again")
			continue // order is not completed, check again
		}

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

// GetShedulerRuntime returns the duration the order scheduler has been running.
func (mgr *AssetsManager) GetShedulerRuntime() string {
	return time.Since(mgr.InstantSwap.SchedulerStartTime).Round(time.Second).String()
}
