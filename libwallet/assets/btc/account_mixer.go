package btc

import 	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"

const (
	smalletSplitPoint  = 000.00262144
	MixedAccountBranch = 0
)

func (asset *Asset) AddAccountMixerNotificationListener(accountMixerNotificationListener *AccountMixerNotificationListener, uniqueIdentifier string) error {
	return nil
}

func (asset *Asset) RemoveAccountMixerNotificationListener(uniqueIdentifier string) {
}

// CreateMixerAccounts creates the two accounts needed for the account mixer. This function
// is added to ease unlocking the wallet before creating accounts. This function should be
// used with auto cspp mixer setup.
func (asset *Asset) CreateMixerAccounts(mixedAccount, unmixedAccount, privPass string) error {
	return nil
}

// SetAccountMixerConfig sets the config for mixed and unmixed account. Private passphrase is verifed
// for security even if not used. This function should be used with manual cspp mixer setup.
func (asset *Asset) SetAccountMixerConfig(mixedAccount, unmixedAccount int32, privPass string) error {
	return nil
}

func (asset *Asset) AccountMixerMixChange() bool {
	return false
}

func (asset *Asset) AccountMixerConfigIsSet() bool {
	return false
}

func (asset *Asset) MixedAccountNumber() int32 {
	return -1
}

func (asset *Asset) UnmixedAccountNumber() int32 {
	return asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)
}

func (asset *Asset) ClearMixerConfig() {
}

func (asset *Asset) ReadyToMix(_ int) (bool, error) {
	return false, nil
}

// StartAccountMixer starts the automatic account mixer
func (asset *Asset) StartAccountMixer(walletPassphrase string) error {
	return nil
}

func (asset *Asset) readCSPPConfig() *CSPPConfig {
	mixedAccount := asset.MixedAccountNumber()
	unmixedAccount := asset.UnmixedAccountNumber()

	if mixedAccount == -1 || unmixedAccount == -1 {
		// not configured for mixing
		return nil
	}

	return &CSPPConfig{
		Mixing:             true,
		MixedAccount:       uint32(mixedAccount),
		MixedAccountBranch: uint32(MixedAccountBranch),
		ChangeAccount:      uint32(unmixedAccount),
		TicketSplitAccount: uint32(mixedAccount), // upstream desc: Account to derive fresh addresses from for mixed ticket splits; uses mixedaccount if unset
	}
}

// StopAccountMixer stops the active account mixer
func (asset *Asset) StopAccountMixer() error {
	return nil
}

func (asset *Asset) accountHasMixableOutput(accountNumber int32) bool {
	return false
}

// IsAccountMixerActive returns true if account mixer is active
func (asset *Asset) IsAccountMixerActive() bool {
	return false
}

func (asset *Asset) publishAccountMixerStarted(walletID int) {
}

func (asset *Asset) publishAccountMixerEnded(walletID int) {
}
