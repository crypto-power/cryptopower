package values

import "gitlab.com/raedah/cryptopower/ui/values/localizable"

var (
	ArrLanguages          map[string]string
	ArrExchangeCurrencies map[string]string
	ArrMixerAccounts      map[string]string
)

const (
	DefaultExchangeValue = "none"
	USDExchangeValue     = "USD (Bittrex)"

	DefaultAccount = StrDefault
	MixedAcc       = StrMixed
	UnmixedAcc     = StrUnmixed
)

func init() {
	ArrLanguages = map[string]string{localizable.ENGLISH: StrEnglish, localizable.FRENCH: StrFrench, localizable.SPANISH: StrSpanish}
	ArrExchangeCurrencies = map[string]string{DefaultExchangeValue: StrNone, USDExchangeValue: StrUsdBittrex}
	ArrMixerAccounts = map[string]string{StrDefault: StrDefault, StrMixed: StrMixed, StrUnmixed: StrUnmixed}
}
