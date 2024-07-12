// Copyright (c) 2019-2021, The Decred developers
// Copyright (c) 2023, The Cryptopower developers
// See LICENSE for details.

package ext

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	// These are constants used to represent various rate sources supported.
	binance        = values.BinanceExchange
	binanceUS      = values.BinanceUSExchange
	coinpaprika    = values.Coinpaprika
	messari        = values.Messari
	kucoinExchange = values.KucoinExchange
	none           = values.DefaultExchangeValue

	// MktSep is used repo wide to separate market symbols.
	MktSep = "-"
)

var (
	// According to the docs (See:
	// https://www.binance.com/en/support/faq/frequently-asked-questions-on-api-360004492232),
	// there's a 6,000 request weight per minute (keep in mind that this is not
	// necessarily the same as 6,000 requests) limit for API requests. Multiple
	// tickers are quested in a single call every 5min. We can never get in
	// trouble for this. An HTTP 403 is returned for those that violates this
	// hard rule. More information on limits can be found here:
	// https://binance-docs.github.io/apidocs/spot/en/#limits
	binanceURLs = sourceURLs{
		// See: https://binance-docs.github.io/apidocs/spot/en/#current-average-price
		price: "https://api.binance.com/api/v3/ticker/24hr?symbol=%s",
	}

	binanceUSURLs = sourceURLs{
		// See: https://binance-docs.github.io/apidocs/spot/en/#current-average-price
		price: "https://api.binance.us/api/v3/ticker/24hr?symbol=%s",
	}

	// According to the docs (See:
	// https://api.coinpaprika.com/#section/Rate-limit), the free version is
	// eligible to 20,000 calls per month. All tickers are fetched in one call,
	// that means we only exhaust 288 calls per day and 8928 calls per month if
	// we request rate every 5min. Max of 2000 asset data returned and API is
	// updated every 5min.
	coinpaprikaURLs = sourceURLs{
		price: "https://api.coinpaprika.com/v1/tickers",
	}

	// According to the x-ratelimit-limit header, we can make 4000 requests
	// every 24hours. The x-ratelimit-reset header tells when the next reset
	// will be. See: Header values for
	// https://data.messari.io/api/v1/assets/DCR/metrics/market-data. From a
	// previous research by buck, say "Without an API key requests are rate
	// limited to 20 requests per minute". That means we are limited to 20
	// requests for tickers per minute but with with a 10min refresh interval,
	// we'd only exhaust 2880 call assuming we are fetching data for 20 tickers
	// (assets supported by dex are still below 20, revisit if we implement up
	// to 20 assets).
	messariURLs = sourceURLs{
		price: "https://data.messari.io/api/v1/assets/%s/metrics/market-data",
	}

	// According to the gw-ratelimit-limit header, we can make 2000 requests
	// every 24hours(I think there's only a gw-ratelimit-reset header set to
	// 30000 but can't decipher if it's in seconds or minutes). Multiple tickers
	// can be requested in a single call (Firo and ZCL not supported). See
	// Header values for
	// https://api.kucoin.com/api/v1/prices?currencies=BTC,DCR. Requesting for
	// ticker data every 5min gives us 288 calls per day, with the remaining
	// 1712 calls left unused.

	kucoinURLs = sourceURLs{
		price: "https://api.kucoin.com/api/v1/prices?currencies=%s",   // symbol is asset like BTC
		stats: "https://api.kucoin.com/api/v1/market/stats?symbol=%s", // symbol like BTC-USDT
	}

	// supportedMarkets is a map of markets supported by rate sources
	// implemented (Binance, Bittrex).
	supportedMarkets = map[values.Market]*struct{}{
		values.BTCUSDTMarket: {},
		values.DCRUSDTMarket: {},
		values.LTCUSDTMarket: {},
	}

	// Rates exceeding rateExpiry are expired and should be removed unless there
	// was an error fetching a new rate.
	rateExpiry = 30 * time.Minute

	// Rate sources should be refreshed every RateRefreshDuration to replace
	// expired rates and reconnect websocket if need be.
	RateRefreshDuration = 60 * time.Minute
)

// RateSource is the interface that binds different rate sources. Most of the
// methods are implemented by CommonRateSource, but Refresh is implemented in
// the individual rate source.
type RateSource interface {
	Name() string
	Ready() bool
	Refresh(force bool)
	Refreshing() bool
	LastUpdate() time.Time
	GetTicker(market values.Market, cacheOnly bool) *Ticker
	ToggleStatus(disable bool)
	ToggleSource(newSource string) error
}

