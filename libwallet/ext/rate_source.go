// Copyright (c) 2019-2021, The Decred developers
// Copyright (c) 2023, The Cryptopower developers
// See LICENSE for details.

package ext

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	// These are constants used to represent various rate sources supported.
	bittrex = values.BittrexExchange
	binance = values.BinanceExchange
	none    = values.DefaultExchangeValue

	// These are method names for expected bittrex specific websocket messages.
	BittrexMsgHeartbeat  = "heartbeat"
	BittrexMarketSummary = "marketSummary"
	BittrexTicker        = "ticker"

	// MktSep is used repo wide to separate market symbols.
	MktSep = "-"
)

var (
	// These are urls to fetch rate information from the Bittrex exchange.
	bittrexURLs = sourceURLs{
		price: "https://api.bittrex.com/v3/markets/%s/ticker",
		stats: "https://api.bittrex.com/v3/markets/%s/summary",
		// Bittrex uses SignalR, which retrieves the actual websocket endpoint
		// via HTTP.
		ws: "socket-v3.bittrex.com",
	}

	// These are urls to fetch rate information from the Binance exchange.
	binanceURLs = sourceURLs{
		// See: https://binance-docs.github.io/apidocs/spot/en/#current-average-price
		price: "https://api.binance.com/api/v3/ticker/24hr?symbol=%s",
		ws:    "wss://stream.binance.com:9443/stream?streams=%s",
	}

	// supportedMarkets is a map of markets supported by rate sources
	// implemented (Binance, Bittrex).
	supportedMarkets = map[string]*struct{}{
		values.BTCUSDTMarket: {},
		values.DCRUSDTMarket: {},
		values.LTCUSDTMarket: {},
		values.DCRBTCMarket:  {},
		values.LTCBTCMarket:  {},
	}

	// binanceMarkets is a map of Binance formatted market to the repo's format,
	// e.g BTCUSDT : BTC-USDT. This is to facilitate quick lookup and to/fro
	// market name formatting.
	binanceMarkets = make(map[string]string)

	// Rates exceeding rateExpiry are expired and should be removed unless there
	// was an error fetching a new rate.
	rateExpiry = 30 * time.Minute

	// Rate sources should be refreshed every RateRefreshDuration to replace
	// expired rates and reconnect websocket if need be.
	RateRefreshDuration = 60 * time.Minute

	rateNotificationInterval = 5 * time.Minute

	bittrexRateSubscription = signalRClientMsg{
		H: "c3",
		M: "Subscribe",
		A: []interface{}{},
	}
)

// Prepare subscription data for supported rate sources.
func init() {
	channels := []string{"heartbeat"}
	var binanceParams []string
	for market := range supportedMarkets {
		channels = append(channels, BittrexTicker+"_"+market)
		channels = append(channels, BittrexMarketSummary+"_"+market)
		binanceMarketName := strings.ReplaceAll(market, MktSep, "")
		binanceMarkets[binanceMarketName] = market
		binanceParams = append(binanceParams, strings.ToLower(binanceMarketName)+"@ticker")
	}

	bittrexRateSubscription.A = append(bittrexRateSubscription.A, channels)
	// See: https://binance-docs.github.io/apidocs/spot/en/#websocket-market-streams
	binanceURLs.ws = fmt.Sprintf(binanceURLs.ws, strings.Join(binanceParams, "/"))
}

// RateSource is the interface that binds different rate sources. Most of the
// methods are implemented by CommonRateSource, but Refresh is implemented in
// the individual rate source.
type RateSource interface {
	Name() string
	Ready() bool
	Refresh(force bool)
	Refreshing() bool
	LastUpdate() time.Time
	GetTicker(market string) *Ticker
	ToggleStatus(disable bool)
	ToggleSource(newSource string) error
	AddRateListener(listener *RateListener, uniqueID string) error
	RemoveRateListener(uniqueID string)
}

// CommonRateSource is an external rate source for fiat and crypto-currency
// rates. These rates are estimates and maybe be affected by server latency and
// should not be used for actual buy or sell orders except to display reasonable
// estimates. CommonRateSource is embedded in all of the rate sources supported.
type CommonRateSource struct {
	ctx           context.Context
	source        string
	disabled      bool
	mtx           sync.RWMutex
	tickers       map[string]*Ticker
	refreshing    bool
	cond          *sync.Cond
	getTicker     func(market string) (*Ticker, error)
	sourceChanged chan *struct{}
	lastUpdate    time.Time

	wsMtx  sync.RWMutex
	ws     websocketFeed
	wsSync struct {
		errCount   int
		lastUpdate time.Time
		fail       time.Time
	}
	// wsProcessor is used to process websocket messages.
	wsProcessor WebsocketProcessor

	rateListenersMtx sync.RWMutex
	rateListeners    map[string]*RateListener
	lastNotified     time.Time
}

