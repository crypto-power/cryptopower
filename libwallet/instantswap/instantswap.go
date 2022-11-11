package instantswap

import (
	"fmt"
	"time"

	"code.cryptopower.dev/exchange/instantswap"
	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"

	// Initialize exchange servers.
	_ "code.cryptopower.dev/exchange/instantswap/exchange/changelly"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/changenow"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/coinswitch"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/flypme"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/godex"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/simpleswap"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/swapzone"
)

func NewInstantSwap(db *storm.DB) (*InstantSwap, error) {
	if err := db.Init(&Order{}); err != nil {
		log.Errorf("Error initializing instantSwap database: %s", err.Error())
		return nil, err
	}

	return &InstantSwap{
		db: db,
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

func (instantSwap *InstantSwap) updateOrder(order *Order) error {
	return instantSwap.db.Update(order)
}

// NewExchanageServer sets up a new exchange server for use.
func (instantSwap *InstantSwap) NewExchanageServer(exchangeServer ExchangeServer) (instantswap.IDExchange, error) {
	const op errors.Op = "instantSwap.NewExchanageServer"

	exchange, err := instantswap.NewExchange(exchangeServer.ToString(), instantswap.ExchangeConfig{
		Debug:     false,
		ApiKey:    "",
		ApiSecret: "",
	})
	if err != nil {
		return nil, errors.E(op, err)
	}

	return exchange, nil
}

// GetOrdersRaw fetches and returns all saved orders.
func (instantSwap *InstantSwap) GetOrdersRaw(offset, limit int32, newestFirst bool) ([]*Order, error) {

	var query storm.Query

	query = instantSwap.db.Select(
		q.True(),
	)

	if offset > 0 {
		query = query.Skip(int(offset))
	}

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	if newestFirst {
		query = query.OrderBy("CreatedAt").Reverse()
	} else {
		query = query.OrderBy("CreatedAt")
	}

	var orders []*Order
	err := query.Find(&orders)
	if err != nil && err != storm.ErrNotFound {
		return nil, fmt.Errorf("error fetching orders: %s", err.Error())
	}

	return orders, nil
}

// GetOrderRaw fetches and returns a single order specified by it's UUID.
func (instantSwap *InstantSwap) GetOrderRaw(orderUUID string) (*Order, error) {
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

		Server:                   params.Server,
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

	return order, nil
}

func (instantSwap *InstantSwap) GetOrderInfo(exchangeObject instantswap.IDExchange, orderUUID string) (*Order, error) {
	const op errors.Op = "instantSwap.GetOrderInfo"

	order, err := instantSwap.GetOrderRaw(orderUUID)
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
		Changelly,
		ChangeNow,
		CoinSwitch,
		FlypMe,
		GoDex,
		SimpleSwap,
		SwapZone,
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
