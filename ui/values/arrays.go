package values

import "github.com/crypto-power/cryptopower/libwallet/utils"

// These are a list of supported rate sources.
const (
	DefaultExchangeValue = "none"
	BinanceExchange      = "binance"
	BinanceUSExchange    = "binanceus"
	Coinpaprika          = "coinpaprika"
	Messari              = "messari"
	KucoinExchange       = "kucoin"
)

// initialize an asset market value map
var AssetExchangeMarketValue = map[utils.AssetType]Market{
	utils.DCRWalletAsset: DCRUSDTMarket,
	utils.BTCWalletAsset: BTCUSDTMarket,
	utils.LTCWalletAsset: LTCUSDTMarket,
}
