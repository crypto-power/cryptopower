package values

import "github.com/crypto-power/cryptopower/libwallet/utils"

// These are a list of markets supported by rate sources.
const (
	DCRUSDTMarket = "DCR-USDT"
	BTCUSDTMarket = "BTC-USDT"
	LTCUSDTMarket = "LTC-USDT"
	DCRBTCMarket  = "DCR-BTC"
	LTCBTCMarket  = "LTC-BTC"
)

// These are a list of supported rate sources.
const (
	DefaultExchangeValue = "none"
	BittrexExchange      = "bittrex"
	BinanceExchange      = "binance"
)

// initialize an asset market value map
var AssetExchangeMarketValue = map[utils.AssetType]string{
	utils.DCRWalletAsset: DCRUSDTMarket,
	utils.BTCWalletAsset: BTCUSDTMarket,
	utils.LTCWalletAsset: LTCUSDTMarket,
}