// RateListener listens for new tickers and rate source change notifications.
type RateListener struct {
	OnRateUpdated func()
}

type tickerFunc func(market values.Market) (*Ticker, error)

// CommonRateSource is an external rate source for fiat and crypto-currency
// rates. These rates are estimates and maybe be affected by server latency and
// should not be used for actual buy or sell orders except to display reasonable
// estimates. CommonRateSource is embedded in all of the rate sources supported.
type CommonRateSource struct {
	ctx           context.Context
	source        string
	disabled      bool
	mtx           sync.RWMutex
	tickers       map[values.Market]*Ticker
	refreshing    bool
	cond          *sync.Cond
	getTicker     tickerFunc
	sourceChanged chan *struct{}
	lastUpdate    time.Time

	disableConversionExchange func()
}

// Used to initialize a rate source.
func NewCommonRateSource(ctx context.Context, source string, disableConversionExchange func()) (*CommonRateSource, error) {
	if !isValidSource(source) {
		return nil, fmt.Errorf("new rate source %s is not supported", source)
	}

	s := &CommonRateSource{
		ctx:                       ctx,
		source:                    source,
		tickers:                   make(map[values.Market]*Ticker),
		sourceChanged:             make(chan *struct{}),
		disableConversionExchange: disableConversionExchange,
	}
	s.getTicker = s.sourceGetTickerFunc(source)
	s.cond = sync.NewCond(&s.mtx)

	return s, nil
}

// Name is the string associated with the rate source for display.
func (cs *CommonRateSource) Name() string {
	return cs.source
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

func (cs *CommonRateSource) ToggleStatus(disable bool) {
	if cs.isDisabled() != disable {
		return
	}

	cs.mtx.Lock()
	cs.disabled = disable
	cs.mtx.Unlock()
}

func (cs *CommonRateSource) ratesUpdated(t time.Time) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.lastUpdate = t
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
	refresh := true
	if newSource == none {
		refresh = false /* none is the dummy rate source for when user disables rates */
	}

	getTickerFn := cs.sourceGetTickerFunc(newSource)
	if getTickerFn == nil {
		return fmt.Errorf("new rate source %s is not supported", newSource)
	}

	// Update source specific fields.
	cs.mtx.Lock()
	cs.source = newSource
	cs.getTicker = getTickerFn
	cs.tickers = make(map[values.Market]*Ticker)
	cs.mtx.Unlock()

	if refresh {
		cs.Refresh(true)
	}

	return nil
}

// Log the error along with the token and an additional passed identifier.
func (cs *CommonRateSource) fail(msg string, err error) {
	log.Errorf("%s: %s: %v", cs.source, msg, err)
}

func (cs *CommonRateSource) copyRates() map[values.Market]*Ticker {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	tickers := make(map[values.Market]*Ticker, len(cs.tickers))
	for m, t := range cs.tickers {
		tickerCopy := *t
		tickers[m] = &tickerCopy
	}
	return tickers
}

// Refresh refreshes all expired rates if it
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

	tickers := make(map[values.Market]*Ticker)
	if !force {
		tickers = cs.copyRates()
	}

	for market := range supportedMarkets {
		t, ok := tickers[market]
		if ok && time.Since(t.lastUpdate) < rateExpiry {
			continue
		}

		ticker, err := cs.retryGetTicker(market)
		if err != nil {
			cs.fail("Error fetching ticker", err)
			continue
		}

		tickers[market] = ticker
	}

	cs.mtx.Lock()
	cs.tickers = tickers
	cs.mtx.Unlock()
}

// GetTicker retrieves ticker information for the provided market. Data will be
// retrieved from cache if its available and still valid. Returns nil if valid,
// cached isn't available and cacheOnly is true. If cacheOnly is false and no
// valid, cached data is available, a network call will be made to fetch the
// latest ticker information and update the cache.
func (cs *CommonRateSource) GetTicker(market values.Market, cacheOnly bool) *Ticker {
	marketName, ok := isSupportedMarket(market, cs.source)
	if !ok {
		return nil
	}

	cs.mtx.RLock()
	ticker, ok := cs.tickers[marketName]
	cs.mtx.RUnlock()

	// Get rate if market ticker does not exist
	if !ok {
		if cacheOnly {
			return nil
		}
		return cs.fetchRate(marketName)
	}

	t := *ticker
	if !cacheOnly && time.Since(t.lastUpdate) > rateExpiry {
		if ticker := cs.fetchRate(marketName); ticker != nil {
			return ticker
		}
	}

	return &t
}

