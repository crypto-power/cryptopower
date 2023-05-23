// Package wallet provides functions and types for interacting
// with the libwallet backend.
package wallet

import (
	"fmt"
	"path/filepath"
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

const (
	syncID      = "cryptopower"
	DevBuild    = "dev"
	ProdBuild   = "prod"
	logFilename = "cryptopower.log"
)

// Wallet represents the wallet back end of the app
type Wallet struct {
	assetsManager       *libwallet.AssetsManager
	Root                string
	buildDate           time.Time
	version             string
	logDir              string
	startUpTime         time.Time
	Net                 libutils.NetworkType
	LightningWorkingDir string
}

// NewWallet initializies an new Wallet instance.
// The Wallet is not loaded until LoadWallets is called.
func NewWallet(root, net, version, logFolder string, buildDate time.Time, lWokingDir string) (*Wallet, error) {
	if root == "" {
		return nil, fmt.Errorf("root directory cannot be empty")
	}

	resolvedNetType := libutils.ToNetworkType(net)
	if resolvedNetType == libutils.Unknown {
		return nil, fmt.Errorf("network type is not supportted: %s", net)
	}

	wal := &Wallet{
		Root:                root,
		Net:                 resolvedNetType,
		buildDate:           buildDate,
		version:             version,
		logDir:              logFolder,
		startUpTime:         time.Now(),
		LightningWorkingDir: lWokingDir,
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
	return filepath.Join(wal.logDir, logFilename)
}

func (wal *Wallet) StartupTime() time.Time {
	return wal.startUpTime
}

func (wal *Wallet) GetAssetsManager() *libwallet.AssetsManager {
	return wal.assetsManager
}

func (wal *Wallet) InitAssetsManager() error {
	politeiaHost := libwallet.PoliteiaMainnetHost
	if wal.Net == libwallet.Testnet {
		politeiaHost = libwallet.PoliteiaTestnetHost
	}
	assetsManager, err := libwallet.NewAssetsManager(wal.Root, "bdb", politeiaHost, wal.logDir, wal.Net, wal.LightningWorkingDir)
	if err != nil {
		return err
	}

	wal.assetsManager = assetsManager
	return nil
}

// Shutdown shutsdown the assetsManager
func (wal *Wallet) Shutdown() {
	if wal.assetsManager != nil {
		wal.assetsManager.Shutdown()
	}
}

// GetDCRBlockExplorerURL accept transaction hash,
// return the block explorer URL with respect to the network
func (wal *Wallet) GetDCRBlockExplorerURL(txnHash string) string {
	switch wal.Net {
	case libwallet.Testnet:
		return "https://testnet.dcrdata.org/tx/" + txnHash
	case libwallet.Mainnet:
		return "https://explorer.dcrdata.org/tx/" + txnHash
	default:
		return ""
	}
}

// GetBTCBlockExplorerURL accept transaction hash,
// return the block explorer URL with respect to the network
func (wal *Wallet) GetBTCBlockExplorerURL(txnHash string) string {
	switch wal.Net {
	case libwallet.Testnet:
		return "https://live.blockcypher.com/btc-testnet/tx/" + txnHash
	case libwallet.Mainnet:
		return "https://www.blockchain.com/btc/tx/" + txnHash
	default:
		return ""
	}
}

// GetLTCBlockExplorerURL accepts transaction hash,
// return the block explorer URL with respect to the network
func (wal *Wallet) GetLTCBlockExplorerURL(txnHash string) string {
	switch wal.Net {
	case libwallet.Testnet:
		return "https://chain.so/tx/LTCTEST/" + txnHash
	case libwallet.Mainnet:
		return "https://chain.so/tx/LTC/" + txnHash
	default:
		return ""
	}
}
