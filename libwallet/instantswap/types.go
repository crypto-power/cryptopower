package instantswap

import (
	"strings"

	"code.cryptopower.dev/group/instantswap"
	"github.com/asdine/storm"
)

type ExchangeServer string

const (
	Changelly  ExchangeServer = "changelly"
	ChangeNow  ExchangeServer = "changenow"
	CoinSwitch ExchangeServer = "coinswitch"
	FlypMe     ExchangeServer = "flypme"
	GoDex      ExchangeServer = "godex"
	SimpleSwap ExchangeServer = "simpleswap"
	SwapZone   ExchangeServer = "swapzone"
)

func (es ExchangeServer) ToString() string {
	return string(es)
}

// CapFirstLetter capitalizes the first letter of the ExchangeServer
func (es ExchangeServer) CapFirstLetter() string {
	return strings.Title(string(es))
}

type InstantSwap struct {
	db *storm.DB
}

type Order struct {
	ID                       int            `storm:"id,increment"`
	UUID                     string         `storm:"unique" json:"uuid"`
	Server                   ExchangeServer `json:"server"`
	SourceWalletID           int            `json:"sourceWalletID"`
	SourceAccountNumber      int32          `json:"sourceAccountNumber"`
	DestinationWalletID      int            `json:"destinationWalletID"`
	DestinationAccountNumber int32          `json:"destinationAccountNumber"`

	OrderedAmount  float64 `json:"orderedAmount"`
	InvoicedAmount float64 `json:"invoicedAmount"`
	ReceiveAmount  float64 `json:"receiveAmount"`
	TxID           string  `json:"txid"`

	FromCurrency string `json:"fromCurrency"`
	ToCurrency   string `json:"toCurrency"`

	DepositAddress     string  `json:"depositAddress"`     // Address where funds that need to be exchanged should be sent to
	RefundAddress      string  `json:"refundAddress"`      // Address where funds are returned to if the exchange fails
	DestinationAddress string  `json:"destinationAddress"` // Address where successfully converted funds would be sent to
	ExchangeRate       float64 `json:"exchangeRate"`
	ChargedFee         float64 `json:"chargedFee"`

	Confirmations string             `json:"confirmations"`
	Status        instantswap.Status `json:"status"`
	ExpiryTime    int                `json:"expiryTime"` // in seconds
	CreatedAt     int64              `storm:"index" json:"createdAt"`
	LastUpdate    string             `json:"lastUpdate"` // should be timestamp (api currently returns string)

	ExtraID   string `json:"extraId"`   //changenow.io requirement //changelly payinExtraId value
	Signature string `json:"signature"` //evercoin requirement
}
