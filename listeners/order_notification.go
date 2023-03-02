package listeners

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/wallet"
)

// ProposalNotificationListener satisfies dcr
// OrderNotificationListener interface contract.
type OrderNotificationListener struct {
	OrderNotifChan chan wallet.Order
}

func NewOrderNotificationListener() *OrderNotificationListener {
	return &OrderNotificationListener{
		OrderNotifChan: make(chan wallet.Order, 4),
	}
}

func (on *OrderNotificationListener) OnExchangeOrdersSynced() {
	on.sendNotification(wallet.Order{
		Order:       &instantswap.Order{},
		OrderStatus: wallet.OrderStatusSynced,
	})
}

func (on *OrderNotificationListener) sendNotification(signal wallet.Order) {
	if signal.Order != nil {
		select {
		case on.OrderNotifChan <- signal:
		default:
		}
	}
}