// Name is the string associated with the rate source for display.
func (cs *CommonRateSource) Name() string {
	src := cs.source
	return strings.ToUpper(src[:1]) + src[1:]
}

func (cs *CommonRateSource) Ready() bool {
	return len(cs.tickers) > 0 && !cs.isDisabled()
}

func (cs *CommonRateSource) LastUpdate() time.Time {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.lastUpdate
}

func (cs *CommonRateSource) Refreshing() bool {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.refreshing
}

func (cs *CommonRateSource) ratesUpdated(t time.Time) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.lastUpdate = t
}

func (cs *CommonRateSource) ToggleStatus(disable bool) {
	if cs.isDisabled() != disable {
		return
	}

	cs.mtx.Lock()
	cs.disabled = disable
	cs.mtx.Unlock()

	cs.resetWs(nil)
}

func (cs *CommonRateSource) isDisabled() bool {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.disabled
}

// ToggleSource changes the rate source to newSource. This method takes some
// time to refresh the rates and should be executed a a goroutine.
func (cs *CommonRateSource) ToggleSource(newSource string) error {
	if newSource == cs.source {
		return nil // nothing to do
	}

	getTickerFn := dummyGetTickerFunc
	wsProcessor := func([]byte) ([]*Ticker, error) { return nil, nil }
	refresh := true
	switch newSource {
	case none: /* none is the dummy rate source for when user disables rates */
		refresh = false
	case binance:
		getTickerFn = binanceGetTicker
		wsProcessor = processBinanceWsMessage
	case bittrex:
		getTickerFn = binanceGetTicker
		wsProcessor = processBittrexWsMessage
	default:
		return fmt.Errorf("New rate source %s is not supported", newSource)
	}

	// Update source specific fields.
	cs.mtx.Lock()
	cs.source = newSource
	cs.getTicker = getTickerFn
	cs.tickers = make(map[string]*Ticker)
	cs.mtx.Unlock()

	cs.resetWs(wsProcessor)

	go cs.notifyRateListeners(true)

	if refresh {
		cs.Refresh(true)
	}

	return nil
}

// resetWs resets the rate source's websocket connect and related data.
// processor is optional.
func (cs *CommonRateSource) resetWs(processor WebsocketProcessor) {
	cs.wsMtx.Lock()
	defer cs.wsMtx.Unlock()
	// Update websocket fields
	var tZero time.Time
	if cs.ws != nil {
		cs.ws.Close()
		cs.ws = nil
	}
	if processor != nil {
		cs.wsProcessor = processor
	}
	cs.wsSync.errCount = 0
	cs.wsSync.fail = tZero
	cs.wsSync.lastUpdate = tZero
}

func (cs *CommonRateSource) AddRateListener(listener *RateListener, uniqueID string) error {
	cs.rateListenersMtx.Lock()
	defer cs.rateListenersMtx.Unlock()

	_, ok := cs.rateListeners[uniqueID]
	if ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	cs.rateListeners[uniqueID] = listener
	return nil
}

func (cs *CommonRateSource) RemoveRateListener(uniqueID string) {
	cs.rateListenersMtx.Lock()
	defer cs.rateListenersMtx.Unlock()
	delete(cs.rateListeners, uniqueID)
}

// Log the error along with the token and an additional passed identifier.
func (cs *CommonRateSource) fail(msg string, err error) {
	log.Errorf("%s: %s: %v", cs.source, msg, err)
}

// WebsocketProcessor is a callback for new websocket messages from the server.
type WebsocketProcessor func([]byte) ([]*Ticker, error)

// Only the fields are protected for these. (websocketFeed).Write has
// concurrency control.
func (cs *CommonRateSource) websocket() (websocketFeed, WebsocketProcessor) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.ws, cs.wsProcessor
}

// addWebsocketConnection adds a websocket connection.
func (cs *CommonRateSource) addWebsocketConnection(ws websocketFeed) {
	cs.wsMtx.Lock()
	// Ensure that any previous websocket is closed.
	if cs.ws != nil {
		cs.ws.Close()
	}
	cs.ws = ws
	cs.wsMtx.Unlock()

	cs.startWebsocket()
}

