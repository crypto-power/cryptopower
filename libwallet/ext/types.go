package ext

import (
	"time"

	"github.com/decred/dcrdata/v8/api/types"
)

type (
	// BlockDataBasic models primary information about a block.
	BlockDataBasic struct {
		Height     uint32        `json:"height"`
		Size       uint32        `json:"size"`
		Hash       string        `json:"hash"`
		Difficulty float64       `json:"diff"`
		StakeDiff  float64       `json:"sdiff"`
		Time       types.TimeAPI `json:"time"`
		NumTx      uint32        `json:"txlength"`
		MiningFee  *int64        `json:"fees,omitempty"`
		TotalSent  *int64        `json:"total_sent,omitempty"`
		// TicketPoolInfo may be nil for side chain blocks.
		PoolInfo *types.TicketPoolInfo `json:"ticket_pool,omitempty"`
	}

	// TreasuryDetails is the current balance, spent amount, and tx count for
	// the treasury.
	TreasuryDetails struct {
		Height         int64 `json:"height"`
		MaturityHeight int64 `json:"maturity_height"`
		Balance        int64 `json:"balance"`
		TxCount        int64 `json:"output_count"`
		AddCount       int64 `json:"add_count"`
		Added          int64 `json:"added"`
		SpendCount     int64 `json:"spend_count"`
		Spent          int64 `json:"spent"`
		TBaseCount     int64 `json:"tbase_count"`
		TBase          int64 `json:"tbase"`
		ImmatureCount  int64 `json:"immature_count"`
		Immature       int64 `json:"immature"`
	}

	// BaseState are the non-iterable fields of the ExchangeState, which embeds
	// BaseState.
	BaseState struct {
		Price float64 `json:"price"`
		// BaseVolume is poorly named. This is the volume in terms of (usually)
		// BTC, not the base asset of any particular market.
		BaseVolume float64 `json:"base_volume,omitempty"`
		Volume     float64 `json:"volume,omitempty"`
		Change     float64 `json:"change,omitempty"`
		Stamp      int64   `json:"timestamp,omitempty"`
	}

	// ExchangeRates is the dcr and btc prices converted to fiat.
	ExchangeRates struct {
		BtcIndex  string               `json:"btcIndex"`
		DcrPrice  float64              `json:"dcrPrice"`
		BtcPrice  float64              `json:"btcPrice"`
		Exchanges map[string]BaseState `json:"exchanges"`
	}
	// ExchangeState models the dcrdata supported exchanges state.
	ExchangeState struct {
		BtcIndex    string                    `json:"btc_index"`
		BtcPrice    float64                   `json:"btc_fiat_price"`
		Price       float64                   `json:"price"`
		Volume      float64                   `json:"volume"`
		DcrBtc      map[string]*ExchangeState `json:"dcr_btc_exchanges"`
		FiatIndices map[string]*ExchangeState `json:"btc_indices"`
	}

	// AddressState models the address balances and transactions.
	AddressState struct {
		Address            string   `json:"address"`
		Balance            int64    `json:"balance,string"`
		TotalReceived      int64    `json:"totalReceived,string"`
		TotalSent          int64    `json:"totalSent,string"`
		UnconfirmedBalance int64    `json:"unconfirmedBalance,string"`
		UnconfirmedTxs     int64    `json:"unconfirmedTxs"`
		Txs                int32    `json:"txs"`
		TxIds              []string `json:"txids"`
	}

	// XpubAddress models data about a specific xpub token.
	XpubAddress struct {
		Address       string `json:"name"`
		Path          string `json:"path"`
		Transfers     int32  `json:"transfers"`
		Decimals      int32  `json:"decimals"`
		Balance       int64  `json:"balance,string"`
		TotalReceived int64  `json:"totalReceived,string"`
		TotalSent     int64  `json:"totalSent,string"`
	}

	// XpubBalAndTxs models xpub transactions and balance.
	XpubBalAndTxs struct {
		Xpub               string        `json:"address"`
		Balance            int64         `json:"balance,string"`
		TotalReceived      int64         `json:"totalReceived,string"`
		TotalSent          int64         `json:"totalSent,string"`
		UnconfirmedBalance int64         `json:"unconfirmedBalance,string"`
		UnconfirmedTxs     int64         `json:"unconfirmedTxs"`
		Txs                int32         `json:"txs"`
		TxIds              []string      `json:"txids"`
		UsedTokens         int32         `json:"usedTokens"`
		XpubAddress        []XpubAddress `json:"tokens"`
	}

	// Ticker is the generic ticker information that is returned to a caller of
	// GetTicker function.
	Ticker struct {
		Market             string
		LastTradePrice     float64
		PriceChangePercent *float64

		lastUpdate time.Time
	}

	// BittrexTickerResponse models bittrex specific ticker information from
	// markets/{market}/ticker.
	BittrexTickerResponse struct {
		Symbol        string  `json:"symbol"`
		LastTradeRate float64 `json:"lastTradeRate,string"`
	}

	// BittrexMarketSummaryResponse is the response from
	// markets/{market}/summary.
	BittrexMarketSummaryResponse struct {
		Symbol        string  `json:"symbol"`
		PercentChange float64 `json:"percentChange,string"`
	}

	// BinanceTickerResponse models binance specific ticker information.
	BinanceTickerResponse struct {
		Symbol             string  `json:"symbol"`
		LastPrice          float64 `json:"lastPrice,string"`
		PriceChangePercent float64 `json:"priceChangePercent,string"`
	}

	// KuCoinTicker models Kucoin's specific ticker information.
	KuCoinTicker struct {
		Code int `json:"code,string"`
		Data struct {
			Time        int64   `json:"time"`
			Sequence    int64   `json:"sequence,string"`
			Price       float64 `json:"price,string"`
			Size        float64 `json:"size,string"`
			BestBid     float64 `json:"bestBid,string"`
			BestBidSize float64 `json:"bestBidSize,string"`
			BestAsk     float64 `json:"bestAsk,string"`
			BestAskSize float64 `json:"bestAskSize,string"`
		} `json:"data"`
	}

	// AgendaAPIResponse holds two sets of AgendaVoteChoices charts data.
	AgendaAPIResponse struct {
		ByHeight *AgendaVoteChoices `json:"by_height"`
		ByTime   *AgendaVoteChoices `json:"by_time"`
	}

	// AgendaVoteChoices contains the vote counts on multiple intervals of time. The
	// interval length may be either a single block, in which case Height contains
	// the block heights, or a day, in which case Time contains the time stamps of
	// each interval. Total is always the sum of Yes, No, and Abstain.
	AgendaVoteChoices struct {
		Abstain []uint64    `json:"abstain"`
		Yes     []uint64    `json:"yes"`
		No      []uint64    `json:"no"`
		Total   []uint64    `json:"total"`
		Height  []uint64    `json:"height,omitempty"`
		Time    []time.Time `json:"time,omitempty"`
	}

	sourceURLs struct {
		price, stats, ws string
	}
)
