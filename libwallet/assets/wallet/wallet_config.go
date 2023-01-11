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

	userConfigBucketName      = "user_config" // Asset level bucket.
	walletsMetadataBucketName = "metadata"    // Wallet level bucket.

	LogLevelConfigKey = "log_level"

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

	ExchangeSourceAssetTypeConfigKey      = "exchange_source_asset_type"
	ExchangeDestinationAssetTypeConfigKey = "exchange_destination_asset_type"
	ExchangeSourceWalletConfigKey         = "exchange_source_wallet"
	ExchangeDestinationWalletConfigKey    = "exchange_destination_wallet"
	ExchangeSourceAccountConfigKey        = "exchange_source_account"
	ExchangeDestinationAccountConfigKey   = "exchange_destination_account"

	HideBalanceConfigKey             = "hide_balance"
	AutoSyncConfigKey                = "autoSync"
	FetchProposalConfigKey           = "fetch_proposals"
	SeedBackupNotificationConfigKey  = "seed_backup_notification"
	ProposalNotificationConfigKey    = "proposal_notification_key"
	TransactionNotificationConfigKey = "transaction_notification_key"
	SpendUnmixedFundsKey             = "spend_unmixed_funds"
	KnownDexServersConfigKey         = "known_dex_servers"
	LanguagePreferenceKey            = "app_language"
	DarkModeConfigKey                = "dark_mode"

	PassphraseTypePin  int32 = 0
	PassphraseTypePass int32 = 1
)

// AssetsManagerDB defines the main generic methods required to access and manage
// the DB at the assets manager level.
type AssetsManagerDB interface {
	// DeleteWalletConfigValue deletes a generic value at the assets manager level.
	DeleteWalletConfigValue(key string)
	// SaveWalletConfigValue stores a generic value at the assets manager level.
	SaveWalletConfigValue(key string, value interface{})
	// ReadWalletConfigValue reads a generic value at the assets manager level.
	ReadWalletConfigValue(key string, valueOut interface{}) error
}

// walletConfigSave method manages all the write operations.
func (wallet *Wallet) walletConfigSave(isAssetsManager bool, key string, value interface{}) error {
	bucket := walletsMetadataBucketName
	if !isAssetsManager {
		bucket = userConfigBucketName
		key = fmt.Sprintf("%d%s", wallet.ID, key)
	}
	return wallet.db.Set(bucket, key, value)
}

// walletConfigRead manages all the read operations.
func (wallet *Wallet) walletConfigRead(isAssetsManager bool, key string, valueOut interface{}) error {
	bucket := walletsMetadataBucketName
	if !isAssetsManager {
		bucket = userConfigBucketName
		key = fmt.Sprintf("%d%s", wallet.ID, key)
	}
	return wallet.db.Get(bucket, key, valueOut)
}

// walletConfigDelete manages all delete operations.
func (wallet *Wallet) walletConfigDelete(isAssetsManager bool, key string) error {
	bucket := walletsMetadataBucketName
	if !isAssetsManager {
		bucket = userConfigBucketName
		key = fmt.Sprintf("%d%s", wallet.ID, key)
	}
	return wallet.db.Delete(bucket, key)
}

// SaveWalletConfigValue stores a generic value against the provided key
// at the assets manager level.
func (wallet *Wallet) SaveWalletConfigValue(key string, value interface{}) {
	err := wallet.walletConfigSave(true, key, value)
	if err != nil {
		log.Errorf("error setting wallet config value for key: %s, error: %v", key, err)
	}
}

// ReadWalletConfigValue reads a generic value stored against the provided key
// at the assets manager level.
func (wallet *Wallet) ReadWalletConfigValue(key string, valueOut interface{}) error {
	err := wallet.walletConfigRead(true, key, valueOut)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading wallet config value for key: %s, error: %v", key, err)
	}
	return err
}

// DeleteWalletConfigValue deletes the value associated with the provided key
// at the assets manager level.
func (wallet *Wallet) DeleteWalletConfigValue(key string) {
	err := wallet.walletConfigDelete(true, key)
	if err != nil {
		log.Errorf("error deleting wallet config value for key: %s, error: %v", key, err)
	}
}

// SaveUserConfigValue stores the generic value against the provided key
// at the asset level.
func (wallet *Wallet) SaveUserConfigValue(key string, value interface{}) {
	err := wallet.walletConfigSave(false, key, value)
	if err != nil {
		log.Errorf("error setting user config value for key: %s, error: %v", key, err)
	}
}

// ReadUserConfigValue reads the generic value stored against the provided
// key at the asset level.
func (wallet *Wallet) ReadUserConfigValue(key string, valueOut interface{}) error {
	err := wallet.walletConfigRead(false, key, valueOut)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading user config value for key: %s, error: %v", key, err)
	}
	return err
}

// DeleteUserConfigValueForKey method deletes the value stored against the provided
// key at the asset level.
func (wallet *Wallet) DeleteUserConfigValueForKey(key string) {
	err := wallet.walletConfigDelete(false, key)
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
