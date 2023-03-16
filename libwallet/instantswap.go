package libwallet

import (
	"context"
	"fmt"
	"time"

	api "code.cryptopower.dev/group/instantswap"
	"decred.org/dcrwallet/v2/errors"

	// sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/blockexplorer"
	_ "code.cryptopower.dev/group/blockexplorer/btcexplorer"
	_ "code.cryptopower.dev/group/blockexplorer/dcrexplorer"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

const (
	BTCBlockTime = 1 * time.Minute
	DCRBlockTime = 1 * time.Minute
)

func (mgr *AssetsManager) StartScheduler(ctx context.Context, params instantswap.SchedulerParams) error {
	fmt.Println("[][][]][] params: ", params)
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()

	if mgr.InstantSwap.CancelOrderScheduler != nil {
		mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
		return errors.New("scheduler already running")
	}

	mgr.InstantSwap.SchedulerCtx, mgr.InstantSwap.CancelOrderScheduler = context.WithCancel(ctx)
	defer func() {
		mgr.InstantSwap.CancelOrderScheduler = nil
	}()
	mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	// startTime := time.Now()
	log.Info("Order Scheduler: started")

	// Initialize the exchange server.
	log.Info("Order Scheduler: initializing exchange server")
	exchangeObject, err := mgr.InstantSwap.NewExchanageServer(params.Order.ExchangeServer)
	if err != nil {
		log.Errorf("Error instantiating exchange server: %v", err)
		return err
	}

	for {
		// Check if scheduler has been shutdown and exit if true.
		if mgr.InstantSwap.SchedulerCtx.Err() != nil {
			log.Info("InstantSwap.SchedulerCtx.Err()", mgr.InstantSwap.SchedulerCtx.Err())
			return mgr.InstantSwap.SchedulerCtx.Err()
		}

		log.Info("Order Scheduler: finding source wallet")
		sourceWallet := mgr.WalletWithID(params.Order.SourceWalletID)
		fmt.Println("sourceWallet: ", sourceWallet)
		sourceAccountBalance, err := sourceWallet.GetAccountBalance(params.Order.SourceAccountNumber)
		if err != nil {
			log.Error(err)
			break
		}

		defer sourceWallet.LockWallet()
		err = sourceWallet.UnlockWallet(params.SpendingPassphrase)
		if err != nil {
			log.Error(err)
			return err
		}

		if sourceAccountBalance.Spendable.ToCoin() <= params.BalanceToMaintain {
			log.Error("source wallet balance is less than or equals the set balance to maintain")
			break // stop scheduling if the source wallet balance is less than or equals the set balance to maintain
		}

		rateRequestParams := api.ExchangeRateRequest{
			From:   params.Order.FromCurrency,
			To:     params.Order.ToCurrency,
			Amount: 1,
		}
		log.Info("Order Scheduler: getting exchange rate info")

		res, err := mgr.InstantSwap.GetExchangeRateInfo(exchangeObject, rateRequestParams)
		if err != nil {
			log.Error(err)
			break
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
			break // stop scheduling if the source wallet balance after the exchange would be less than the set balance to maintain
		}

		log.Info("Order Scheduler: creating order")

		params.Order.InvoicedAmount = invoicedAmount
		order, err := mgr.InstantSwap.CreateOrder(exchangeObject, params.Order)
		if err != nil {
			log.Error("CreateOrder", err)
			break
		}

		log.Info("Order Scheduler: creating unsigned transaction")

		// construct the transaction to send the invoiced amount to the exchange server
		err = sourceWallet.NewUnsignedTx(params.Order.SourceAccountNumber)
		if err != nil {
			log.Error("NewUnsignedTx", err)
			break
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
			log.Error("AddSendDestination", err)
			break
		}

		log.Info("Order Scheduler: broadcasting")

		// _, err = sourceWallet.Broadcast(params.SpendingPassphrase, "")
		// if err != nil {
		// 	log.Error(err)
		// 	break
		// }

		// wait for the order to be completed before scheduling the next order
		var isRefunded bool
		for {
			// Check if scheduler has been shutdown and exit if true.
			if mgr.InstantSwap.SchedulerCtx.Err() != nil {
				log.Error("InstantSwap.SchedulerCtx.Err()", mgr.InstantSwap.SchedulerCtx.Err())
				return mgr.InstantSwap.SchedulerCtx.Err()
			}
			// depending on the block time for the asset, the order may take a while to complete
			// so we wait for the estimated block time before checking the order status
			switch params.Order.FromCurrency {
			case utils.BTCWalletAsset.String():
				log.Info("Order Scheduler: sleeping btc block time (10 minutes)")

				time.Sleep(BTCBlockTime)
			case utils.DCRWalletAsset.String():
				log.Info("Order Scheduler: sleeping dcr block time (5 minutes)")

				time.Sleep(DCRBlockTime)
			}

			log.Info("Order Scheduler: get newly created order info")

			orderInfo, err := mgr.InstantSwap.GetOrderInfo(exchangeObject, order.UUID)
			if err != nil {
				log.Error(err)
				break
			}

			if orderInfo.Status == api.OrderStatusRefunded {
				log.Error("order was refunded. verify that the order was refunded successfully from the blockchain explorer ")
				tmpFromCurrency := params.Order.FromCurrency
				tmpToCurrency := params.Order.ToCurrency

				// swap ToCurrency and FromCurrency
				params.Order.ToCurrency = tmpFromCurrency
				params.Order.FromCurrency = tmpToCurrency

				orderInfo.ReceiveAmount = params.Order.InvoicedAmount

				isRefunded = true
			}

			// if orderInfo.Status == api.OrderStatusCompleted {
			// Verify that the order was completed successfully from the blockchain explorer.
			// explorer, err := blockexplorer.NewExplorer(params.Order.ToCurrency, false)
			// if err != nil {
			// 	log.Error(err)
			// 	break
			// }

			// verificationInfo := blockexplorer.TxVerifyRequest{
			// 	TxId:      orderInfo.TxID,
			// 	Amount:    orderInfo.ReceiveAmount,
			// 	CreatedAt: orderInfo.CreatedAt,
			// 	Address:   orderInfo.DestinationAddress,
			// 	Confirms:  1, //TODO:STATIC until deciding for another config param??
			// }

			// verification, err := explorer.VerifyTransaction(verificationInfo)
			// if err != nil {
			// 	log.Error(err)
			// 	break
			// }

			// if verification.Verified {
			// 	if verification.BlockExplorerAmount.ToCoin() != orderInfo.ReceiveAmount {
			// 		log.Error("received amount does not match the expected amount")
			// 		break
			// 	}
			// }

			// }

			log.Info("Order Scheduler: instantiate block explorer")

			// verify that the order was completed successfully from the blockchain explorer
			explorer, err := blockexplorer.NewExplorer(params.Order.ToCurrency, false)
			if err != nil {
				log.Error(err)
				break
			}

			verificationInfo := blockexplorer.TxVerifyRequest{
				TxId:      orderInfo.TxID,
				Amount:    orderInfo.ReceiveAmount,
				CreatedAt: orderInfo.CreatedAt,
				Address:   orderInfo.DestinationAddress,
				Confirms:  1, //TODO:STATIC until deciding for another config param??
			}

			log.Info("Order Scheduler: verify transaction")

			verification, err := explorer.VerifyTransaction(verificationInfo)
			if err != nil {
				log.Error(err)
				break
			}

			if verification.Verified {
				if verification.BlockExplorerAmount.ToCoin() != orderInfo.ReceiveAmount {
					log.Error("received amount does not match the expected amount")
					break
				}

				if isRefunded {
					log.Info("order was refunded successfully")
				} else {
					log.Info("order was completed successfully")
				}
			}

			continue // order is not completed, continue waiting
		}
		log.Info("Order Scheduler: try again using frequency set")

		// run at the specified frequency
		time.Sleep(params.Frequency * time.Hour)
	}

	return nil
}

func (mgr *AssetsManager) StopScheduler() {
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()
	if mgr.InstantSwap.CancelOrderScheduler != nil {
		mgr.InstantSwap.CancelOrderScheduler()
		mgr.InstantSwap.CancelOrderScheduler = nil
	}
	mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	log.Info("Order scheduler: stopped")
}

func (mgr *AssetsManager) IsOrderSchedulerRunning() bool {
	mgr.InstantSwap.CancelOrderSchedulerMu.RLock()
	defer mgr.InstantSwap.CancelOrderSchedulerMu.RUnlock()
	return mgr.InstantSwap.CancelOrderScheduler != nil
}

// func (mgr *AssetsManager) constructTx(depositAddress string, unitAmount float64, sourceWallet sharedW.Asset) error {
// 	destinationAddress := depositAddress

// 	sourceAccount := com.sourceAccountSelector.SelectedAccount()
// 	err := com.sourceWalletSelector.SelectedWallet().NewUnsignedTx(sourceAccount.Number)
// 	if err != nil {
// 		return err
// 	}

// 	var amount int64
// 	switch com.sourceWalletSelector.SelectedWallet().GetAssetType().ToStringLower() {
// 	case utils.BTCWalletAsset.ToStringLower():
// 		amount = btc.AmountSatoshi(unitAmount)
// 	case utils.DCRWalletAsset.ToStringLower():
// 		amount = dcr.AmountAtom(unitAmount)
// 	}
// 	err = com.sourceWalletSelector.SelectedWallet().AddSendDestination(destinationAddress, amount, false)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
