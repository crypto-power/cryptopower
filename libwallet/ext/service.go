package ext

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/chaincfg/v3"
	chainjson "github.com/decred/dcrd/rpc/jsonrpc/types/v4"
	apiTypes "github.com/decred/dcrdata/v8/api/types"
)

type (
	// Service provide functionality for retrieving data from
	// 3rd party services or external resources.
	Service struct {
		chainParams *chaincfg.Params
	}
)

const (
	// Bittrex is a string constant that identifies the bittrex backend service provider within this package.
	// It Should be used when calling bittrex specific functions from external application/library
	// e.g Service.GetTicker(api.Bittrex, "btc-usdt")
	// All backend service providers constant defined below should be used in similar fashion.
	Bittrex                  = "bittrex"
	Binance                  = "binance"
	BlockBook                = "blockbook"
	DcrData                  = "dcrdata"
	KuCoin                   = "kucoin"
	testnetAddressIndetifier = "T"
	mainnetAddressIdentifier = "D"
	mainnetXpubIdentifier    = "d"
	testnetXpubIdentifier    = "t"
)

var (
	// mainnetURL maps supported backends to their current mainnet url scheme and authority.
	mainnetURL = map[string]string{
		Bittrex:   "https://api.bittrex.com/v3",
		Binance:   "https://api.binance.com",
		BlockBook: "https://blockbook.decred.org:9161/",
		DcrData:   "https://mainnet.dcrdata.org/",
		KuCoin:    "https://api.kucoin.com",
	}
	// testnetURL maps supported backends to their current testnet url scheme and authority.
	testnetURL = map[string]string{
		Binance:   "https://testnet.binance.vision",
		BlockBook: "https://blockbook.decred.org:19161/",
		DcrData:   "https://testnet.dcrdata.org/",
		KuCoin:    "https://openapi-sandbox.kucoin.com",
	}
	backendURL = map[string]map[string]string{
		chaincfg.MainNetParams().Name:  mainnetURL,
		chaincfg.TestNet3Params().Name: testnetURL,
	}
)

// NewService configures and return a news instance of the service type.
func NewService(chainParams *chaincfg.Params) *Service {
	return &Service{
		chainParams: chainParams,
	}
}

// Setbackend sets the appropriate URL scheme and authority for the backend resource.
func setBackend(backend, net, rawURL string) string {
	// Check if URL scheme and authority is already set.
	if strings.HasPrefix(rawURL, "http") {
		return rawURL
	}

	// Prepend URL scheme and authority to the URL.
	if authority, ok := backendURL[net][backend]; ok {
		rawURL = fmt.Sprintf("%s%s", authority, rawURL)
	}
	return rawURL
}

// GetBestBlock returns the best block height as int32.
func (s *Service) GetBestBlock() int32 {
	reqConf := &utils.ReqConfig{
		Method:    http.MethodGet,
		HTTPURL:   setBackend(DcrData, s.chainParams.Name, "api/block/best/height"),
		IsRetByte: true,
	}

	var resp []byte
	_, err := utils.HTTPRequest(reqConf, &resp)
	if err != nil {
		log.Error(err)
		return -1
	}

	h, err := strconv.ParseInt(string(resp), 10, 32)
	if err != nil {
		log.Error(err)
		return -1
	}

	return int32(h)
}

// GetBestBlockTimeStamp returns best block time, as unix timestamp.
func (s *Service) GetBestBlockTimeStamp() int64 {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/block/best?txtotals=false"),
	}

	resp := &BlockDataBasic{}
	_, err := utils.HTTPRequest(reqConf, resp)
	if err != nil {
		log.Error(err)
		return -1
	}
	return resp.Time.UNIX()
}

// GetCurrentAgendaStatus returns the current agenda and its status.
func (s *Service) GetCurrentAgendaStatus() (agenda *chainjson.GetVoteInfoResult, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/stake/vote/info"),
	}
	agenda = &chainjson.GetVoteInfoResult{}
	_, err = utils.HTTPRequest(reqConf, agenda)
	return agenda, err
}

// GetAgendas returns all agendas high level details
func (s *Service) GetAgendas() (agendas *[]apiTypes.AgendasInfo, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/agendas"),
	}
	agendas = &[]apiTypes.AgendasInfo{}
	_, err = utils.HTTPRequest(reqConf, agendas)
	return agendas, err
}

// GetAgendaDetails returns the details for agenda with agendaId
func (s *Service) GetAgendaDetails(agendaID string) (agendaDetails *AgendaAPIResponse, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/agenda/"+agendaID),
	}
	agendaDetails = &AgendaAPIResponse{}
	_, err = utils.HTTPRequest(reqConf, agendaDetails)
	return agendaDetails, err
}

