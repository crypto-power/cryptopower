package values

import (
	"fmt"
	"strings"
)

// The structure of the Market class is Asset-Unit
// example: BTC-USDT, DCR-BTC
type Market string

const (
	DCRUSDTMarket Market = "DCR-USDT"
	BTCUSDTMarket Market = "BTC-USDT"
	LTCUSDTMarket Market = "LTC-USDT"
	DCRBTCMarket  Market = "DCR-BTC"
	LTCBTCMarket  Market = "LTC-BTC"
	UnknownMarket Market = "Unknown"
)

func NewMarket(asset, unit string) Market {
	return Market(fmt.Sprintf("%s-%s", asset, unit))
}

func (m Market) String() string {
	return string(m)
}

func (m Market) AssetString() string {
	marketArr := strings.Split(m.String(), "-")
	return marketArr[0]
}

func (m Market) MarketWithoutSep() string {
	market := strings.ReplaceAll(m.String(), "-", "")
	return market
}
