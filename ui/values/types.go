package values

import (
	"strings"
)

// The structure of the Market class is Asset-Unit
// example: BTC-USDT, DCR-BTC
type Market string

const (
	DCRUSDTMarket Market = "DCR-USDT"
	BTCUSDTMarket Market = "BTC-USDT"
	LTCUSDTMarket Market = "LTC-USDT"
	UnknownMarket Market = "Unknown"
)

func (m Market) String() string {
	return string(m)
}

func (m Market) AssetString() string {
	marketArr := strings.Split(m.String(), "-")
	return marketArr[0]
}
