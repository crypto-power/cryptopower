package listeners

import (
// "code.cryptopower.dev/group/cryptopower/libwallet"
// "code.cryptopower.dev/group/cryptopower/wallet"
)

// ExchangeNotificationListener satisfies dcr
// ExchangeNotificationListener interface contract.
// type ExchangeNotificationListener struct {
// 	ExchangeNotifChan chan wallet.Exchange
// }

// func NewExchangeNotificationListener() *ExchangeNotificationListener {
// 	return &ExchangeNotificationListener{
// 		ExchangeNotifChan: make(chan wallet.Exchange, 4),
// 	}
// }

// func (pn *ExchangeNotificationListener) OnExchangeOrdersSynced() {
// 	pn.sendNotification(wallet.Exchange{
// 		Exchange:       &libwallet.Exchange{},
// 		ExchangeStatus: wallet.Synced,
// 	})
// }

// func (pn *ExchangeNotificationListener) sendNotification(signal wallet.Proposal) {
// 	if signal.Proposal != nil {
// 		select {
// 		case pn.ProposalNotifChan <- signal:
// 		default:
// 		}
// 	}
// }
