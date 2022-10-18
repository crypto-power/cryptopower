package libwallet

import (
	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
	"gitlab.com/raedah/cryptopower/libwallet/utils"

	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"

	"golang.org/x/crypto/bcrypt"
)

const (
	logFileName   = "libwallet.log"
	walletsDbName = "wallets.db"

	Mainnet  = utils.Mainnet
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

func (mgr *AssetsManager) SetStartupPassphrase(passphrase []byte, passphraseType int32) error {
	return mgr.ChangeStartupPassphrase([]byte(""), passphrase, passphraseType)
}

func (mgr *AssetsManager) VerifyStartupPassphrase(startupPassphrase []byte) error {
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
	err = bcrypt.CompareHashAndPassword(startupPassphraseHash, startupPassphrase)
	if err != nil {
		return errors.E(utils.ErrInvalidPassphrase)
	}

	return nil
}

func (mgr *AssetsManager) ChangeStartupPassphrase(oldPassphrase, newPassphrase []byte, passphraseType int32) error {
	if len(newPassphrase) == 0 {
		return mgr.RemoveStartupPassphrase(oldPassphrase)
	}

	err := mgr.VerifyStartupPassphrase(oldPassphrase)
	if err != nil {
		return err
	}

	startupPassphraseHash, err := bcrypt.GenerateFromPassword(newPassphrase, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	mgr.db.SaveWalletConfigValue(walletstartupPassphraseField, startupPassphraseHash)
	mgr.db.SaveWalletConfigValue(sharedW.IsStartupSecuritySetConfigKey, true)
	mgr.db.SaveWalletConfigValue(sharedW.StartupSecurityTypeConfigKey, passphraseType)

	return nil
}

func (mgr *AssetsManager) RemoveStartupPassphrase(oldPassphrase []byte) error {
	err := mgr.VerifyStartupPassphrase(oldPassphrase)
	if err != nil {
		return err
	}

	mgr.db.DeleteWalletConfigValue(walletstartupPassphraseField)
	mgr.db.SaveWalletConfigValue(sharedW.IsStartupSecuritySetConfigKey, false)
	mgr.db.DeleteWalletConfigValue(sharedW.StartupSecurityTypeConfigKey)

	return nil
}

func (mgr *AssetsManager) IsStartupSecuritySet() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.IsStartupSecuritySetConfigKey, &data)
	return data
}

func (mgr *AssetsManager) IsDarkModeOn() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.DarkModeConfigKey, &data)
	return data
}

func (mgr *AssetsManager) SetDarkMode(data bool) {
	mgr.db.SaveWalletConfigValue(sharedW.DarkModeConfigKey, data)
}

func (mgr *AssetsManager) GetDexServers() (map[string][]byte, error) {
	var servers = make(map[string][]byte, 0)
	err := mgr.db.ReadWalletConfigValue(sharedW.KnownDexServersConfigKey, &servers)
	return servers, err
}

func (mgr *AssetsManager) SaveDexServers(servers map[string][]byte) {
	mgr.db.SaveWalletConfigValue(sharedW.KnownDexServersConfigKey, servers)
}

func (mgr *AssetsManager) GetCurrencyConversionExchange() string {
	var key string
	mgr.db.ReadWalletConfigValue(sharedW.CurrencyConversionConfigKey, &key)
	if key == "" {
		return "none" // default exchange value
	}
	return key
}

func (mgr *AssetsManager) SetCurrencyConversionExchange(data string) {
	mgr.db.SaveWalletConfigValue(sharedW.CurrencyConversionConfigKey, data)
}

func (mgr *AssetsManager) GetLanguagePreference() string {
	var lang string
	mgr.db.ReadWalletConfigValue(sharedW.LanguagePreferenceKey, &lang)
	return lang
}

func (mgr *AssetsManager) SetLanguagePreference(lang string) {
	mgr.db.SaveWalletConfigValue(sharedW.LanguagePreferenceKey, lang)
}

func (mgr *AssetsManager) GetUserAgent() string {
	var data string
	mgr.db.ReadWalletConfigValue(sharedW.UserAgentConfigKey, data)
	return data
}

func (mgr *AssetsManager) SetUserAgent(data string) {
	mgr.db.SaveWalletConfigValue(sharedW.UserAgentConfigKey, data)
}

func (mgr *AssetsManager) IsTransactionNotificationsOn() bool {
	var data bool
	mgr.db.ReadWalletConfigValue(sharedW.TransactionNotificationConfigKey, &data)
	return data
}

func (mgr *AssetsManager) SetTransactionsNotifications(data bool) {
	mgr.db.SaveWalletConfigValue(sharedW.TransactionNotificationConfigKey, data)
}

func (mgr *AssetsManager) SetLogLevels() {
	//TODO: loglevels should have a custom type supported on libwallet.
	// Issue is to be addressed in here: https://code.cryptopower.dev/group/cryptopower/-/issues/965
	var logLevel string
	mgr.db.ReadWalletConfigValue(sharedW.LogLevelConfigKey, &logLevel)
	SetLogLevels(logLevel)
}
