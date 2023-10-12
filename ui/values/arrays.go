package values

import "github.com/crypto-power/cryptopower/libwallet/utils"

const (
	DefaultExchangeValue = "none"
	DCRUSDTMarket        = "DCR-USDT"
	BTCUSDTMarket        = "BTC-USDT"
	LTCUSDTMarket        = "LTC-USDT"
	DCRBTCMarket         = "DCR-BTC"
	LTCBTCMarket         = "LTC-BTC"
	BittrexExchange      = "Bittrex"
	BinanceExchange      = "Binance"
)

// initialize an asset market value map
var AssetExchangeMarketValue = map[utils.AssetType]string{
	utils.DCRWalletAsset: DCRUSDTMarket,
	utils.BTCWalletAsset: BTCUSDTMarket,
	utils.LTCWalletAsset: LTCUSDTMarket,
}
