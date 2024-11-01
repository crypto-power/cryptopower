package instantswap

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"decred.org/dcrwallet/v4/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/crypto-power/cryptopower/appos"
	"github.com/crypto-power/instantswap/instantswap"

	// load instantswap exchange packages
	_ "github.com/crypto-power/instantswap/instantswap/exchange/changelly"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/changenow"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/flypme"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/godex"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/simpleswap"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/swapzone"
	_ "github.com/crypto-power/instantswap/instantswap/exchange/trocador"
)

//go:embed instant.json

var instants []byte
var privKeyMap = map[Server]string{
	Trocador:  "",
	ChangeNow: "",
	GoDex:     "",
}

func init() {
	if appos.Current().IsMobile() {
		// Initialize private key map from embedded data
		initPrivKeyMap(instants)
		return
	}

	// Call checkAndCreateInstantJSON to ensure instant.json is available
	if err := checkAndCreateInstantJSON(); err != nil {
		panic(errors.Errorf("Error setting up instant.json: %s", err.Error()))
	}

	// Load the instant.json content into the instants variable
	instantFilePath := getFilePath("instant.json") // Ensure correct path is used for reading
	instantsData, err := os.ReadFile(instantFilePath)
	if err != nil {
		panic(errors.Errorf("Error reading instant.json: %s", err.Error()))
	}

	// Initialize private key map
	initPrivKeyMap(instantsData)
}

// initPrivKeyMap initializes the private key map from the JSON data
func initPrivKeyMap(data []byte) {
	var newPrivKeyMap = make(map[Server]string)
	err := json.Unmarshal(data, &newPrivKeyMap)
	if err != nil {
		panic(err)
	}
	for key := range privKeyMap {
		if val, ok := newPrivKeyMap[key]; ok {
			privKeyMap[key] = val
		}
	}
	for key, val := range privKeyMap {
		if val == "" {
			delete(privKeyMap, key)
		}
	}
	// add flypme to privKeyMap because it does not requires private key to access
	privKeyMap[FlypMe] = ""
}

// checkAndCreateInstantJSON checks if instant.json exists, and if not, copies instant_example.json to instant.json.
func checkAndCreateInstantJSON() error {
	exampleFile := getFilePath("instant_example.json")
	instantFile := getFilePath("instant.json")

	// Check if instant.json exists
	if _, err := os.Stat(instantFile); os.IsNotExist(err) {
		// If instant.json doesn't exist, copy from instant_example.json
		err := copyFile(exampleFile, instantFile)
		if err != nil {
			return errors.Errorf("failed to copy %s to %s: %s", exampleFile, instantFile, err.Error())
		}
		fmt.Println("instant.json created from instant_example.json")
	} else if err != nil {
		return errors.Errorf("failed to check if %s exists: %s", instantFile, err.Error())
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// getFilePath constructs the file path relative to the current directory.
func getFilePath(fileName string) string {
	basePath, _ := os.Getwd() // Get current working directory
	filePath := filepath.Join(basePath, "libwallet", "instantswap", fileName)
	return filePath
}

func GetInstantExchangePrivKey(server Server) (string, bool) {
	key, ok := privKeyMap[server]
	return key, ok
}

func NewInstantSwap(db *storm.DB) (*InstantSwap, error) {
	if err := db.Init(&Order{}); err != nil {
		log.Errorf("Error initializing instantSwap database: %s", err.Error())
		return nil, err
	}

	// TODO: Callers should provide a ctx that is tied to the lifetime of the
	// app, since InstantSwap is not tied to any single page. If it is tied to a
	// specific page, then that page's ctx should be provided.
	ctx := context.TODO()

	return &InstantSwap{
		db:  db,
		ctx: ctx,

		notificationListenersMu: &sync.RWMutex{},
		notificationListeners:   make(map[string]*OrderNotificationListener),
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
		_ = instantSwap.db.DeleteStruct(oldOrder)
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
		ApiKey:      exchangeServer.Config.APIKey,
		ApiSecret:   exchangeServer.Config.APISecret,
		AffiliateId: exchangeServer.Config.AffiliateID,
	})
	if err != nil {
		return nil, errors.E(op, err)
	}

	return exchange, nil
}

// GetOrdersRaw fetches and returns all saved orders.
// If status is specified, only orders with that status will be returned.
// status is made optional to the sync functionality can update all orders.
func (instantSwap *InstantSwap) GetOrdersRaw(offset, limit int32, newestFirst bool, server, txID string, status ...instantswap.Status) ([]*Order, error) {
	matchers := make([]q.Matcher, 0)

	if len(status) > 0 {
		matchers = append(matchers, q.Eq("Status", status[0]))
	}

	if server != "" {
		matchers = append(matchers, q.Eq("Server", server))
	}

	if txID != "" {
		matchers = append(matchers, q.Eq("TxID", strings.TrimSpace(txID)))
	}

	if len(matchers) == 0 {
		matchers = append(matchers, q.True())
	}

	var query storm.Query
	query = instantSwap.db.Select(matchers...)

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

	data := instantswap.CreateOrder{
		RefundAddress:  params.RefundAddress,      // if the trading fails, the exchange will refund coins here
		Destination:    params.DestinationAddress, // your exchanged coins will be sent here
		FromCurrency:   params.FromCurrency,
		ToCurrency:     params.ToCurrency,
		InvoicedAmount: params.InvoicedAmount, // use InvoicedAmount or InvoicedAmount
		FromNetwork:    params.FromNetwork,
		ToNetwork:      params.ToNetwork,
		Signature:      params.Signature,
		Provider:       params.Provider,
	}

	res, err := exchangeObject.CreateOrder(data)
	if err != nil {
		return nil, err
	}

	order := &Order{
		UUID: res.UUID,

		Server:                   params.Server,
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

		ExtraID: res.ExtraID, // changenow.io requirement //changelly payinExtraId value
	}

	_ = instantSwap.saveOrder(order)
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
	var exchanges []ExchangeServer
	for exchangeName, privKey := range privKeyMap {
		exchanges = append(exchanges, ExchangeServer{
			Server: exchangeName,
			Config: ExchangeConfig{
				APIKey: privKey,
			},
		})
	}
	return exchanges
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
