package libwallet

import (
	"time"

	api "code.cryptopower.dev/group/instantswap"

	"code.cryptopower.dev/group/blockexplorer"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

const (
	BTCBlockTime = 60 * time.Minute
	DCRBlockTime = 20 * time.Minute
)

func (mgr *AssetsManager) StartScheduler(params instantswap.SchedulerParams) error {

	// Initialize the exchange server.
	exchangeObject, err := mgr.InstantSwap.NewExchanageServer(params.Order.ExchangeServer)
	if err != nil {
		log.Errorf("Error instantiating exchange server: %v", err)
		return err
	}

	for {
		sourceWallet := mgr.WalletWithID(params.Order.SourceWalletID)
		sourceAccountBalance, err := sourceWallet.GetAccountBalance(params.Order.SourceAccountNumber)
		if err != nil {
			log.Error(err)
			break
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

		estimatedBalanceAfterExchange := sourceAccountBalance.Spendable.ToCoin() - invoicedAmount
		if estimatedBalanceAfterExchange < params.BalanceToMaintain {
			log.Error("source wallet balance after the exchange would be less than the set balance to maintain")
			break // stop scheduling if the source wallet balance after the exchange would be less than the set balance to maintain
		}

		params.Order.InvoicedAmount = invoicedAmount
		order, err := mgr.InstantSwap.CreateOrder(exchangeObject, params.Order)
		if err != nil {
			log.Error(err)
			break
		}

		// wait for the order to be completed before scheduling the next order
		for {
			// depending on the block time for the asset, the order may take a while to complete
			// so we wait for the estimated block time before checking the order status
			switch params.Order.FromCurrency {
			case utils.BTCWalletAsset.String():
				time.Sleep(BTCBlockTime)
			case utils.DCRWalletAsset.String():
				time.Sleep(DCRBlockTime)
			}

			orderInfo, err := mgr.InstantSwap.GetOrderInfo(exchangeObject, order.UUID)
			if err != nil {
				log.Error(err)
				break
			}

			if orderInfo.Status == api.OrderStatusCompleted {
				// break // order is completed, break out of the loop

				// Verify that the order was completed successfully from the blockchain explorer.
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
				}

			}

			continue // order is not completed, continue waiting
		}

		// run at the specified frequency
		time.Sleep(params.Frequency * time.Hour)
	}

	return nil
}

func (mgr *AssetsManager) StopScheduler() {

}
