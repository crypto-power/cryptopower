package values

import "gitlab.com/raedah/cryptopower/ui/values/localizable"

var (
	ArrLanguages          map[string]string
	ArrExchangeCurrencies map[string]string
)

const (
	DefaultExchangeValue = "none"
	USDExchangeValue     = "USD (Bittrex)"
)

func init() {
	ArrLanguages = map[string]string{
		localizable.ENGLISH: StrEnglish,
		localizable.FRENCH:  StrFrench,
		localizable.SPANISH: StrSpanish,
	}

	ArrExchangeCurrencies = map[string]string{
		DefaultExchangeValue: StrNone,
		USDExchangeValue:     StrUsdBittrex,
	}
}
