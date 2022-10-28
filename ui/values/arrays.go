package values

import "gitlab.com/raedah/cryptopower/ui/values/localizable"

var (
	ArrLanguages          map[string]string
	ArrExchangeCurrencies map[string]string
)

const (
	DefaultExchangeValue = "none"
	DCRUSDTMarket        = "DCR-USDT"
	BittrexExchange      = "bittrex"
	BinanceExchange      = "binance"
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
}
