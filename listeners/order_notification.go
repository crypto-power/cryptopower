package listeners

import (
	"gitlab.com/cryptopower/cryptopower/libwallet/instantswap"
	"gitlab.com/cryptopower/cryptopower/wallet"
)

// OrderNotificationListener satisfies libwallet OrderNotificationListener
// interface contract.
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

func (on *OrderNotificationListener) OnOrderCreated(order *instantswap.Order) {
	on.sendNotification(wallet.Order{
		Order:       order,
		OrderStatus: wallet.OrderCreated,
	})
}

func (on *OrderNotificationListener) OnOrderSchedulerStarted() {
	on.sendNotification(wallet.Order{
		Order:       &instantswap.Order{},
		OrderStatus: wallet.OrderSchedulerStarted,
	})
}

func (on *OrderNotificationListener) OnOrderSchedulerEnded() {
	on.sendNotification(wallet.Order{
		Order:       &instantswap.Order{},
		OrderStatus: wallet.OrderSchedulerEnded,
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
