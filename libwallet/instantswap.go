package libwallet

import (
	"context"
	"math"
	"time"

	api "code.cryptopower.dev/group/instantswap"
	"decred.org/dcrwallet/v2/errors"

	"code.cryptopower.dev/group/blockexplorer"
	_ "code.cryptopower.dev/group/blockexplorer/btcexplorer"
	_ "code.cryptopower.dev/group/blockexplorer/dcrexplorer"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	"code.cryptopower.dev/group/cryptopower/libwallet/ext"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

const (
	BTCBlockTime = 10 * time.Minute
	DCRBlockTime = 5 * time.Minute

	DefaultMarketDeviation = 5 // 5%
	DefaultConfirmations   = 1
)

func (mgr *AssetsManager) StartScheduler(ctx context.Context, params instantswap.SchedulerParams) error {
	const op errors.Op = "mgr.StartScheduler"
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()

	if mgr.InstantSwap.CancelOrderScheduler != nil {
		mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
		return errors.New("scheduler already running")
	}

	log.Info("Order Scheduler: started")
	mgr.InstantSwap.SchedulerStartTime = time.Now()
	mgr.InstantSwap.SchedulerCtx, mgr.InstantSwap.CancelOrderScheduler = context.WithCancel(ctx)
	mgr.InstantSwap.PublishOrderSchedulerStarted()
	defer func() {
		mgr.InstantSwap.CancelOrderScheduler = nil
		mgr.InstantSwap.SchedulerStartTime = time.Time{}
		mgr.InstantSwap.PublishOrderSchedulerEnded()
	}()
	mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()

	// Initialize the exchange server.
	log.Info("Order Scheduler: initializing exchange server")
	exchangeObject, err := mgr.InstantSwap.NewExchanageServer(params.Order.ExchangeServer)
	if err != nil {
		return errors.E(op, err)
	}

	for {
		// Check if scheduler has been shutdown and exit if true.
		if mgr.InstantSwap.SchedulerCtx.Err() != nil {
			return mgr.InstantSwap.SchedulerCtx.Err()
		}

		log.Info("Order Scheduler: finding source wallet")
		sourceWallet := mgr.WalletWithID(params.Order.SourceWalletID)
		sourceAccountBalance, err := sourceWallet.GetAccountBalance(params.Order.SourceAccountNumber)
		if err != nil {
			return err
		}

		defer sourceWallet.LockWallet()
		err = sourceWallet.UnlockWallet(params.SpendingPassphrase)
		if err != nil {
			log.Error(err)
			return err
		}

		if sourceAccountBalance.Spendable.ToCoin() <= params.BalanceToMaintain {
			log.Error("source wallet balance is less than or equals the set balance to maintain")
			return errors.E(op, err) // stop scheduling if the source wallet balance is less than or equals the set balance to maintain
		}

		rateRequestParams := api.ExchangeRateRequest{
			From:   params.Order.FromCurrency,
			To:     params.Order.ToCurrency,
			Amount: 1,
		}
		log.Info("Order Scheduler: getting exchange rate info")
		res, err := mgr.InstantSwap.GetExchangeRateInfo(exchangeObject, rateRequestParams)
		if err != nil {
			return errors.E(op, err)
		}

		if params.MinimumExchangeRate <= 0 {
			params.MinimumExchangeRate = DefaultMarketDeviation // default 5%
		}

		market := params.Order.FromCurrency + "-" + params.Order.ToCurrency
		ticker, err := mgr.ExternalService.GetTicker(ext.Binance, market)
		if err != nil {
			return errors.E(op, err)
		}

		exchangeServerRate := 1 / res.ExchangeRate
		binanceRate := ticker.LastTradePrice
		log.Info(params.Order.ExchangeServer.Server+" rate: ", exchangeServerRate)
		log.Info(ext.Binance+" rate: ", binanceRate)

		// check if the server rate deviates from the market rate by more than 5%
		// exit
		percentageDiff := math.Abs((exchangeServerRate-binanceRate)/((exchangeServerRate+binanceRate)/2)) * 100
		if percentageDiff > params.MinimumExchangeRate {
			log.Error("exchange rate deviates from the market rate by more than 5%")
			return errors.E(op, err)
		}

		// set the max send amount to the max limit set by the server
		invoicedAmount := res.Max

		// if the max send limit is 0, then the server does not have a max limit constraint
		// so we can send the entire source wallet balance
		if res.Max == 0 {
			invoicedAmount = sourceAccountBalance.Spendable.ToCoin() - params.BalanceToMaintain // deduct the balance to maintain from the source wallet balance
		}

		log.Info("Order Scheduler: check balance after exchange")
		estimatedBalanceAfterExchange := sourceAccountBalance.Spendable.ToCoin() - invoicedAmount
		if estimatedBalanceAfterExchange < params.BalanceToMaintain {
			log.Error("source wallet balance after the exchange would be less than the set balance to maintain")
			return errors.E(op, err) // stop scheduling if the source wallet balance after the exchange would be less than the set balance to maintain
		}

		log.Info("Order Scheduler: creating order")
		params.Order.InvoicedAmount = invoicedAmount
		order, err := mgr.InstantSwap.CreateOrder(exchangeObject, params.Order)
		if err != nil {
			return err
		}

		log.Info("Order Scheduler: creating unsigned transaction")
		// construct the transaction to send the invoiced amount to the exchange server
		err = sourceWallet.NewUnsignedTx(params.Order.SourceAccountNumber)
		if err != nil {
			return errors.E(op, err)
		}

		var amount int64
		switch sourceWallet.GetAssetType().ToStringLower() {
		case utils.BTCWalletAsset.ToStringLower():
			amount = btc.AmountSatoshi(params.Order.InvoicedAmount)
		case utils.DCRWalletAsset.ToStringLower():
			amount = dcr.AmountAtom(params.Order.InvoicedAmount)
		}

		log.Info("Order Scheduler: adding send destination")
		err = sourceWallet.AddSendDestination(order.DepositAddress, amount, false)
		if err != nil {
			return errors.E(op, err)
		}

		log.Info("Order Scheduler: broadcasting tx")
		_, err = sourceWallet.Broadcast(params.SpendingPassphrase, "")
		if err != nil {
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
			switch params.Order.FromCurrency {
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
				log.Error("order was refunded. verifing that the order was refunded successfully from the blockchain explorer")
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
			explorer, err := blockexplorer.NewExplorer(params.Order.ToCurrency, false)
			if err != nil {
				return errors.E(op, err)
			}

			verificationInfo := blockexplorer.TxVerifyRequest{
				TxId:      orderInfo.TxID,
				Amount:    orderInfo.ReceiveAmount,
				CreatedAt: orderInfo.CreatedAt,
				Address:   orderInfo.DestinationAddress,
				Confirms:  DefaultConfirmations,
			}

			log.Info("Order Scheduler: verify transaction")
			verification, err := explorer.VerifyTransaction(verificationInfo)
			if err != nil {
				return errors.E(op, err)
			}

			if verification.Verified {
				if verification.BlockExplorerAmount.ToCoin() != orderInfo.ReceiveAmount {
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

		log.Info("Order Scheduler: creating next order based on selected frequency")
		// run at the specified frequency
		time.Sleep(params.Frequency * time.Hour)

		continue
	}
}

func (mgr *AssetsManager) StopScheduler() {
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()
	if mgr.InstantSwap.CancelOrderScheduler != nil {
		mgr.InstantSwap.CancelOrderScheduler()
		mgr.InstantSwap.CancelOrderScheduler = nil
	}
	mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	log.Info("Order Scheduler: stopped")
}

func (mgr *AssetsManager) IsOrderSchedulerRunning() bool {
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()
	defer mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	return mgr.InstantSwap.CancelOrderScheduler != nil
}

func (mgr *AssetsManager) GetShedulerRuntime() string {
	return time.Since(mgr.InstantSwap.SchedulerStartTime).Round(time.Second).String()
}