// GetTreasuryBalance returns the current treasury balance as int64.
func (s *Service) GetTreasuryBalance() (bal int64, err error) {
	treasury, err := s.GetTreasuryDetails()
	if err != nil {
		return bal, err
	}
	return treasury.Balance, err
}

// GetTreasuryDetails the current tresury balance, spent amount, added amount, and tx count for the
// treasury.
func (s *Service) GetTreasuryDetails() (treasuryDetails *TreasuryDetails, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/treasury/balance"),
	}
	treasuryDetails = &TreasuryDetails{}
	_, err = utils.HTTPRequest(reqConf, treasuryDetails)
	return treasuryDetails, err
}

// GetExchangeRate fetches exchange rate data summary
func (s *Service) GetExchangeRate() (rates *ExchangeRates, err error) {
	reqConf := &utils.ReqConfig{
		Method: http.MethodGet,
		// Use mainnet base url for exchange rate endpoint, there is no Dcrdata
		// support for testnet ExchangeRate.
		HTTPURL: setBackend(DcrData, chaincfg.MainNetParams().Name, "api/exchangerate"),
	}
	rates = &ExchangeRates{}
	_, err = utils.HTTPRequest(reqConf, rates)
	return rates, err
}

// GetExchanges fetches the current known state of all exchanges
func (s *Service) GetExchanges() (state *ExchangeState, err error) {
	reqConf := &utils.ReqConfig{
		Method: http.MethodGet,
		// Use mainnet base url for exchanges endpoint, no Dcrdata support for Exchanges
		// on testnet.
		HTTPURL: setBackend(DcrData, chaincfg.MainNetParams().Name, "api/exchanges"),
	}
	state = &ExchangeState{}
	_, err = utils.HTTPRequest(reqConf, state)
	return state, err
}

// GetTicketFeeRateSummary returns the current ticket fee rate summary. See dcrdata's MempoolTicketFeeInfo for the specific
// data returned.
func (s *Service) GetTicketFeeRateSummary() (ticketInfo *apiTypes.MempoolTicketFeeInfo, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/mempool/sstx"),
	}
	ticketInfo = &apiTypes.MempoolTicketFeeInfo{}
	_, err = utils.HTTPRequest(reqConf, ticketInfo)
	return ticketInfo, err
}

// GetTicketFeeRate returns top 25 ticket fees. Note: in cases where n < 25 and n == number of all ticket fees,
// It returns n.
func (s *Service) GetTicketFeeRate() (ticketFeeRate *apiTypes.MempoolTicketFees, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/mempool/sstx/fees"),
	}
	ticketFeeRate = &apiTypes.MempoolTicketFees{}
	_, err = utils.HTTPRequest(reqConf, ticketFeeRate)
	return ticketFeeRate, err
}

// GetNHighestTicketFeeRate returns the {nHighest} ticket fees. For cases where total number of ticker is less than
// {nHighest} it returns the fee rate for the total number of tickets.
func (s *Service) GetNHighestTicketFeeRate(nHighest int) (ticketFeeRate *apiTypes.MempoolTicketFees, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/mempool/sstx/fees/"+strconv.Itoa(nHighest)),
	}
	ticketFeeRate = &apiTypes.MempoolTicketFees{}
	_, err = utils.HTTPRequest(reqConf, ticketFeeRate)
	return ticketFeeRate, err
}

// GetTicketDetails returns all ticket details see drcdata's MempoolTicketDetails for the spcific information
// returned.
func (s *Service) GetTicketDetails() (ticketDetails *apiTypes.MempoolTicketDetails, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/mempool/sstx/details"),
	}
	ticketDetails = &apiTypes.MempoolTicketDetails{}
	_, err = utils.HTTPRequest(reqConf, ticketDetails)
	return ticketDetails, err
}

// GetNHighestTicketDetails returns the {nHighest} ticket details.
func (s *Service) GetNHighestTicketDetails(nHighest int) (ticketDetails *apiTypes.MempoolTicketDetails, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(DcrData, s.chainParams.Name, "api/mempool/sstx/details/"+strconv.Itoa(nHighest)),
	}
	ticketDetails = &apiTypes.MempoolTicketDetails{}
	_, err = utils.HTTPRequest(reqConf, ticketDetails)
	return ticketDetails, err
}

// GetAddress returns the balances and transactions of an address.
// The returned transactions are sorted by block height, newest blocks first.
func (s *Service) GetAddress(address string) (addressState *AddressState, err error) {
	if address == "" {
		err = errors.New("address can't be empty")
		return
	}

	// on testnet, address prefix - first byte - should match testnet identifier
	if s.chainParams.Name == chaincfg.TestNet3Params().Name && address[:1] != testnetAddressIndetifier {
		return nil, errors.New("net is testnet3 and xpub is not in testnet format")
	}

	// on mainnet, address prefix - first byte - should match mainnet identifier
	if s.chainParams.Name == chaincfg.MainNetParams().Name && address[:1] != mainnetAddressIdentifier {
		return nil, errors.New("net is mainnet and xpub is not in mainnet format")
	}

	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(BlockBook, s.chainParams.Name, "api/v2/address/"+address),
	}
	addressState = &AddressState{}
	_, err = utils.HTTPRequest(reqConf, addressState)
	return addressState, err
}