// fetchRate retrieves new ticker information via the rate source's HTTP API.
func (cs *CommonRateSource) fetchRate(market values.Market) *Ticker {
	newTicker, err := cs.retryGetTicker(market)
	if err != nil {
		cs.fail("Error fetching ticker", err)
		return nil
	}

	cs.mtx.Lock()
	cs.tickers[market] = newTicker
	cs.mtx.Unlock()

	return newTicker
}

func (cs *CommonRateSource) retryGetTicker(market values.Market) (*Ticker, error) {
	var newTicker *Ticker
	var err error
	select {
	case <-cs.ctx.Done():
		log.Errorf("fetching ticker canceled: %v", cs.ctx.Err())
		return nil, cs.ctx.Err()
	default:
		log.Infof("fetching %s rate from %v", market, cs.source)
		newTicker, err = cs.getTicker(market)
		if err == nil {
			return newTicker, nil
		}
	}
	// fetch ticker from available exchanges
	log.Infof("fetching from other exchanges")
	for _, source := range sources {
		if source == cs.source {
			continue
		}
		getTickerFn := cs.sourceGetTickerFunc(source)
		select {
		case <-cs.ctx.Done():
			log.Errorf("fetching ticker canceled: %v", cs.ctx.Err())
			return nil, cs.ctx.Err()
		default:
			log.Infof("fetching %s rate from %v", market, source)
			newTicker, err = getTickerFn(market)
			if err == nil {
				log.Infof("%s is chosen", source)
				cs.source = source
				return newTicker, nil
			}
		}
	}
	if cs.disableConversionExchange != nil {
		cs.disableConversionExchange()
	}
	return nil, err
}

func (cs *CommonRateSource) binanceGetTicker(market values.Market) (*Ticker, error) {
	reqCfg := &utils.ReqConfig{
		HTTPURL: fmt.Sprintf(binanceURLs.price, market.MarketWithoutSep()),
		Method:  "GET",
	}
	if cs.source == binanceUS {
		reqCfg.HTTPURL = fmt.Sprintf(binanceUSURLs.price, market.MarketWithoutSep())
	}

	resp := new(BinanceTickerResponse)
	_, err := utils.HTTPRequest(reqCfg, &resp)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker for %s: %w", cs.source, market, err)
	}

	percentChange := resp.PriceChangePercent
	ticker := &Ticker{
		Market:             market.String(),
		LastTradePrice:     resp.LastPrice,
		PriceChangePercent: &percentChange,
		lastUpdate:         time.Now(),
	}

	return ticker, nil
}

// coinpaprikaGetTicker don't need market param, but need it to satisfy the format of the getrate function
func (cs *CommonRateSource) coinpaprikaGetTicker(market values.Market) (*Ticker, error) {
	reqCfg := &utils.ReqConfig{
		HTTPURL: coinpaprikaURLs.price,
		Method:  "GET",
	}

	var res []*struct {
		Symbol string `json:"symbol"`
		Quotes struct {
			USD struct {
				Price         float64 `json:"price"`
				PercentChange float64 `json:"percent_change_24h"`
			} `json:"USD"`
		} `json:"quotes"`
	}
	_, err := utils.HTTPRequest(reqCfg, &res)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker: %w", coinpaprika, err)
	}

	cs.mtx.Lock()
	for _, coinInfo := range res {
		market := values.NewMarket(coinInfo.Symbol, "USDT")
		_, found := supportedMarkets[market]
		if !found {
			continue
		}

		price := coinInfo.Quotes.USD.Price
		if price == 0 {
			log.Errorf("zero-price returned from coinpaprika for asset with ticker %s", coinInfo.Symbol)
			continue
		}
		ticker := &Ticker{
			Market:             market.String(), // Ok: e.g BTC-USDT
			LastTradePrice:     price,
			lastUpdate:         time.Now(),
			PriceChangePercent: &coinInfo.Quotes.USD.PercentChange,
		}
		cs.tickers[market] = ticker
	}
	cs.mtx.Unlock()

	return cs.tickers[market], nil
}

