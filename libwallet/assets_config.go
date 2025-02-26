package libwallet

import (
	"fmt"
	"time"

	"decred.org/dcrwallet/v4/errors"
	"github.com/asdine/storm"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/values"

	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/assets/ltc"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"

	"golang.org/x/crypto/bcrypt"
)

const (
	logFileName   = "libwallet.log"
	walletsDbName = "wallets.db"

	// Mainnet represents the main network.
	Mainnet = utils.Mainnet
	// Testnet3 represents the test network.
	Testnet = utils.Testnet

	walletsMetadataBucketName    = "metadata"
	walletStartupPassphraseField = "startup-passphrase"
	appConfigBucketName          = "app_config" // App level bucket.

	// GenesisTimestampMainnet represents the genesis timestamp for the DCR mainnet.
	GenesisTimestampMainnet = 1454954400
	// GenesisTimestampTestnet represents the genesis timestamp for the DCR testnet.
	GenesisTimestampTestnet = 1533513600

	// TargetTimePerBlockMainnet represents the target time per block in seconds for DCR mainnet.
	TargetTimePerBlockMainnet = 300
	// TargetTimePerBlockTestnet represents the target time per block in seconds for DCR testnet.
	TargetTimePerBlockTestnet = 120
)

// SaveAppConfigValue method manages all the write operations on the app's
// config.
func (mgr *AssetsManager) SaveAppConfigValue(key string, value interface{}) {
	err := mgr.params.DB.Set(appConfigBucketName, key, value)
	if err != nil {
		log.Errorf("error setting app config value for key: %s, error: %v", key, err)
	}
}

// ReadAppConfigValue reads a generic value stored against the provided key at
// the assets manager level.
func (mgr *AssetsManager) ReadAppConfigValue(key string, valueOut interface{}) {
	err := mgr.params.DB.Get(appConfigBucketName, key, valueOut)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading app config value for key: %s, error: %v", key, err)
	}
}

// appConfigDelete manages all delete operations on the app's config.
func (mgr *AssetsManager) appConfigDelete(key string) {
	err := mgr.params.DB.Delete(appConfigBucketName, key)
	if err != nil {
		log.Errorf("error deleting app config value for key: %s, error: %v", key, err)
	}
}

// SetStartupPassphrase sets the startup passphrase for the wallet.
func (mgr *AssetsManager) SetStartupPassphrase(passphrase string, passphraseType int32) error {
	return mgr.ChangeStartupPassphrase("", passphrase, passphraseType)
}

// VerifyStartupPassphrase verifies the startup passphrase for the wallet.
func (mgr *AssetsManager) VerifyStartupPassphrase(startupPassphrase string) error {
	var startupPassphraseHash []byte
	err := mgr.params.DB.Get(appConfigBucketName, walletStartupPassphraseField, &startupPassphraseHash)
	if err != nil && err != storm.ErrNotFound {
		return err
	}

	if startupPassphraseHash == nil {
		// startup passphrase was not previously set
		if len(startupPassphrase) > 0 {
			return errors.E(utils.ErrInvalidPassphrase)
		}
		return nil
	}

	// startup passphrase was set, verify
	err = bcrypt.CompareHashAndPassword(startupPassphraseHash, []byte(startupPassphrase))
	if err != nil {
		return errors.E(utils.ErrInvalidPassphrase)
	}

	return nil
}