// GetXpub Returns balances and transactions of an xpub.
func (s *Service) GetXpub(xPub string) (xPubBalAndTxs *XpubBalAndTxs, err error) {
	if xPub == "" {
		return nil, errors.New("empty xpub string")
	}

	// on testnet Xpub prefix - first byte - should match testnet identifier
	if s.chainParams.Name == chaincfg.TestNet3Params().Name && xPub[:1] != testnetXpubIdentifier {
		return nil, errors.New("net is testnet3 and xpub is not in testnet format")
	}

	// on mainnet xpup prefix - first byte - should match mainnet identifier
	if s.chainParams.Name == chaincfg.MainNetParams().Name && xPub[:1] != mainnetXpubIdentifier {
		return nil, errors.New("net is mainnet and xpub is not in mainnet format")
	}

	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(BlockBook, s.chainParams.Name, "api/v2/xpub/"+xPub),
	}
	xPubBalAndTxs = &XpubBalAndTxs{}
	_, err = utils.HTTPRequest(reqConf, xPubBalAndTxs)
	return xPubBalAndTxs, err
}

// GetTicker returns market ticker data for the supported exchanges.
// Current supported exchanges: bittrex, binance and kucoin. This endpoint will query mainnet
// resource irrespective.
func (s *Service) GetTicker(exchange string, market string) (ticker *Ticker, err error) {
	switch exchange {
	case Binance:
		// Return early for dcr-ltc markets as binance does not support this
		// atm.
		if strings.EqualFold(market, "ltc-dcr") {
			return &Ticker{}, nil
		}

		symbArr := strings.Split(market, "-")
		if len(symbArr) != 2 {
			return ticker, errors.New("invalid symbol format")
		}
		symb := strings.Join(symbArr[:], "")
		return s.getBinanceTicker(symb)
	case Bittrex:
		return s.getBittrexTicker(market)
	case KuCoin:
		return s.getKucoinTicker(market)
	}

	return nil, errors.New("unknown exchange")
}

func (s *Service) getBinanceTicker(market string) (ticker *Ticker, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(Binance, s.chainParams.Name, "/api/v3/ticker/24hr?symbol="+strings.ToUpper(market)),
	}

	tempTicker := &BinanceTicker{}
	_, err = utils.HTTPRequest(reqConf, tempTicker)
	if err != nil {
		return
	}
	ticker = &Ticker{
		Exchange:       string(Binance),
		Symbol:         tempTicker.Symbol,
		AskPrice:       tempTicker.AskPrice,
		BidPrice:       tempTicker.BidPrice,
		LastTradePrice: tempTicker.LastPrice,
	}

	return
}

func (s *Service) getBittrexTicker(market string) (ticker *Ticker, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(Bittrex, s.chainParams.Name, "/markets/"+strings.ToUpper(market)+"/ticker"),
	}

	tempTicker := &BittrexTicker{}
	_, err = utils.HTTPRequest(reqConf, tempTicker)
	if err != nil {
		return
	}
	ticker = &Ticker{
		Exchange:       string(Bittrex),
		Symbol:         tempTicker.Symbol,
		AskPrice:       tempTicker.Ask,
		BidPrice:       tempTicker.Bid,
		LastTradePrice: tempTicker.LastTradeRate,
	}

	return
}

func (s *Service) getKucoinTicker(market string) (ticker *Ticker, err error) {
	reqConf := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: setBackend(KuCoin, s.chainParams.Name, "/api/v1/market/orderbook/level1?symbol="+strings.ToUpper(market)),
	}

	tempTicker := &KuCoinTicker{}
	_, err = utils.HTTPRequest(reqConf, tempTicker)
	if err != nil {
		return
	}

	// Kucoin doesn't send back error code if it doesn't support the supplied market.
	// We should filter those instances using the sequence number.
	// When sequence is 0, no ticker data was returned.
	if tempTicker.Data.Sequence == 0 {
		return nil, errors.New("error occurred. Most likely unsupported Kucoin market")
	}

	ticker = &Ticker{
		Exchange:       string(KuCoin),
		Symbol:         strings.ToUpper(market),
		AskPrice:       tempTicker.Data.BestAsk,
		BidPrice:       tempTicker.Data.BestBid,
		LastTradePrice: tempTicker.Data.Price,
	}

	return
}
