package values

import (
	"strings"

	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// This files holds implementation to translate errors into user friendly messages.

// TranslateErr translates all server errors to user friendly messages.
func TranslateErr(errStr string) string {
	switch errStr {
	case utils.ErrInvalidPassphrase:
		return String(StrInvalidPassphrase)

	case utils.ErrNotConnected:
		return String(StrNotConnected)

	case utils.ErrInsufficientBalance:
		return String(StrInsufficentFund)

	default:
		if strings.Contains(errStr, "strconv.ParseFloat") {
			return String((StrInvalidAmount))
		}
	}
	return errStr
}
