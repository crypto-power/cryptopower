// Package wallet provides functions and types for interacting
// with the libwallet backend.
package wallet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet"
)

const (
	syncID    = "cryptopower"
	DevBuild  = "dev"
	ProdBuild = "prod"
)

// Wallet represents the wallet back end of the app
type Wallet struct {
	multi       *libwallet.AssetsManager
	Root, Net   string
	buildDate   time.Time
	version     string
	logFile     string
	startUpTime time.Time
}

// NewWallet initializies an new Wallet instance.
// The Wallet is not loaded until LoadWallets is called.
func NewWallet(root, net, version, logFile string, buildDate time.Time) (*Wallet, error) {
	if root == "" || net == "" { // This should really be handled by libwallet
		return nil, fmt.Errorf(`root directory or network cannot be ""`)
	}

	wal := &Wallet{
		Root:        root,
		Net:         net,
		buildDate:   buildDate,
		version:     version,
		logFile:     logFile,
		startUpTime: time.Now(),
	}

	return wal, nil
}

func (wal *Wallet) BuildDate() time.Time {
	return wal.buildDate
}

func (wal *Wallet) Version() string {
	return wal.version
}

func (wal *Wallet) LogFile() string {
	return wal.logFile
}

func (wal *Wallet) StartupTime() time.Time {
	return wal.startUpTime
}

func (wal *Wallet) InitMultiWallet() error {
	politeiaHost := libwallet.PoliteiaMainnetHost
	if wal.Net == string(libwallet.Testnet3) {
		politeiaHost = libwallet.PoliteiaTestnetHost
	}
	multiWal, err := libwallet.NewAssetsManager(wal.Root, "bdb", wal.Net, politeiaHost)
	if err != nil {
		return err
	}

	wal.multi = multiWal
	return nil
}

// Shutdown shutsdown the multiwallet
func (wal *Wallet) Shutdown() {
	if wal.multi != nil {
		wal.multi.Shutdown()
	}
}

// GetDCRBlockExplorerURL accept transaction hash,
// return the block explorer URL with respect to the network
func (wal *Wallet) GetDCRBlockExplorerURL(txnHash string) string {
	switch wal.Net {
	case string(libwallet.Testnet3):
		return "https://testnet.dcrdata.org/tx/" + txnHash
	case string(libwallet.Mainnet):
		return "https://explorer.dcrdata.org/tx/" + txnHash
	default:
		return ""
	}
}

// GetBTCBlockExplorerURL accept transaction hash,
// return the block explorer URL with respect to the network
func (wal *Wallet) GetBTCBlockExplorerURL(txnHash string) string {
	switch wal.Net {
	case string(libwallet.Testnet3):
		return "https://live.blockcypher.com/btc-testnet/tx/" + txnHash
	case string(libwallet.Mainnet):
		return "https://www.blockchain.com/btc/tx/" + txnHash
	default:
		return ""
	}
}

//GetUSDExchangeValues gets the exchange rate of DCR - USDT from a specified endpoint
func (wal *Wallet) GetUSDExchangeValues(target interface{}) error {
	url := "https://api.bittrex.com/v3/markets/DCR-USDT/ticker"
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(target)
	return nil
}
