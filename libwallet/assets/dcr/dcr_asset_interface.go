package dcr

import (
	"context"

	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
)

// DCRUniqueAsset defines the extra interface each DCR wallet must also satisfy on top
// of satisfying the Asset interface.
type DCRUniqueAsset interface {
	IsAccountMixerActive() bool
	StopAccountMixer() error
	MixedAccountNumber() int32
	UnmixedAccountNumber() int32
	AccountMixerMixChange() bool
	AccountMixerConfigIsSet() bool
	StartAccountMixer(walletPassphrase string) error
	CreateMixerAccounts(mixedAccount, unmixedAccount, privPass string) error
	SetAccountMixerConfig(mixedAccount, unmixedAccount int32, privPass string) error

	StopAutoTicketsPurchase() error
	TotalStakingRewards() (int64, error)
	StakingOverview() (stOverview *StakingOverview, err error)
	VSPTicketInfo(hash string) (*VSPTicketInfo, error)
	StartTicketBuyer(passphrase string) error

	KnownVSPs() []*VSP
	ReloadVSPList(ctx context.Context)
	TicketBuyerConfigIsSet() bool
	IsAutoTicketsPurchaseActive() bool
	AutoTicketsBuyerConfig() *TicketBuyerConfig
	TicketPrice() (*TicketPriceResponse, error)
	NextTicketPriceRemaining() (secs int64, err error)
	SetAutoTicketsBuyerConfig(vspHost string, purchaseAccount int32, amountToMaintain int64)

	TicketExpiry() int32
	TicketMaturity() int32
	SaveVSP(host string) (err error)
	TicketHasVotedOrRevoked(ticketHash string) (bool, error)
	SetVoteChoice(agendaID, choiceID, hash, passphrase string) error
	TicketSpender(ticketHash string) (*sharedW.Transaction, error)

	AllVoteAgendas(hash string, newestFirst bool) ([]*Agenda, error)
	TreasuryPolicies(PiKey, tixHash string) ([]*TreasuryKeyPolicy, error)
	SetTreasuryPolicy(PiKey, newVotingPolicy, tixHash string, passphrase string) error
	AccountXPubMatches(account uint32, legacyXPub, slip044XPub string) (bool, error)

	AddAccountMixerNotificationListener(accountMixerNotificationListener AccountMixerNotificationListener, uniqueIdentifier string) error
	AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error
	AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error
	SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener)
	RemoveAccountMixerNotificationListener(uniqueIdentifier string)
	RemoveTxAndBlockNotificationListener(uniqueIdentifier string)
	RemoveSyncProgressListener(uniqueIdentifier string)

	GetUnsignedTx() *TxAuthor
	UseInputs(utxoKeys []string) error
	NewUnsignedTx(sourceAccountNumber int32) error
	Broadcast(privatePassphrase string) ([]byte, error)
	EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error)
	AddSendDestination(address string, atomAmount int64, sendMax bool) error
}