// Creates a websocket connection and starts a listen loop. Closes any existing
// connections for this exchange.
func (cs *CommonRateSource) connectWebsocket() error {
	var ws websocketFeed
	var err error
	var subscribeMsg any
	switch cs.source {
	case binance:
		ws, err = newSocketConnection(&socketConfig{address: binanceURLs.ws})
	case bittrex:
		subscribeMsg = bittrexRateSubscription
		ws, err = connectSignalRWebsocket(bittrexURLs.ws, "/signalr")
	default:
		return errors.New("Websocket connection not supported")
	}
	if err != nil {
		return err
	}

	cs.addWebsocketConnection(ws)
	if cs.source == bittrex {
		err = cs.wsSendJSON(subscribeMsg)
		if err != nil {
			return fmt.Errorf("Failed to send tickers subscription: %w", err)
		}
	}

	return nil
}

// The listen loop for a websocket connection.
func (cs *CommonRateSource) startWebsocket() {
	ws, processor := cs.websocket()
	go func() {
		for {
			if !ws.On() || cs.ctx.Err() != nil {
				return
			}

			message, err := ws.Read()
			if err != nil {
				if ws.On() {
					cs.setWsFail(err)
				}
				return // last close error msg for previous websocket connect.
			}

			tickers, err := processor(message)
			if err != nil {
				cs.setWsFail(err)
				return
			}

			if len(tickers) == 0 {
				continue
			}

			// Update ticker.
			for _, ticker := range tickers {
				market := ticker.Market
				cs.mtx.Lock()
				if _, ok := cs.tickers[market]; !ok {
					cs.tickers[market] = &Ticker{Market: market}
				}

				if ticker.LastTradePrice > 0 {
					cs.tickers[market].LastTradePrice = ticker.LastTradePrice
				}

				if ticker.PriceChangePercent != nil {
					percentChange := *ticker.PriceChangePercent
					cs.tickers[market].PriceChangePercent = &percentChange
				}

				cs.tickers[market].lastUpdate = time.Now()
				cs.mtx.Unlock()

				cs.wsUpdated()
			}

			cs.notifyRateListeners(false)
		}
	}()
}

// wsSendJSON is like wsSend but it encodes msg to JSON before sending.
func (cs *CommonRateSource) wsSendJSON(msg interface{}) error {
	ws, _ := cs.websocket()
	if ws == nil || !ws.On() {
		return errors.New("no connection") // never happens but..
	}
	return ws.Write(msg)
}

// Checks whether the websocketFeed Done channel is closed.
func (cs *CommonRateSource) wsListening() bool {
	cs.wsMtx.RLock()
	defer cs.wsMtx.RUnlock()
	return cs.wsSync.lastUpdate.After(cs.wsSync.fail) && cs.ws != nil && cs.ws.On()
}

// Set the updated flag. Set the error count to 0 when the client has
// successfully updated.
func (cs *CommonRateSource) wsUpdated() {
	now := time.Now()
	cs.wsMtx.Lock()
	cs.wsSync.lastUpdate = now
	cs.wsSync.errCount = 0
	cs.wsMtx.Unlock()
	cs.ratesUpdated(now)
}

func (cs *CommonRateSource) wsLastUpdate() time.Time {
	cs.wsMtx.RLock()
	defer cs.wsMtx.RUnlock()
	return cs.wsSync.lastUpdate
}

// Log the error and time, and increment the error counter.
func (cs *CommonRateSource) setWsFail(err error) {
	cs.fail("Websocket error", err)
	cs.wsMtx.Lock()
	defer cs.wsMtx.Unlock()
	if cs.ws != nil {
		cs.ws.Close()
		// Clear the field to prevent double Close'ing.
		cs.ws = nil
	}
	cs.wsSync.errCount++
	cs.wsSync.fail = time.Now()
}

func (cs *CommonRateSource) wsFailTime() time.Time {
	cs.wsMtx.RLock()
	defer cs.wsMtx.RUnlock()
	return cs.wsSync.fail
}

// Checks whether the websocket is in a failed state.
func (cs *CommonRateSource) wsFailed() bool {
	cs.wsMtx.RLock()
	defer cs.wsMtx.RUnlock()
	return cs.wsSync.fail.After(cs.wsSync.lastUpdate)
}

// The count of errors logged since the last success-triggered reset.
func (cs *CommonRateSource) wsErrorCount() int {
	cs.wsMtx.RLock()
	defer cs.wsMtx.RUnlock()
	return cs.wsSync.errCount
}

