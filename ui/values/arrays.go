package values

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/values/localizable"
)

var (
	ArrLanguages          map[string]string
	ArrExchangeCurrencies map[string]string
	LogLevels             map[string]string
)

const (
	DefaultExchangeValue = "none"
	DCRUSDTMarket        = "DCR-USDT"
	BTCUSDTMarket        = "BTC-USDT"
	BittrexExchange      = "bittrex"
	BinanceExchange      = "binance"

	DefaultLogLevel = utils.LogLevelInfo
)

func init() {
	ArrLanguages = map[string]string{
		localizable.ENGLISH: StrEnglish,
		localizable.FRENCH:  StrFrench,
		localizable.SPANISH: StrSpanish,
	}

	ArrExchangeCurrencies = map[string]string{
		BittrexExchange:      StrUsdBittrex,
		BinanceExchange:      StrUsdBinance,
		DefaultExchangeValue: StrNone,
	}

	LogLevels = map[string]string{
		utils.LogLevelTrace:    StrLogLevelTrace,
		utils.LogLevelDebug:    StrLogLevelDebug,
		utils.LogLevelInfo:     StrLogLevelInfo,
		utils.LogLevelWarn:     StrLogLevelWarn,
		utils.LogLevelError:    StrLogLevelError,
		utils.LogLevelCritical: StrLogLevelCritical,
		utils.LogLevelOff:      StrLogLevelOff,
	}
}
