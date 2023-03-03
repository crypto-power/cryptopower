package wallet

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
)

type OrderStatus int

const (
	OrderStatusSynced OrderStatus = iota
)

type Order struct {
	Order       *instantswap.Order
	OrderStatus OrderStatus
}