func (cs *CommonRateSource) copyRates() map[string]*Ticker {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	tickers := make(map[string]*Ticker, len(cs.tickers))
	for m, t := range cs.tickers {
		tickerCopy := *t
		tickers[m] = &tickerCopy
	}
	return tickers
}

// notifyRateListeners will send a rate notification to all listeners if
// rateNotificationInterval is due. Set force to true to ignore
// rateNotificationInterval.
func (cs *CommonRateSource) notifyRateListeners(force bool) {
	cs.rateListenersMtx.RLock()
	lastNotified := cs.lastNotified
	cs.rateListenersMtx.RUnlock()

	now := time.Now()
	if now.Sub(lastNotified) < rateNotificationInterval && !force {
		return
	}

	// Update cs.lastNotified.
	cs.rateListenersMtx.Lock()
	cs.lastNotified = now
	cs.rateListenersMtx.Unlock()

	// Notify all listeners.
	cs.rateListenersMtx.RLock()
	for _, l := range cs.rateListeners {
		l.Notify()
	}
	cs.rateListenersMtx.RUnlock()
}

// Refresh refreshes all expired rates and reconnects the rates websocket if it
// was previously disconnect. This method takes some time to refresh the rates
// and should be executed a a goroutine.
func (cs *CommonRateSource) Refresh(force bool) {
	if cs.source == none || cs.isDisabled() {
		return
	}

	// Block until previous refresh is done.
	cs.mtx.Lock()
	for cs.refreshing {
		cs.cond.Wait()
	}
	cs.refreshing = true
	cs.mtx.Unlock()

	defer func() {
		cs.mtx.Lock()
		cs.refreshing = false
		cs.cond.Signal()
		cs.mtx.Unlock()
	}()

	defer cs.ratesUpdated(time.Now())
	defer cs.notifyRateListeners(true)

	tickers := make(map[string]*Ticker)
	if !force {
		tickers = cs.copyRates()
	}

	for market := range supportedMarkets {
		t, ok := tickers[market]
		if ok && time.Since(t.lastUpdate) < rateExpiry {
			continue
		}

		ticker, err := cs.getTicker(market)
		if err != nil {
			cs.fail("Error fetching ticker", err)
			continue
		}

		tickers[market] = ticker
	}

	cs.mtx.Lock()
	cs.tickers = tickers
	cs.mtx.Unlock()

	// Check if the websocket connection is still on.
	if cs.wsListening() {
		return
	}

	if !cs.wsFailed() {
		// Connection has not been initialized.
		log.Tracef("Initializing websocket connection for %s", cs.source)
		err := cs.connectWebsocket()
		if err != nil {
			cs.fail("Error connecting websocket", err)
		}
		return
	}

	errCount := cs.wsErrorCount()
	var delay time.Duration
	var wsStarting bool
	switch {
	case errCount < 5:
	case errCount < 20:
		delay = 10 * time.Minute
	default:
		delay = time.Minute * 60
	}
	okToTry := cs.wsFailTime().Add(delay)
	if time.Now().After(okToTry) {
		wsStarting = true
		err := cs.connectWebsocket()
		if err != nil {
			cs.fail("Error connecting websocket", err)
		}
	} else {
		log.Errorf("%s websocket disabled. Too many errors. Refresh after %.1f minutes", cs.source, time.Until(okToTry).Minutes())
	}

	if !wsStarting {
		sinceLast := time.Since(cs.wsLastUpdate())
		log.Tracef("Last %s websocket update %.3f seconds ago", sinceLast.Seconds(), cs.source)
		if sinceLast > RateRefreshDuration && cs.wsFailed() {
			cs.setWsFail(fmt.Errorf("Lost connection detected. %s websocket will restart during next refresh", cs.source))
		}
	}
}

// fetchRate retrieves new ticker information via the rate source's HTTP API.
func (cs *CommonRateSource) fetchRate(market string) *Ticker {
	newTicker, err := cs.getTicker(market)
	if err != nil {
		cs.fail("Error fetching ticker", err)
		return nil
	}

	cs.mtx.Lock()
	cs.tickers[market] = newTicker
	cs.mtx.Unlock()

	t := *newTicker
	return &t
}

