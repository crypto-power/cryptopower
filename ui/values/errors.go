package values

import (
	"strings"

	"gitlab.com/raedah/cryptopower/libwallet"
)

// This files holds implementation to translate errors into user friendly messages.

// TranslateErr translates all server errors to user friendly messages.
func TranslateErr(errStr string) string {
	switch errStr {
	case libwallet.ErrInvalidPassphrase:
		return String(StrInvalidPassphrase)

	case libwallet.ErrNotConnected:
		return String(StrNotConnected)

	case libwallet.ErrInsufficientBalance:
		return String(StrInsufficentFund)

	default:
		if strings.Contains(errStr, "strconv.ParseFloat") {
			return String((StrInvalidAmount))
		}
	}
	return errStr
}
