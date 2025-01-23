package wallet

import (
	"fmt"

	"github.com/asdine/storm"
)

const (
	AccountMixerConfigSet      = "account_mixer_config_set"
	AccountMixerMixedAccount   = "account_mixer_mixed_account"
	AccountMixerUnmixedAccount = "account_mixer_unmixed_account"
	AccountMixerMixTxChange    = "account_mixer_mix_tx_change"

	walletsMetadataBucketName = "metadata" // Wallet level bucket.

	LogLevelConfigKey           = "log_level"
	PrivacyModeConfigKey        = "privacy_mode"
	SpendUnconfirmedConfigKey   = "spend_unconfirmed"
	CurrencyConversionConfigKey = "currency_conversion_option"

	IsStartupSecuritySetConfigKey = "startup_security_set"
	StartupSecurityTypeConfigKey  = "startup_security_type"
	UseBiometricConfigKey         = "use_biometric"

	IncomingTxNotificationsConfigKey = "tx_notification_enabled"
	BeepNewBlocksConfigKey           = "beep_new_blocks"

	SyncOnCellularConfigKey             = "always_sync"
	NetworkModeConfigKey                = "network_mode"
	SpvPersistentPeerAddressesConfigKey = "spv_peer_addresses"
	UserAgentConfigKey                  = "user_agent"

	PoliteiaNotificationConfigKey = "politeia_notification"

	LastTxHashConfigKey = "last_tx_hash"

	KnownVSPsConfigKey = "known_vsps"

	TicketBuyerVSPHostConfigKey = "tb_vsp_host"
	TicketBuyerWalletConfigKey  = "tb_wallet_id"
	TicketBuyerAccountConfigKey = "tb_account_number"
	TicketBuyerATMConfigKey     = "tb_amount_to_maintain"

	ExchangeSourceDstnTypeConfigKey = "exchange_source_destination_key"

	HideBalanceConfigKey             = "hide_balance"
	AutoSyncConfigKey                = "autoSync"
	FetchProposalConfigKey           = "fetch_proposals"
	SeedBackupNotificationConfigKey  = "seed_backup_notification"
	ProposalNotificationConfigKey    = "proposal_notification_key"
	TransactionNotificationConfigKey = "transaction_notification_key"
	SpendUnmixedFundsKey             = "spend_unmixed_funds"
	LanguagePreferenceKey            = "app_language"
	DarkModeConfigKey                = "dark_mode"
	HideTotalBalanceConfigKey        = "hideTotalUSDBalance"
	IsCEXFirstVisitConfigKey         = "is_cex_first_visit"
	DBDriverConfigKey                = "db_driver"

	PassphraseTypePin  int32 = 0
	PassphraseTypePass int32 = 1
)

// walletConfigSave method manages all the write operations.
func (wallet *Wallet) walletConfigSave(key string, value interface{}) error {
	key = fmt.Sprintf("%d%s", wallet.ID, key)
	return wallet.db.Set(walletsMetadataBucketName, key, value)
}

// walletConfigRead manages all the read operations.
func (wallet *Wallet) walletConfigRead(key string, valueOut interface{}) error {
	key = fmt.Sprintf("%d%s", wallet.ID, key)
	return wallet.db.Get(walletsMetadataBucketName, key, valueOut)
}

// walletConfigDelete manages all delete operations.
func (wallet *Wallet) walletConfigDelete(key string) error {
	key = fmt.Sprintf("%d%s", wallet.ID, key)
	return wallet.db.Delete(walletsMetadataBucketName, key)
}

// SaveUserConfigValue stores the generic value against the provided key
// at the asset level.
func (wallet *Wallet) SaveUserConfigValue(key string, value interface{}) {
	err := wallet.walletConfigSave(key, value)
	if err != nil {
		log.Errorf("error setting user config value for key: %s, error: %v", key, err)
	}
}

// ReadUserConfigValue reads the generic value stored against the provided
// key at the asset level.
func (wallet *Wallet) ReadUserConfigValue(key string, valueOut interface{}) error {
	err := wallet.walletConfigRead(key, valueOut)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading user config value for key: %s, error: %v", key, err)
	}
	return err
}

