package instantswap

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"time"
)

func (instantSwap *InstantSwap) StartScheduler(params SchedulerParams, wallet *sharedW.Wallet) error {

	// Initialize the exchange server.
	exchangeObject, err := instantSwap.NewExchanageServer(params.Order.ExchangeServer)
	if err != nil {
		log.Errorf("Error instantiating exchange server: %v", err)
		return err
	}

	for {
		_, err = instantSwap.CreateOrder(exchangeObject, params.Order)
		if err != nil {
			return err
		}

		// run at the specified frequency
		time.Sleep(params.Frequency * time.Hour)
	}

	// return nil
}

func (instantSwap *InstantSwap) StopScheduler() {

}
