package libwallet

import (
	"fmt"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"

	"golang.org/x/crypto/bcrypt"
)

const (
	logFileName   = "libwallet.log"
	walletsDbName = "wallets.db"

	// Mainnet represents the main network.
	Mainnet = utils.Mainnet
	// Testnet3 represents the test network.
	Testnet3 = utils.Testnet

	walletsMetadataBucketName    = "metadata"
	walletstartupPassphraseField = "startup-passphrase"
)

// setDBInterface extract the assets manager db interface that is available
// in each wallet by default from one of the validly created wallets.
func (mgr *AssetsManager) setDBInterface(db sharedW.AssetsManagerDB) {
	if db != nil {
		mgr.db = db
	}
}

// SetStartupPassphrase sets the startup passphrase for the wallet.
func (mgr *AssetsManager) SetStartupPassphrase(passphrase string, passphraseType int32) error {
	return mgr.ChangeStartupPassphrase("", passphrase, passphraseType)
}

// VerifyStartupPassphrase verifies the startup passphrase for the wallet.
func (mgr *AssetsManager) VerifyStartupPassphrase(startupPassphrase string) error {
	var startupPassphraseHash []byte
	err := mgr.db.ReadWalletConfigValue(walletstartupPassphraseField, &startupPassphraseHash)
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

	mgr.db.SaveWalletConfigValue(walletstartupPassphraseField, startupPassphraseHash)
	mgr.db.SaveWalletConfigValue(sharedW.IsStartupSecuritySetConfigKey, true)
	mgr.db.SaveWalletConfigValue(sharedW.StartupSecurityTypeConfigKey, passphraseType)

	return nil
}

// RemoveStartupPassphrase removes the startup passphrase for the wallet.
func (mgr *AssetsManager) RemoveStartupPassphrase(oldPassphrase string) error {
	err := mgr.VerifyStartupPassphrase(oldPassphrase)
	if err != nil {
		return err
	}

	mgr.db.DeleteWalletConfigValue(walletstartupPassphraseField)
	mgr.db.SaveWalletConfigValue(sharedW.IsStartupSecuritySetConfigKey, false)
	mgr.db.DeleteWalletConfigValue(sharedW.StartupSecurityTypeConfigKey)

	return nil
}

// IsStartupSecuritySet checks if the startup security is set.
func (mgr *AssetsManager) IsStartupSecuritySet() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.IsStartupSecuritySetConfigKey, &data)
	return data
}

// IsDarkModeOn checks if the dark mode is set.
func (mgr *AssetsManager) IsDarkModeOn() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.DarkModeConfigKey, &data)
	return data
}

// SetDarkMode sets the dark mode for the wallet.
func (mgr *AssetsManager) SetDarkMode(data bool) {
	mgr.db.SaveWalletConfigValue(sharedW.DarkModeConfigKey, data)
}

// GetDexServers returns the dex servers.
func (mgr *AssetsManager) GetDexServers() (map[string][]byte, error) {
	servers := make(map[string][]byte, 0)
	err := mgr.db.ReadWalletConfigValue(sharedW.KnownDexServersConfigKey, &servers)
	return servers, err
}

// SaveDexServers saves the dex servers.
func (mgr *AssetsManager) SaveDexServers(servers map[string][]byte) {
	mgr.db.SaveWalletConfigValue(sharedW.KnownDexServersConfigKey, servers)
}

// GetCurrencyConversionExchange returns the currency conversion exchange.
func (mgr *AssetsManager) GetCurrencyConversionExchange() string {
	var key string
	mgr.db.ReadWalletConfigValue(sharedW.CurrencyConversionConfigKey, &key)
	if key == "" {
		return "none" // default exchange value
	}
	return key
}

// SetCurrencyConversionExchange sets the currency conversion exchange.
func (mgr *AssetsManager) SetCurrencyConversionExchange(data string) {
	mgr.db.SaveWalletConfigValue(sharedW.CurrencyConversionConfigKey, data)
}

// GetLanguagePreference returns the language preference.
func (mgr *AssetsManager) GetLanguagePreference() string {
	var lang string
	mgr.db.ReadWalletConfigValue(sharedW.LanguagePreferenceKey, &lang)
	return lang
}

