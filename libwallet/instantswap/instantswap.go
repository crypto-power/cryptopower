package instantswap

import (
	"fmt"
	"sync"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/crypto-power/instantswap/instantswap"

	// Initialize exchange servers.
	_ "github.com/crypto-power/instantswap/instantswap/exchange/changelly"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/changenow"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/coinswitch"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/flypme"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/godex"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/simpleswap"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/swapzone"
)

const (
	// API_KEY_CHANGENOW is the changenow API key.
	API_KEY_CHANGENOW = "249665653f1bbc620a70b4a6d25d0f8be126552e30c253df87685b880183be93"
	// API_KEY_GODEX is the godex API key.
	API_KEY_GODEX = "lPM1O83kxGXJn9CpMhVRc8Yx22Z3h2/1EWyZ3lDoqtqEPYJqimHxysLKm7RN5HO3QyH9PMXZy7n3CUQhF40cYWY2zg==a44e77479feb30c28481c020bce2a3b3"
)

func NewInstantSwap(db *storm.DB) (*InstantSwap, error) {
	if err := db.Init(&Order{}); err != nil {
		log.Errorf("Error initializing instantSwap database: %s", err.Error())
		return nil, err
	}

	return &InstantSwap{
		db: db,
		mu: &sync.RWMutex{},

		notificationListenersMu: &sync.RWMutex{},

		notificationListeners: make(map[string]OrderNotificationListener),
	}, nil
}

func (instantSwap *InstantSwap) saveOrOverwriteOrder(order *Order) error {
	var oldOrder Order
	err := instantSwap.db.One("UUID", order.UUID, &oldOrder)
	if err != nil && err != storm.ErrNotFound {
		return errors.Errorf("error checking if order was already indexed: %s", err.Error())
	}

	if oldOrder.UUID != "" {
		// delete old record before saving new (if it exists)
		instantSwap.db.DeleteStruct(oldOrder)
	}

	return instantSwap.db.Save(order)
}

func (instantSwap *InstantSwap) saveOrder(order *Order) error {
	return instantSwap.db.Save(order)
}

// UpdateOrder updates an order in the database.
func (instantSwap *InstantSwap) UpdateOrder(order *Order) error {
	return instantSwap.updateOrder(order)
}

func (instantSwap *InstantSwap) updateOrder(order *Order) error {
	return instantSwap.db.Update(order)
}

// NewExchangeServer sets up a new exchange server for use.
func (instantSwap *InstantSwap) NewExchangeServer(exchangeServer ExchangeServer) (instantswap.IDExchange, error) {
	const op errors.Op = "instantSwap.NewExchangeServer"

	exchange, err := instantswap.NewExchange(exchangeServer.Server.ToString(), instantswap.ExchangeConfig{
		Debug:       exchangeServer.Config.Debug,
		ApiKey:      exchangeServer.Config.ApiKey,
		ApiSecret:   exchangeServer.Config.ApiSecret,
		AffiliateId: exchangeServer.Config.AffiliateId,
	})
	if err != nil {
		return nil, errors.E(op, err)
	}

	return exchange, nil
}

// GetOrdersRaw fetches and returns all saved orders.
// If status is specified, only orders with that status will be returned.
// status is made optional to the sync functionality can update all orders.
func (instantSwap *InstantSwap) GetOrdersRaw(offset, limit int32, newestFirst bool, status ...instantswap.Status) ([]*Order, error) {

	var query storm.Query
	query = instantSwap.db.Select(
		q.True(),
	)

	if len(status) > 0 {
		query = instantSwap.db.Select(
			q.Eq("Status", status[0]),
		)
	}

	if offset > 0 {
		query = query.Skip(int(offset))
	}

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	query = query.OrderBy("CreatedAt")
	if newestFirst {
		query = query.Reverse()
	}

	var orders []*Order
	err := query.Find(&orders)
	if err != nil && err != storm.ErrNotFound {
		return nil, fmt.Errorf("error fetching orders: %s", err.Error())
	}

	return orders, nil
}

