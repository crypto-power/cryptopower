package instantswap

import "github.com/asdine/storm"

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

func (str ExchangeServer) ToString() string {
	return string(str)
}

type InstantSwap struct {
	db *storm.DB
}

type Order struct {
	ID                   int    `storm:"id,increment"`
	UUID                 string `storm:"unique" json:"uuid"`
	SourceWalletID       int    `json:"sourceWalletID"`
	SourceAccountID      int    `json:"sourceAccountID"`
	DestinationWalletID  int    `json:"destinationWalletID"`
	DestinationAccountID int    `json:"destinationAccountID"`

	OrderedAmount float64 `json:"orderedAmount"`
	ReceiveAmount float64 `json:"receiveAmount"`
	TxID          string  `json:"txid"`

	FromCurrency string `json:"fromCurrency"`
	ToCurrency   string `json:"toCurrency"`

	DepositAddress     string  `json:"depositAddress"`     // Address where funds that need to be exchanged should be sent to
	RefundAddress      string  `json:"refundAddress"`      // Address where funds are returned to if the exchange fails
	DestinationAddress string  `json:"destinationAddress"` // Address where successfully converted funds would be sent to
	ExchangeRate       float64 `json:"exchangeRate"`
	ChargedFee         float64 `json:"chargedFee"`

	Confirmations string `json:"confirmations"`
	Status        string `json:"status"`
	ExpiryTime    int    `json:"expiryTime"` // in seconds
	CreatedAt     int64  `storm:"index" json:"createdAt"`
	LastUpdate    string `json:"lastUpdate"`

	ExtraID string `json:"extraId"` //changenow.io requirement //changelly payinExtraId value
}