// GetTicker retrieves ticker information for th provided market. Data will be
// retrieved from cache if its available and still valid.
func (cs *CommonRateSource) GetTicker(market string) *Ticker {
	marketName, ok := isSupportedMarket(market, cs.source)
	if !ok {
		return nil
	}

	cs.mtx.RLock()
	ticker, ok := cs.tickers[marketName]
	cs.mtx.RUnlock()
	if !ok {
		return cs.fetchRate(marketName)
	}
	t := *ticker

	if time.Since(t.lastUpdate) > rateExpiry {
		if ticker := cs.fetchRate(marketName); ticker != nil {
			return ticker
		}
	}

	return &t
}

// Used to initialize the embedding rate source.
func NewCommonRateSource(ctx context.Context, source string) (*CommonRateSource, error) {
	if source != binance && source != bittrex && source != none {
		return nil, fmt.Errorf("New rate source %s is not supported", source)
	}

	getTickerFunc := dummyGetTickerFunc
	wsProcessor := func([]byte) ([]*Ticker, error) { return nil, nil }
	switch source {
	case binance:
		getTickerFunc = binanceGetTicker
		wsProcessor = processBinanceWsMessage
	case bittrex:
		getTickerFunc = bittrexGetTicker
		wsProcessor = processBittrexWsMessage
	}

	s := &CommonRateSource{
		ctx:           ctx,
		source:        source,
		tickers:       make(map[string]*Ticker),
		getTicker:     getTickerFunc,
		wsProcessor:   wsProcessor,
		rateListeners: make(map[string]*RateListener),
		sourceChanged: make(chan *struct{}),
	}
	s.cond = sync.NewCond(&s.mtx)

	// Start shutdown goroutine.
	go func() {
		<-ctx.Done()
		ws, _ := s.websocket()
		if ws != nil {
			ws.Close()
		}
	}()

	return s, nil
}

func binanceGetTicker(market string) (*Ticker, error) {
	market = strings.ReplaceAll(market, MktSep, "")
	if _, ok := binanceMarkets[market]; !ok {
		return nil, fmt.Errorf("Market %s not supported", market)
	}

	reqCfg := &utils.ReqConfig{
		HTTPURL: fmt.Sprintf(binanceURLs.price, market),
		Method:  "GET",
	}

	resp := new(BinanceTickerResponse)
	_, err := utils.HTTPRequest(reqCfg, &resp)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker for %s: %w", binance, market, err)
	}

	percentChange := resp.PriceChangePercent
	ticker := &Ticker{
		Market:             market,
		LastTradePrice:     resp.LastPrice,
		PriceChangePercent: &percentChange,
		lastUpdate:         time.Now(),
	}

	return ticker, nil
}

func bittrexGetTicker(market string) (*Ticker, error) {
	reqCfg := &utils.ReqConfig{
		HTTPURL: fmt.Sprintf(bittrexURLs.price, market),
		Method:  "GET",
	}

	// Fetch current rate.
	resp := new(BittrexTickerResponse)
	_, err := utils.HTTPRequest(reqCfg, &resp)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker for %s: %w", bittrex, market, err)
	}

	ticker := &Ticker{
		Market:         resp.Symbol, // Ok: e.g BTC-USDT
		LastTradePrice: resp.LastTradeRate,
	}

	// Fetch percentage change.
	reqCfg.HTTPURL = fmt.Sprintf(bittrexURLs.stats, market)
	res := new(BittrexMarketSummaryResponse)
	_, err = utils.HTTPRequest(reqCfg, &res)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker for %s: %w", bittrex, market, err)
	}

	percentChange := res.PercentChange
	ticker.PriceChangePercent = &percentChange
	ticker.lastUpdate = time.Now()
	return ticker, nil
}

type binanceWsMsg struct {
	// Unmarshalling data.c is causing unexpected behavior and returning value
	// for data.C so we are manually accessing the map values we need.
	Data map[string]any `json:"data"`
}

func processBinanceWsMessage(inMsg []byte) ([]*Ticker, error) {
	msg := new(binanceWsMsg)
	err := json.Unmarshal(inMsg, &msg)
	if err != nil {
		return nil, fmt.Errorf("Binance: unable to read message bytes: %w", err)
	}

	if len(msg.Data) == 0 {
		return nil, nil // handled
	}

	methodName, ok := msg.Data["e"].(string)
	if !ok || !strings.Contains(methodName, "Ticker") {
		return nil, nil // handled
	}

	market, ok := msg.Data["s"].(string)
	if !ok {
		return nil, errors.New("unexpected type received as ticker market name")
	}

	priceChangePercentStr, ok := msg.Data["P"].(string)
	if !ok {
		return nil, errors.New("unexpected type received as ticker price change percent")
	}

	priceChangePercent, err := strconv.ParseFloat(priceChangePercentStr, 64)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat error: %w", err)
	}

	lastPriceStr, ok := msg.Data["c"].(string)
	if !ok {
		return nil, errors.New("unexpected type received as ticker last price")
	}

	lastPrice, err := strconv.ParseFloat(lastPriceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat error: %w", err)
	}

	ticker := &Ticker{
		Market:             market,
		LastTradePrice:     lastPrice,
		PriceChangePercent: &priceChangePercent,
		lastUpdate:         time.Now(),
	}

	return []*Ticker{ticker}, err
}