// GetOrderByUUIDRaw fetches and returns a single order specified by it's UUID.
func (instantSwap *InstantSwap) GetOrderByUUIDRaw(orderUUID string) (*Order, error) {
	var order Order
	err := instantSwap.db.One("UUID", orderUUID, &order)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// GetOrderByIDRaw fetches and returns a single order specified by it's ID
func (instantSwap *InstantSwap) GetOrderByIDRaw(orderID int) (*Order, error) {
	var order Order
	err := instantSwap.db.One("ID", orderID, &order)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (instantSwap *InstantSwap) CreateOrder(exchangeObject instantswap.IDExchange, params Order) (*Order, error) {
	const op errors.Op = "instantSwap.CreateOrder"

	data := instantswap.CreateOrder{
		RefundAddress:  params.RefundAddress,      // if the trading fails, the exchange will refund coins here
		Destination:    params.DestinationAddress, // your exchanged coins will be sent here
		FromCurrency:   params.FromCurrency,
		ToCurrency:     params.ToCurrency,
		InvoicedAmount: params.InvoicedAmount, // use InvoicedAmount or InvoicedAmount
	}

	res, err := exchangeObject.CreateOrder(data)
	if err != nil {
		return nil, errors.E(op, err)
	}

	order := &Order{
		UUID: res.UUID,

		ExchangeServer:           params.ExchangeServer,
		SourceWalletID:           params.SourceWalletID,
		SourceAccountNumber:      params.SourceAccountNumber,
		DestinationWalletID:      params.DestinationWalletID,
		DestinationAccountNumber: params.DestinationAccountNumber,

		InvoicedAmount: res.InvoicedAmount,
		OrderedAmount:  res.OrderedAmount,
		FromCurrency:   res.FromCurrency,
		ToCurrency:     res.ToCurrency,

		DepositAddress:     res.DepositAddress,
		RefundAddress:      res.DepositAddress,
		DestinationAddress: res.Destination,
		ExchangeRate:       res.ExchangeRate,
		ChargedFee:         res.ChargedFee,
		ExpiryTime:         res.Expires,
		Status:             instantswap.OrderStatusWaitingForDeposit,
		CreatedAt:          time.Now().Unix(),

		ExtraID: res.ExtraID, //changenow.io requirement //changelly payinExtraId value
	}

	instantSwap.saveOrder(order)
	instantSwap.publishOrderCreated(order)

	return order, nil
}

func (instantSwap *InstantSwap) GetOrderInfo(exchangeObject instantswap.IDExchange, orderUUID string) (*Order, error) {
	const op errors.Op = "instantSwap.GetOrderInfo"

	order, err := instantSwap.GetOrderByUUIDRaw(orderUUID)
	if err != nil {
		return nil, errors.E(op, err)
	}

	res, err := exchangeObject.OrderInfo(orderUUID)
	if err != nil {
		return nil, errors.E(op, err)
	}

	order.TxID = res.TxID
	order.ReceiveAmount = res.ReceiveAmount
	order.Status = res.InternalStatus
	order.ExpiryTime = res.Expires
	order.Confirmations = res.Confirmations
	order.LastUpdate = res.LastUpdate

	err = instantSwap.updateOrder(order)
	if err != nil {
		return nil, errors.E(op, err)
	}

	return order, nil
}

func (instantSwap *InstantSwap) GetExchangeRateInfo(exchangeObject instantswap.IDExchange, params instantswap.ExchangeRateRequest) (*instantswap.ExchangeRateInfo, error) {
	const op errors.Op = "instantSwap.GetExchangeRateInfo"

	res, err := exchangeObject.GetExchangeRateInfo(params)
	if err != nil {
		return nil, errors.E(op, err)
	}

	return &res, nil
}

func (instantSwap *InstantSwap) ExchangeServers() []ExchangeServer {
	return []ExchangeServer{
		{
			ChangeNow,
			ExchangeConfig{
				ApiKey: API_KEY_CHANGENOW,
			},
		},
		{
			FlypMe,
			ExchangeConfig{},
		},
		{
			GoDex,
			ExchangeConfig{
				ApiKey: API_KEY_GODEX,
			},
		},
	}
}

// DeleteOrders deletes all orders saved to the DB.
func (instantSwap *InstantSwap) DeleteOrders() error {
	err := instantSwap.db.Drop(&Order{})
	if err != nil {
		return err
	}

	return instantSwap.db.Init(&Order{})
}

func (instantSwap *InstantSwap) DeleteOrder(order *Order) error {
	return instantSwap.db.DeleteStruct(order)
}
