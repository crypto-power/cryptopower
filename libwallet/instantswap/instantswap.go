package instantswap

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cryptopower.dev/exchange/instantswap"
	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"

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

func (instantSwap *InstantSwap) NewExchanageServer(exchangeServer, ApiKey, ApiSecret string) (instantswap.IDExchange, error) {
	const op errors.Op = "instantSwap.NewExchanageServer"

	exchange, err := instantswap.NewExchange(exchangeServer, instantswap.ExchangeConfig{
		Debug:     false,
		ApiKey:    ApiKey,
		ApiSecret: ApiSecret,
	})
	if err != nil {
		return nil, errors.E(op, err)
	}

	return exchange, nil
}

// GetOrdersRaw fetches and returns all saved orders.
func (instantSwap *InstantSwap) GetOrdersRaw(offset, limit int32, newestFirst bool) ([]Order, error) {

	var query storm.Query

	if offset > 0 {
		query = query.Skip(int(offset))
	}

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	if newestFirst {
		query = query.OrderBy("Timestamp").Reverse()
	} else {
		query = query.OrderBy("Timestamp")
	}

	var orders []Order
	err := query.Find(&orders)
	if err != nil && err != storm.ErrNotFound {
		return nil, fmt.Errorf("error fetching orders: %s", err.Error())
	}

	return orders, nil
}

// GetOrders returns the result of GetOrdersRaw as a JSON string
func (instantSwap *InstantSwap) GetOrders(offset, limit int32, newestFirst bool) (string, error) {

	result, err := instantSwap.GetOrdersRaw(offset, limit, newestFirst)
	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "[]", nil
	}

	response, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling result: %s", err.Error())
	}

	return string(response), nil
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

// GetOrderByID returns the result of GetOrderByIDRaw as a JSON string
func (instantSwap *InstantSwap) GetOrderByID(orderID int) (string, error) {
	return instantSwap.marshalResult(instantSwap.GetOrderByIDRaw(orderID))
}

// GetOrder returns the result of GetOrderRaw as a JSON string
func (instantSwap *InstantSwap) GetOrder(orderUUID string) (string, error) {
	return instantSwap.marshalResult(instantSwap.GetOrderRaw(orderUUID))
}

func (instantSwap *InstantSwap) CreateOrder(exchangeServer instantswap.IDExchange, params instantswap.CreateOrder) (*Order, error) {
	const op errors.Op = "instantSwap.CreateOrder"

	res, err := exchangeServer.CreateOrder(params)
	if err != nil {
		return nil, errors.E(op, err)
	}

	order := &Order{
		UUID: res.UUID,

		OrderedAmount: res.InvoicedAmount,
		FromCurrency:  res.FromCurrency,
		ToCurrency:    res.ToCurrency,

		DepositAddress:     res.DepositAddress,
		DestinationAddress: res.Destination,
		ExchangeRate:       res.ExchangeRate,
		ChargedFee:         res.ChargedFee,
		ExpiryTime:         res.Expires,
		CreatedAt:          time.Now().Unix(),

		ExtraID: res.ExtraID, //changenow.io requirement //changelly payinExtraId value
	}

	instantSwap.saveOrder(order)

	return order, nil
}

func (instantSwap *InstantSwap) GetOrderInfo(exchangeServer instantswap.IDExchange, orderUUID string) (*Order, error) {
	const op errors.Op = "instantSwap.GetOrderInfo"

	res, err := exchangeServer.OrderInfo(orderUUID)
	if err != nil {
		return nil, errors.E(op, err)
	}

	order := &Order{
		TxID:          res.TxID,
		ReceiveAmount: res.ReceiveAmount,
		Status:        res.Status,
		ExpiryTime:    res.Expires,
		Confirmations: res.Confirmations,
		LastUpdate:    res.LastUpdate,
	}

	err = instantSwap.updateOrder(order)
	if err != nil {
		return nil, errors.E(op, err)
	}

	return order, nil
}

func (instantSwap *InstantSwap) GetExchangeRateInfo(exchangeServer instantswap.IDExchange, params instantswap.ExchangeRateRequest) (*instantswap.ExchangeRateInfo, error) {
	const op errors.Op = "instantSwap.GetExchangeRateInfo"

	res, err := exchangeServer.GetExchangeRateInfo(params)
	if err != nil {
		return nil, errors.E(op, err)
	}

	return &res, nil
}

func (instantSwap *InstantSwap) marshalResult(result interface{}, err error) (string, error) {

	if err != nil {
		return "", err
	}

	response, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling result: %s", err.Error())
	}

	return string(response), nil
}