// SetLanguagePreference sets the language preference.
func (mgr *AssetsManager) SetLanguagePreference(lang string) {
	mgr.db.SaveWalletConfigValue(sharedW.LanguagePreferenceKey, lang)
}

// GetUserAgent returns the user agent.
func (mgr *AssetsManager) GetUserAgent() string {
	var data string
	mgr.db.ReadWalletConfigValue(sharedW.UserAgentConfigKey, data)
	return data
}

// SetUserAgent sets the user agent.
func (mgr *AssetsManager) SetUserAgent(data string) {
	mgr.db.SaveWalletConfigValue(sharedW.UserAgentConfigKey, data)
}

// IsTransactionNotificationsOn checks if the transaction notifications is set.
func (mgr *AssetsManager) IsTransactionNotificationsOn() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.TransactionNotificationConfigKey, &data)
	return data && mgr.IsPrivacyModeOn()
}

// SetTransactionsNotifications sets the transaction notifications for the wallet.
func (mgr *AssetsManager) SetTransactionsNotifications(data bool) {
	mgr.db.SaveWalletConfigValue(sharedW.TransactionNotificationConfigKey, data)
}

// SetPrivacyMode sets the privacy mode for the wallet.
func (mgr *AssetsManager) SetPrivacyMode(isActive bool) {
	mgr.db.SaveWalletConfigValue(sharedW.PrivacyModeConfigKey, isActive)
}

// IsPrivacyModeOn checks if the privacy mode is set.
// If Privacy mode is on, no API calls that can be made.
func (mgr *AssetsManager) IsPrivacyModeOn() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.PrivacyModeConfigKey, &data)
	return data
}

// SetHTTPAPIPrivacyMode sets Http API the privacy mode for the wallet.
func (mgr *AssetsManager) SetHTTPAPIPrivacyMode(apiType utils.HttpAPIType, isActive bool) {
	dataKey := genKey(sharedW.PrivacyModeConfigKey, apiType)
	mgr.db.SaveWalletConfigValue(dataKey, isActive)
}

// IsHttpAPIPrivacyModeOff returns true if the given API type is enabled and false
// if otherwise.
func (mgr *AssetsManager) IsHttpAPIPrivacyModeOff(apiType utils.HttpAPIType) bool {
	var data bool
	dataKey := genKey(sharedW.PrivacyModeConfigKey, apiType)
	mgr.db.ReadWalletConfigValue(dataKey, &data)
	return data && !mgr.IsPrivacyModeOn()
}

// GetLogLevels returns the log levels.
func (mgr *AssetsManager) GetLogLevels() string {
	var logLevel string
	mgr.db.ReadWalletConfigValue(sharedW.LogLevelConfigKey, &logLevel)
	if logLevel == "" {
		// return default debug level if no option is stored.
		return utils.DefaultLogLevel
	}
	return logLevel
}

// SetLogLevels sets the log levels.
func (mgr *AssetsManager) SetLogLevels(logLevel string) {
	mgr.db.SaveWalletConfigValue(sharedW.LogLevelConfigKey, logLevel)
	SetLogLevels(logLevel)
}

// SetExchangeConfig sets the exchange config for the asset.
func (mgr *AssetsManager) SetExchangeConfig(data sharedW.ExchangeConfig) {
	mgr.db.SaveWalletConfigValue(sharedW.ExchangeSourceDstnTypeConfigKey, data)
}

// GetExchangeConfig returns the previously set exchange config for the asset.
func (mgr *AssetsManager) GetExchangeConfig() *sharedW.ExchangeConfig {
	data := &sharedW.ExchangeConfig{}
	mgr.db.ReadWalletConfigValue(sharedW.ExchangeSourceDstnTypeConfigKey, data)
	return data
}

// IsExchangeConfigSet checks if the exchange config is set for the asset.
func (mgr *AssetsManager) IsExchangeConfigSet() bool {
	return mgr.GetExchangeConfig().SourceAsset != utils.NilAsset
}

// ClearExchangeConfig clears the wallet's exchange config.
func (mgr *AssetsManager) ClearExchangeConfig() {
	mgr.db.DeleteWalletConfigValue(sharedW.ExchangeSourceDstnTypeConfigKey)
}

func genKey(prefix, identifier interface{}) string {
	return fmt.Sprintf("%v-%v", prefix, identifier)
}