// ChangeStartupPassphrase changes the startup passphrase for the wallet.
func (mgr *AssetsManager) ChangeStartupPassphrase(oldPassphrase, newPassphrase string, passphraseType int32) error {
	if len(newPassphrase) == 0 {
		return mgr.RemoveStartupPassphrase(oldPassphrase)
	}

	err := mgr.VerifyStartupPassphrase(oldPassphrase)
	if err != nil {
		return err
	}

	startupPassphraseHash, err := bcrypt.GenerateFromPassword([]byte(newPassphrase), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	mgr.SaveAppConfigValue(walletStartupPassphraseField, startupPassphraseHash)
	mgr.SaveAppConfigValue(sharedW.IsStartupSecuritySetConfigKey, true)
	mgr.SaveAppConfigValue(sharedW.StartupSecurityTypeConfigKey, passphraseType)
	return nil
}

// RemoveStartupPassphrase removes the startup passphrase for the wallet.
func (mgr *AssetsManager) RemoveStartupPassphrase(oldPassphrase string) error {
	err := mgr.VerifyStartupPassphrase(oldPassphrase)
	if err != nil {
		return err
	}

	mgr.appConfigDelete(walletStartupPassphraseField)
	mgr.SaveAppConfigValue(sharedW.IsStartupSecuritySetConfigKey, false)
	mgr.appConfigDelete(sharedW.StartupSecurityTypeConfigKey)

	return nil
}

// IsStartupSecuritySet checks if the startup security is set.
func (mgr *AssetsManager) IsStartupSecuritySet() bool {
	var data bool
	mgr.ReadAppConfigValue(sharedW.IsStartupSecuritySetConfigKey, &data)
	return data
}

// IsDarkModeOn checks if the dark mode is set.
func (mgr *AssetsManager) IsDarkModeOn() bool {
	var data bool
	mgr.ReadAppConfigValue(sharedW.DarkModeConfigKey, &data)
	return data
}

// SetDarkMode sets the dark mode for the app.
func (mgr *AssetsManager) SetDarkMode(data bool) {
	mgr.SaveAppConfigValue(sharedW.DarkModeConfigKey, data)
}

// GetCurrencyConversionExchange returns the currency conversion exchange.
func (mgr *AssetsManager) GetCurrencyConversionExchange() string {
	if mgr.RateSource != nil {
		return mgr.RateSource.Name()
	}
	var key string
	mgr.ReadAppConfigValue(sharedW.CurrencyConversionConfigKey, &key)
	if key == "" {
		return values.DefaultExchangeValue // default exchange value
	}
	return key
}

// SetCurrencyConversionExchange sets the currency conversion exchange.
func (mgr *AssetsManager) SetCurrencyConversionExchange(xc string) {
	mgr.rateMutex.Lock()
	defer mgr.rateMutex.Unlock()
	mgr.SaveAppConfigValue(sharedW.CurrencyConversionConfigKey, xc)
	go func() {
		err := mgr.RateSource.ToggleSource(xc)
		if err != nil {
			log.Errorf("Failed to toggle rate source: %v", err)
		}
	}()
}

// ExchangeRateFetchingEnabled returns true if privacy mode isn't turned on and
// a valid exchange rate source is configured.
func (mgr *AssetsManager) ExchangeRateFetchingEnabled() bool {
	if mgr.IsPrivacyModeOn() {
		return false
	}
	xc := mgr.GetCurrencyConversionExchange()
	return xc != "" && xc != values.DefaultExchangeValue
}

// GetLanguagePreference returns the language preference.
func (mgr *AssetsManager) GetLanguagePreference() string {
	var lang string
	mgr.ReadAppConfigValue(sharedW.LanguagePreferenceKey, &lang)
	return lang
}

// SetLanguagePreference sets the language preference.
func (mgr *AssetsManager) SetLanguagePreference(lang string) {
	mgr.SaveAppConfigValue(sharedW.LanguagePreferenceKey, lang)
}

// GetUserAgent returns the user agent.
func (mgr *AssetsManager) GetUserAgent() string {
	var data string
	mgr.ReadAppConfigValue(sharedW.UserAgentConfigKey, data)
	return data
}

// SetUserAgent sets the user agent.
func (mgr *AssetsManager) SetUserAgent(data string) {
	mgr.SaveAppConfigValue(sharedW.UserAgentConfigKey, data)
}

// IsTransactionNotificationsOn checks if the transaction notifications is set.
// When privacy mode is enabled, Tx notification is also disabled.
func (mgr *AssetsManager) IsTransactionNotificationsOn() bool {
	var data bool
	mgr.ReadAppConfigValue(sharedW.TransactionNotificationConfigKey, &data)
	return data && !mgr.IsPrivacyModeOn()
}

// SetTransactionsNotifications sets the transaction notifications for the app.
func (mgr *AssetsManager) SetTransactionsNotifications(data bool) {
	mgr.SaveAppConfigValue(sharedW.TransactionNotificationConfigKey, data)
}

// SetPrivacyMode sets the privacy mode for the app.
func (mgr *AssetsManager) SetPrivacyMode(isActive bool) {
	mgr.SaveAppConfigValue(sharedW.PrivacyModeConfigKey, isActive)
	mgr.RateSource.ToggleStatus(isActive)
	if !isActive && mgr.GetCurrencyConversionExchange() != values.DefaultExchangeValue {
		go mgr.RateSource.Refresh(true)
	}
}

// IsPrivacyModeOn checks if the privacy mode is set.
// If Privacy mode is on, no API calls that can be made.
func (mgr *AssetsManager) IsPrivacyModeOn() bool {
	var data bool
	mgr.ReadAppConfigValue(sharedW.PrivacyModeConfigKey, &data)
	return data
}

// SetHTTPAPIPrivacyMode sets Http API the privacy mode for the app.
func (mgr *AssetsManager) SetHTTPAPIPrivacyMode(apiType utils.HTTPAPIType, isActive bool) {
	dataKey := genKey(sharedW.PrivacyModeConfigKey, apiType)
	mgr.SaveAppConfigValue(dataKey, isActive)
}

// IsHTTPAPIPrivacyModeOff returns true if the given API type is enabled and false
// if otherwise.
func (mgr *AssetsManager) IsHTTPAPIPrivacyModeOff(apiType utils.HTTPAPIType) bool {
	var data bool
	dataKey := genKey(sharedW.PrivacyModeConfigKey, apiType)
	mgr.ReadAppConfigValue(dataKey, &data)
	return data && !mgr.IsPrivacyModeOn()
}

// GetLogLevels returns the log levels.
func (mgr *AssetsManager) GetLogLevels() string {
	var logLevel string
	mgr.ReadAppConfigValue(sharedW.LogLevelConfigKey, &logLevel)
	if logLevel == "" {
		// return default debug level if no option is stored.
		return utils.DefaultLogLevel
	}
	return logLevel
}

// SetLogLevels sets the log levels.
func (mgr *AssetsManager) SetLogLevels(logLevel string) {
	mgr.SaveAppConfigValue(sharedW.LogLevelConfigKey, logLevel)
	SetLogLevels(logLevel)
}

// SetExchangeConfig sets the exchange config for the asset.
func (mgr *AssetsManager) SetExchangeConfig(data sharedW.ExchangeConfig) {
	mgr.SaveAppConfigValue(sharedW.ExchangeSourceDstnTypeConfigKey, data)
}

// GetExchangeConfig returns the previously set exchange config for the asset.
func (mgr *AssetsManager) GetExchangeConfig() *sharedW.ExchangeConfig {
	data := &sharedW.ExchangeConfig{}
	mgr.ReadAppConfigValue(sharedW.ExchangeSourceDstnTypeConfigKey, data)
	return data
}

// IsExchangeConfigSet checks if the exchange config is set for the asset.
func (mgr *AssetsManager) IsExchangeConfigSet() bool {
	return mgr.GetExchangeConfig().SourceAsset != utils.NilAsset
}

// ClearExchangeConfig clears the wallet's exchange config.
func (mgr *AssetsManager) ClearExchangeConfig() {
	mgr.appConfigDelete(sharedW.ExchangeSourceDstnTypeConfigKey)
}

// IsTotalBalanceVisible checks if the total balance visibility is set.
func (mgr *AssetsManager) IsTotalBalanceVisible() bool {
	var data bool
	mgr.ReadAppConfigValue(sharedW.HideTotalBalanceConfigKey, &data)
	return data
}

// SetTotalBalanceVisibility sets the transaction notifications for the app.
func (mgr *AssetsManager) SetTotalBalanceVisibility(data bool) {
	mgr.SaveAppConfigValue(sharedW.HideTotalBalanceConfigKey, data)
}

func genKey(prefix, identifier interface{}) string {
	return fmt.Sprintf("%v-%v", prefix, identifier)
}

// GetDBDriver returns the saved db driver.
func (mgr *AssetsManager) GetDBDriver() string {
	var dbDriver string
	mgr.ReadAppConfigValue(sharedW.DBDriverConfigKey, &dbDriver)
	if dbDriver == "" {
		// return default db driver if no option is stored.
		return BoltDB
	}
	return dbDriver
}

// SetDBDriver sets the db driver.
func (mgr *AssetsManager) SetDBDriver(dbDriver string) {
	mgr.SaveAppConfigValue(sharedW.DBDriverConfigKey, dbDriver)
}

// GetGenesisTimestamp returns the genesis timestamp for the provided asset type and network.
func (mgr *AssetsManager) GetGenesisTimestamp(assetType utils.AssetType, network utils.NetworkType) int64 {
	switch assetType {
	case utils.DCRWalletAsset:
		return dcr.GetGenesisTimestamp(network)
	case utils.BTCWalletAsset:
		return btc.GetGenesisTimestamp(network)
	case utils.LTCWalletAsset:
		return ltc.GetGenesisTimestamp(network)
	default:
		return dcr.GetGenesisTimestamp(network) // Default to DCR
	}
}

// GetTargetTimePerBlock returns the target time per block for the provided asset type and network.
func (mgr *AssetsManager) GetTargetTimePerBlock(assetType utils.AssetType, network utils.NetworkType) int64 {
	switch assetType {
	case utils.DCRWalletAsset:
		return dcr.GetTargetTimePerBlock(network)
	case utils.BTCWalletAsset:
		return btc.GetTargetTimePerBlock(network)
	case utils.LTCWalletAsset:
		return ltc.GetTargetTimePerBlock(network)
	default:
		return dcr.GetTargetTimePerBlock(network) // Default to DCR
	}
}

// IsInternalStorageSufficient checks if the available disk space is sufficient for the
// wallet's operations.
func (mgr *AssetsManager) IsInternalStorageSufficient(assetType utils.AssetType, network utils.NetworkType) (bool, int64, uint64) {
	// Current timestamp in seconds
	currentTime := time.Now().Unix()

	// Calculate the estimated blocks since genesis
	blocksSinceGenesis := (currentTime - mgr.GetGenesisTimestamp(assetType, network)) / mgr.GetTargetTimePerBlock(assetType, network)

	// Estimated space requirement in MB (1MB per 1000 blocks)
	estimatedHeadersSize := blocksSinceGenesis / 1000

	// Get free internal memory in MB
	freeInternalMemory, err := utils.GetFreeDiskSpace()
	if err != nil {
		log.Infof("Error checking free disk space: %v", err)
		return false, 0, 0
	}

	// Check if available space is insufficient
	if uint64(estimatedHeadersSize) > freeInternalMemory {
		log.Infof("Insufficient storage space. Estimated headers size: %d MB, Free internal memory: %d MB\n", estimatedHeadersSize, freeInternalMemory)
		return false, estimatedHeadersSize, freeInternalMemory
	}

	return true, 0, 0
}
