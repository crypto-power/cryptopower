package instantswap

import (
	"context"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/crypto-power/instantswap/instantswap"
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
	APIKey    string
	APISecret string
	// AffiliateID is used to earn refer coin from transaction
	AffiliateID string
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
	Trocador   Server = "trocador"
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
	db  *storm.DB
	ctx context.Context

	syncMu     sync.RWMutex
	cancelSync context.CancelFunc

	SchedulerCtx           context.Context
	CancelOrderScheduler   context.CancelFunc
	CancelOrderSchedulerMu sync.RWMutex
	SchedulerStartTime     time.Time

	notificationListenersMu *sync.RWMutex // Pointer required to avoid copying literal values.
	notificationListeners   map[string]*OrderNotificationListener
}

type OrderNotificationListener struct {
	OnExchangeOrdersSynced  func()
	OnOrderCreated          func(order *Order)
	OnOrderSchedulerStarted func()
	OnOrderSchedulerEnded   func()
}

type Order struct {
	ID                       int            `storm:"id,increment"`
	UUID                     string         `storm:"unique" json:"uuid"`
	Server                   Server         `json:"server" storm:"index"`         // Legacy Exchange Server field, used to update the new ExchangeServer field
	ExchangeServer           ExchangeServer `json:"exchangeServer" storm:"index"` // New Exchange Server field
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
	FromNetwork  string `json:"fromNetwork"`
	ToNetwork    string `json:"toNetwork"`
	Provider     string `json:"provider"`

	DepositAddress     string  `json:"depositAddress"`     // Address where funds that need to be exchanged should be sent to
	RefundAddress      string  `json:"refundAddress"`      // Address where funds are returned to if the exchange fails
	DestinationAddress string  `json:"destinationAddress"` // Address where successfully converted funds would be sent to
	ExchangeRate       float64 `json:"exchangeRate"`
	ChargedFee         float64 `json:"chargedFee"`

	Confirmations string             `json:"confirmations"`
	Status        instantswap.Status `json:"status" storm:"index"`
	ExpiryTime    int                `json:"expiryTime"` // in seconds
	CreatedAt     int64              `storm:"index" json:"createdAt"`
	LastUpdate    string             `json:"lastUpdate"` // should be timestamp (api currently returns string)

	ExtraID string `json:"extraId"` // changenow.io requirement //changelly payinExtraId value
	UserID  string `json:"userId"`  // changenow.io partner requirement

	Signature string `json:"signature"` // evercoin requirement
}

type SchedulerParams struct {
	Order Order

	Frequency         time.Duration // in hours
	BalanceToMaintain float64
	// MaxDeviationRate is the maximum deviation rate allowed between
	// the exchange server rate and the market rate. If the deviation
	// rate is greater than the MaxDeviationRate, the order is not created
	MaxDeviationRate float64

	SpendingPassphrase string
}
