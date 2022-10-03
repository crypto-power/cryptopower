package load

import (
	"golang.org/x/text/message"
)

func FormatUSDBalance(p *message.Printer, balance float64) string {
	return p.Sprintf("$%.2f", balance)
}

func DCRToUSD(exchangeRate, dcr float64) float64 {
	return dcr * exchangeRate
}

func USDToDCR(exchangeRate, usd float64) float64 {
	return usd / exchangeRate
}