func messariGetTicker(market values.Market) (*Ticker, error) {
	reqCfg := &utils.ReqConfig{
		HTTPURL: fmt.Sprintf(messariURLs.price, market.AssetString()),
		Method:  "GET",
	}
	var res struct {
		Data struct {
			MarketData struct {
				Price         float64 `json:"price_usd"`
				PercentChange float64 `json:"percent_change_usd_last_24_hours"`
			} `json:"market_data"`
		} `json:"data"`
	}

	_, err := utils.HTTPRequest(reqCfg, &res)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker for %s: %w", messari, market, err)
	}

	ticker := &Ticker{
		Market:             market.String(), // Ok: e.g BTC-USDT
		LastTradePrice:     res.Data.MarketData.Price,
		lastUpdate:         time.Now(),
		PriceChangePercent: &res.Data.MarketData.PercentChange,
	}

	return ticker, nil
}

func kucoinGetTicker(market values.Market) (*Ticker, error) {
	reqCfg := &utils.ReqConfig{
		HTTPURL: fmt.Sprintf(kucoinURLs.price, market.AssetString()),
		Method:  "GET",
	}
	var res struct {
		Data map[string]string `json:"data"`
	}

	_, err := utils.HTTPRequest(reqCfg, &res)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch ticker for %s: %w", kucoinExchange, market, err)
	}

	rate, err := strconv.ParseFloat(res.Data[market.AssetString()], 64)
	if err != nil {
		return nil, fmt.Errorf("kucoin: failed to convert fiat rate for %s to float64: %v", market.String(), err)
	}

	// Get Stats
	reqStatsCfg := &utils.ReqConfig{
		HTTPURL: fmt.Sprintf(kucoinURLs.stats, market.String()),
		Method:  "GET",
	}
	var statsRes struct {
		Data struct {
			ChangeRate string `json:"changeRate"`
		} `json:"data"`
	}

	_, err = utils.HTTPRequest(reqStatsCfg, &statsRes)
	if err != nil {
		return nil, fmt.Errorf("%s failed to fetch stat for %s: %w", kucoinExchange, market, err)
	}

	changeRate, err := strconv.ParseFloat(statsRes.Data.ChangeRate, 64)
	if err != nil {
		return nil, fmt.Errorf("kucoin: failed to convert fiat changeRate to float64: %v", err)
	}

	ticker := &Ticker{
		Market:             market.String(), // Ok: e.g BTC-USDT
		LastTradePrice:     rate,
		lastUpdate:         time.Now(),
		PriceChangePercent: &changeRate,
	}

	return ticker, nil
}

// isSupportedMarket returns a proper market name for the provided rate source
// and returns false if the market is not supported.
func isSupportedMarket(market values.Market, rateSource string) (values.Market, bool) {
	if rateSource == none {
		return "", false
	}

	marketStr := strings.ToTitle(market.String())
	currencies := strings.Split(marketStr, MktSep)
	if len(currencies) != 2 {
		return "", false
	}

	fromCur, toCur := currencies[0], currencies[1]
	btcToOthersExceptUsdt := strings.EqualFold(fromCur, "btc") && !strings.EqualFold(toCur, "usdt")
	if btcToOthersExceptUsdt {
		fromCur, toCur = toCur, fromCur // e.g DCR/BTC, LTC/BTC and not BTC/LTC or BTC/DCR
	}

	marketName := values.NewMarket(fromCur, toCur)
	_, ok := supportedMarkets[marketName]
	if !ok {
		return "", false
	}

	return marketName, true
}

func isValidSource(source string) bool {
	switch source {
	case binance, binanceUS, coinpaprika, messari, kucoinExchange, none:
		return true
	default:
		return false
	}
}

func (cs *CommonRateSource) sourceGetTickerFunc(source string) func(values.Market) (*Ticker, error) {
	switch source {
	case binance, binanceUS:
		return cs.binanceGetTicker
	case messari:
		return messariGetTicker
	case kucoinExchange:
		return kucoinGetTicker
	case coinpaprika:
		return cs.coinpaprikaGetTicker
	case none:
		return dummyGetTickerFunc
	default:
		return nil
	}
}

var sources = []string{
	binance,
	binanceUS,
	messari,
	kucoinExchange,
	coinpaprika,
	none,
}

func dummyGetTickerFunc(values.Market) (*Ticker, error) {
	return &Ticker{}, nil
}