// processWsMessage handles message from the bittrex websocket. The message can
// be either a full orderbook at msg.R (msg.I == "1"), or a list of updates in
// msg.M[i].A.
func processBittrexWsMessage(inMsg []byte) ([]*Ticker, error) {
	// Ignore KeepAlive messages.
	if len(inMsg) == 2 && inMsg[0] == '{' && inMsg[1] == '}' {
		return nil, nil // handled
	}

	var msg signalRMessage
	err := json.Unmarshal(inMsg, &msg)
	if err != nil {
		return nil, fmt.Errorf("unable to read message bytes: %w", err)
	}

	msgs, err := decodeBittrexWSMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("websocket message decode error: %w", err)
	}

	if len(msgs) == 0 {
		return nil, nil // handled
	}

	var tickers []*Ticker
	for _, msgData := range msgs {
		ticker := &Ticker{}
		switch d := msgData.(type) {
		case *BittrexMarketSummaryResponse:
			ticker.Market = d.Symbol // Ok: e.g BTC-USDT
			percent := d.PercentChange
			ticker.PriceChangePercent = &percent
		case *BittrexTickerResponse:
			ticker.Market = d.Symbol // Ok: e.g BTC-USDT
			ticker.LastTradePrice = d.LastTradeRate
		default:
			return nil, fmt.Errorf("received unexpected message type %T from decodeBittrexWSMessage", d)
		}

		ticker.lastUpdate = time.Now()
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func decodeBittrexWSMessage(msg signalRMessage) ([]any, error) {
	if len(msg.M) == 0 {
		return nil, nil // handled
	}

	var msgs []any
	for i := range msg.M {
		msgInfo := msg.M[i]
		name := msgInfo.M
		if name == BittrexMsgHeartbeat {
			return nil, nil // handled
		}

		isSummary := name == BittrexMarketSummary
		isTicker := name == BittrexTicker
		if !isSummary && !isTicker {
			return nil, fmt.Errorf("unknown message type %q: %+v", name, msgInfo)
		}

		msgStr := msgInfo.A[0]
		s, ok := msgStr.(string)
		if !ok {
			return nil, errors.New("message not a string")
		}

		data, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("base64 error: %w", err)
		}

		buf := bytes.NewBuffer(data)
		zr := flate.NewReader(buf)
		defer zr.Close()

		var b bytes.Buffer
		if _, err := io.Copy(&b, zr); err != nil {
			return nil, fmt.Errorf("copy error: %w", err)
		}

		var msgData any
		switch name {
		case BittrexTicker:
			msgData = new(BittrexTickerResponse)
			err = json.Unmarshal(b.Bytes(), &msgData)
		case BittrexMarketSummary:
			msgData = new(BittrexMarketSummaryResponse)
			err = json.Unmarshal(b.Bytes(), &msgData)
		}
		if err != nil {
			return nil, err
		}

		msgs = append(msgs, msgData)
	}

	return msgs, nil
}

// isSupportedMarket returns a proper market name for the provided rate source
// and returns false if the market is not supported.
func isSupportedMarket(market, rateSource string) (string, bool) {
	if rateSource == none {
		return "", false
	}

	market = strings.ToTitle(market)
	currencies := strings.Split(market, MktSep)
	if len(currencies) != 2 {
		return "", false
	}

	fromCur, toCur := currencies[0], currencies[1]
	btcToOthersExceptUsdt := strings.EqualFold(fromCur, "btc") && !strings.EqualFold(toCur, "usdt")
	if btcToOthersExceptUsdt {
		fromCur, toCur = toCur, fromCur // e.g DCR/BTC, LTC/BTC and not BTC/LTC or BTC/DCR
	}

	marketName := fromCur + MktSep + toCur
	_, ok := supportedMarkets[marketName]
	if !ok {
		return "", false
	}

	return marketName, true
}

func dummyGetTickerFunc(string) (*Ticker, error) {
	return &Ticker{}, nil
}