// DeleteUserConfigValueForKey method deletes the value stored against the provided
// key at the asset level.
func (wallet *Wallet) DeleteUserConfigValueForKey(key string) {
	err := wallet.walletConfigDelete(key)
	if err != nil {
		log.Errorf("error deleting user config value for key: %s, error: %v", key, err)
	}
}

// SetBoolConfigValueForKey stores the boolean value against the provided key
// at the asset level.
func (wallet *Wallet) SetBoolConfigValueForKey(key string, value bool) {
	wallet.SaveUserConfigValue(key, value)
}

// SetDoubleConfigValueForKey stores the float64 value against the provided key
// at the asset level.
func (wallet *Wallet) SetDoubleConfigValueForKey(key string, value float64) {
	wallet.SaveUserConfigValue(key, value)
}

// SetIntConfigValueForKey stores the int value against the provided key
// at the asset level.
func (wallet *Wallet) SetIntConfigValueForKey(key string, value int) {
	wallet.SaveUserConfigValue(key, value)
}

// SetInt32ConfigValueForKey stores the int32 value against the provided key
// at the asset level.
func (wallet *Wallet) SetInt32ConfigValueForKey(key string, value int32) {
	wallet.SaveUserConfigValue(key, value)
}

// SetLongConfigValueForKey stores the int64 value against the provided key
// at the asset level.
func (wallet *Wallet) SetLongConfigValueForKey(key string, value int64) {
	wallet.SaveUserConfigValue(key, value)
}

// SetStringConfigValueForKey stores the string value against the provided key
// at the asset level.
func (wallet *Wallet) SetStringConfigValueForKey(key, value string) {
	wallet.SaveUserConfigValue(key, value)
}

// ReadBoolConfigValueForKey reads the boolean value stored against the provided
// key at the asset level. Provided default value is returned if the key is not found.
func (wallet *Wallet) ReadBoolConfigValueForKey(key string, defaultValue bool) (valueOut bool) {
	if err := wallet.ReadUserConfigValue(key, &valueOut); err == storm.ErrNotFound {
		valueOut = defaultValue
	}
	return
}

// ReadDoubleConfigValueForKey reads the float64 value stored against the provided
// key at the asset level. Provided default value is returned if the key is not found.
func (wallet *Wallet) ReadDoubleConfigValueForKey(key string, defaultValue float64) (valueOut float64) {
	if err := wallet.ReadUserConfigValue(key, &valueOut); err == storm.ErrNotFound {
		valueOut = defaultValue
	}
	return
}

// ReadIntConfigValueForKey reads the int value stored against the provided
// key at the asset level. Provided default value is returned if the key is not found.
func (wallet *Wallet) ReadIntConfigValueForKey(key string, defaultValue int) (valueOut int) {
	if err := wallet.ReadUserConfigValue(key, &valueOut); err == storm.ErrNotFound {
		valueOut = defaultValue
	}
	return
}

// ReadInt32ConfigValueForKey int32 the boolean value stored against the provided
// key at the asset level. Provided default value is returned if the key is not found.
func (wallet *Wallet) ReadInt32ConfigValueForKey(key string, defaultValue int32) (valueOut int32) {
	if err := wallet.ReadUserConfigValue(key, &valueOut); err == storm.ErrNotFound {
		valueOut = defaultValue
	}
	return
}

// ReadLongConfigValueForKey reads the int64 value stored against the provided
// key at the asset level. Provided default value is returned if the key is not found.
func (wallet *Wallet) ReadLongConfigValueForKey(key string, defaultValue int64) (valueOut int64) {
	if err := wallet.ReadUserConfigValue(key, &valueOut); err == storm.ErrNotFound {
		valueOut = defaultValue
	}
	return
}

// ReadStringConfigValueForKey reads the string value stored against the provided
// key at the asset level. Provided default value is returned if the key is not found.
func (wallet *Wallet) ReadStringConfigValueForKey(key string, defaultValue string) (valueOut string) {
	if err := wallet.ReadUserConfigValue(key, &valueOut); err == storm.ErrNotFound {
		valueOut = defaultValue
	}
	return
}
