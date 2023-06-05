package wallet

import (
	"gitlab.com/cryptopower/cryptopower/libwallet/instantswap"
)

type OrderStatus int

const (
	OrderStatusSynced OrderStatus = iota
	OrderCreated
	OrderSchedulerStarted
	OrderSchedulerEnded
)

type Order struct {
	Order       *instantswap.Order
	OrderStatus OrderStatus
}
