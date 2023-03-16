package instantswap

import (
	"context"
	"sync"
	"time"

	"code.cryptopower.dev/group/instantswap"
	"github.com/asdine/storm"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ExchangeServer struct {
	Server Server
	Config ExchangeConfig
}

// ExchangeConfig
type ExchangeConfig struct {
	Debug     bool
	ApiKey    string
	ApiSecret string
	// AffiliateId is used to earn refer coin from transaction
	AffiliateId string
	UserID      string
}

type Server string

const (
	Changelly  Server = "changelly"
	ChangeNow  Server = "changenow"
	CoinSwitch Server = "coinswitch"
	FlypMe     Server = "flypme"
	GoDex      Server = "godex"
	SimpleSwap Server = "simpleswap"
	SwapZone   Server = "swapzone"
)

func (es Server) ToString() string {
	return string(es)
}

// CapFirstLetter capitalizes the first letter of the Server
func (es Server) CapFirstLetter() string {
	caser := cases.Title(language.Und)
	return caser.String(string(es))
}

type InstantSwap struct {
	db *storm.DB

	mu         *sync.RWMutex // Pointer required to avoid copying literal values.
	ctx        context.Context
	cancelSync context.CancelFunc

	SchedulerCtx           context.Context
	CancelOrderScheduler   context.CancelFunc
	CancelOrderSchedulerMu sync.RWMutex

	notificationListenersMu *sync.RWMutex // Pointer required to avoid copying literal values.
	notificationListeners   map[string]OrderNotificationListener
}

type OrderNotificationListener interface {
	OnExchangeOrdersSynced()
}

type Order struct {
	ID                       int            `storm:"id,increment"`
	UUID                     string         `storm:"unique" json:"uuid"`
	Server                   Server         `json:"server"`         // Legacy Exchange Server field, used to update the new ExchangeServer field
	ExchangeServer           ExchangeServer `json:"exchangeServer"` // New Exchange Server field
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

	ExtraID string `json:"extraId"` //changenow.io requirement //changelly payinExtraId value
	UserID  string `json:"userId"`  //changenow.io partner requirement

	Signature string `json:"signature"` //evercoin requirement
}

type SchedulerParams struct {
	Order Order

	// ExchangeServer ExchangeServer

	// SourceWalletID      int
	// SourceAccountNumber int32

	// FromCurrency string
	// ToCurrency   string
	// InvoicedAmount float64
	// DestinationAddress string
	// RefundAddress      string

	Frequency           time.Duration // in hours
	BalanceToMaintain   float64
	MinimumExchangeRate float64

	SpendingPassphrase string
}
