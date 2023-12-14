package wallet

import (
	"context"

	"github.com/crypto-power/cryptopower/libwallet/internal/loader"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

// Asset defines the interface each wallet must satisfy.
type Asset interface {
	Shutdown()
	IsSynced() bool
	IsSyncing() bool
	SpvSync() error
	CancelRescan()
	CancelSync()
	IsRescanning() bool
	RescanBlocks() error
	ConnectedPeers() int32
	RemovePeers()
	SetSpecificPeer(address string)
	GetExtendedPubKey(account int32) (string, error)
	IsSyncShuttingDown() bool

	LockWallet()
	IsLocked() bool
	IsWaiting() bool
	WalletOpened() bool
	OpenWallet() error
	GetWalletID() int
	GetWalletName() string
	IsWatchingOnlyWallet() bool
	UnlockWallet(string) error
	DeleteWallet(privPass string) error
	RenameWallet(newName string) error
	DecryptSeed(privatePassphrase string) (string, error)
	VerifySeedForWallet(seedMnemonic, privpass string) (bool, error)
	ChangePrivatePassphraseForWallet(oldPrivatePassphrase, newPrivatePassphrase string, privatePassphraseType int32) error

	RootDir() string
	DataDir() string
	GetEncryptedSeed() string
	IsConnectedToNetwork() bool
	NetType() utils.NetworkType
	ToAmount(v int64) AssetAmount
	GetAssetType() utils.AssetType
	Internal() *loader.LoadedWallets
	TargetTimePerBlockMinutes() float64
	RequiredConfirmations() int32
	ShutdownContextWithCancel() (context.Context, context.CancelFunc)
	LogFile() string

	PublishUnminedTransactions() error
	CountTransactions(txFilter int32) (int, error)
	GetTransactionRaw(txHash string) (*Transaction, error)
	TxMatchesFilter(tx *Transaction, txFilter int32) bool
	GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool, txHashSearch string) ([]*Transaction, error)

	GetBestBlock() *BlockInfo
	GetBestBlockHeight() int32
	GetBestBlockTimeStamp() int64

	ContainsDiscoveredAccounts() bool
	GetAccountsRaw() (*Accounts, error)
	GetAccount(accountNumber int32) (*Account, error)
	AccountName(accountNumber int32) (string, error)
	CreateNewAccount(accountName, privPass string) (int32, error)
	RenameAccount(accountNumber int32, newName string) error
	AccountNumber(accountName string) (int32, error)
	AccountNameRaw(accountNumber uint32) (string, error)
	GetAccountBalance(accountNumber int32) (*Balance, error)
	GetWalletBalance() (*Balance, error)
	UnspentOutputs(account int32) ([]*UnspentOutput, error)

	AddSyncProgressListener(syncProgressListener *SyncProgressListener, uniqueIdentifier string) error
	RemoveSyncProgressListener(uniqueIdentifier string)
	AddTxAndBlockNotificationListener(txAndBlockNotificationListener *TxAndBlockNotificationListener, uniqueIdentifier string) error
	RemoveTxAndBlockNotificationListener(uniqueIdentifier string)
	SetBlocksRescanProgressListener(blocksRescanProgressListener *BlocksRescanProgressListener)

	CurrentAddress(account int32) (string, error)
	NextAddress(account int32) (string, error)
	IsAddressValid(address string) bool
	HaveAddress(address string) bool

	SignMessage(passphrase, address, message string) ([]byte, error)
	VerifyMessage(address, message, signatureBase64 string) (bool, error)

	SaveUserConfigValue(key string, value interface{})
	ReadUserConfigValue(key string, valueOut interface{}) error

	SetBoolConfigValueForKey(key string, value bool)
	SetDoubleConfigValueForKey(key string, value float64)
	SetIntConfigValueForKey(key string, value int)
	SetInt32ConfigValueForKey(key string, value int32)
	SetLongConfigValueForKey(key string, value int64)
	SetStringConfigValueForKey(key, value string)

	ReadBoolConfigValueForKey(key string, defaultValue bool) bool
	ReadDoubleConfigValueForKey(key string, defaultValue float64) float64
	ReadIntConfigValueForKey(key string, defaultValue int) int
	ReadInt32ConfigValueForKey(key string, defaultValue int32) int32
	ReadLongConfigValueForKey(key string, defaultValue int64) int64
	ReadStringConfigValueForKey(key string, defaultValue string) string

	NewUnsignedTx(accountNumber int32, utxos []*UnspentOutput) error
	AddSendDestination(address string, unitAmount int64, sendMax bool) error
	ComputeTxSizeEstimation(dstAddress string, utxos []*UnspentOutput) (int, error)
	Broadcast(passphrase, label string) ([]byte, error)
	EstimateFeeAndSize() (*TxFeeAndSize, error)
	IsUnsignedTxExist() bool
}
