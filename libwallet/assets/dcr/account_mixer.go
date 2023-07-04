package dcr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"decred.org/dcrwallet/v3/ticketbuyer"
	w "decred.org/dcrwallet/v3/wallet"
	"decred.org/dcrwallet/v3/wallet/udb"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/internal/certs"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
)

const (
	smalletSplitPoint  = 000.00262144
	ShuffleServer      = "mix.decred.org"
	MainnetShufflePort = "5760"
	TestnetShufflePort = "15760"
	MixedAccountBranch = int32(udb.ExternalBranch)
)

func (asset *DCRAsset) AddAccountMixerNotificationListener(accountMixerNotificationListener AccountMixerNotificationListener, uniqueIdentifier string) error {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	if _, ok := asset.accountMixerNotificationListener[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.accountMixerNotificationListener[uniqueIdentifier] = accountMixerNotificationListener
	return nil
}

func (asset *DCRAsset) RemoveAccountMixerNotificationListener(uniqueIdentifier string) {
	asset.notificationListenersMu.Lock()
	defer asset.notificationListenersMu.Unlock()

	delete(asset.accountMixerNotificationListener, uniqueIdentifier)
}

// CreateMixerAccounts creates the two accounts needed for the account mixer. This function
// is added to ease unlocking the wallet before creating accounts. This function should be
// used with auto cspp mixer setup.
func (asset *DCRAsset) CreateMixerAccounts(mixedAccount, unmixedAccount, privPass string) error {
	accountMixerConfigSet := asset.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false)
	if accountMixerConfigSet {
		return errors.New(utils.ErrInvalid)
	}

	if asset.HasAccount(mixedAccount) || asset.HasAccount(unmixedAccount) {
		return errors.New(utils.ErrExist)
	}

	err := asset.UnlockWallet(privPass)
	if err != nil {
		return err
	}

	defer asset.LockWallet()

	mixedAccountNumber, err := asset.NextAccount(mixedAccount)
	if err != nil {
		return err
	}

	unmixedAccountNumber, err := asset.NextAccount(unmixedAccount)
	if err != nil {
		return err
	}

	asset.SetInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, mixedAccountNumber)
	asset.SetInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, unmixedAccountNumber)
	asset.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, true)

	return nil
}

// SetAccountMixerConfig sets the config for mixed and unmixed account. Private passphrase is verifed
// for security even if not used. This function should be used with manual cspp mixer setup.
func (asset *DCRAsset) SetAccountMixerConfig(mixedAccount, unmixedAccount int32, privPass string) error {
	if mixedAccount == unmixedAccount {
		return errors.New(utils.ErrInvalid)
	}

	// Verify that account numbers are correct
	_, err := asset.GetAccount(mixedAccount)
	if err != nil {
		return errors.New(utils.ErrNotExist)
	}

	_, err = asset.GetAccount(unmixedAccount)
	if err != nil {
		return errors.New(utils.ErrNotExist)
	}

	err = asset.UnlockWallet(privPass)
	if err != nil {
		return err
	}
	asset.LockWallet()

	asset.SetInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, mixedAccount)
	asset.SetInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, unmixedAccount)
	asset.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, true)

	return nil
}

func (asset *DCRAsset) AccountMixerMixChange() bool {
	return asset.ReadBoolConfigValueForKey(sharedW.AccountMixerMixTxChange, false)
}

func (asset *DCRAsset) AccountMixerConfigIsSet() bool {
	return asset.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false)
}

func (asset *DCRAsset) MixedAccountNumber() int32 {
	return asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, -1)
}

func (asset *DCRAsset) UnmixedAccountNumber() int32 {
	return asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)
}

func (asset *DCRAsset) ClearMixerConfig() {
	asset.SetInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, -1)
	asset.SetInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)
	asset.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false)
}

func (asset *DCRAsset) ReadyToMix(walletID int) (bool, error) {
	if asset == nil {
		return false, errors.New(utils.ErrNotExist)
	}

	unmixedAccount := asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)

	hasMixableOutput, err := asset.accountHasMixableOutput(unmixedAccount)
	if err != nil {
		return false, utils.TranslateError(err)
	}

	return hasMixableOutput, nil
}

// StartAccountMixer starts the automatic account mixer
func (asset *DCRAsset) StartAccountMixer(walletPassphrase string) error {
	if !asset.IsConnectedToDecredNetwork() {
		return errors.New(utils.ErrNotConnected)
	}

	if asset == nil {
		return errors.New(utils.ErrNotExist)
	}

	cfg := asset.readCSPPConfig()
	if cfg == nil {
		return errors.New(utils.ErrFailedPrecondition)
	}

	hasMixableOutput, err := asset.accountHasMixableOutput(int32(cfg.ChangeAccount))
	if err != nil {
		return utils.TranslateError(err)
	} else if !hasMixableOutput {
		return errors.New(utils.ErrNoMixableOutput)
	}

	tb := ticketbuyer.New(asset.Internal().DCR)
	tb.AccessConfig(func(c *ticketbuyer.Config) {
		c.MixedAccountBranch = cfg.MixedAccountBranch
		c.MixedAccount = cfg.MixedAccount
		c.ChangeAccount = cfg.ChangeAccount
		c.CSPPServer = cfg.CSPPServer
		c.DialCSPPServer = cfg.DialCSPPServer
		c.TicketSplitAccount = cfg.TicketSplitAccount
		c.BuyTickets = false
		c.MixChange = true
		// c.VotingAccount = 0 // TODO: VotingAccount should be configurable.
	})

	err = asset.UnlockWallet(walletPassphrase)
	if err != nil {
		return utils.TranslateError(err)
	}

	go func() {
		log.Info("Running account mixer")
		if asset.accountMixerNotificationListener != nil {
			asset.publishAccountMixerStarted(asset.ID)
		}

		ctx, cancel := asset.ShutdownContextWithCancel()
		asset.cancelAccountMixer = cancel
		err = tb.Run(ctx, []byte(walletPassphrase))
		if err != nil {
			log.Errorf("AccountMixer instance errored: %v", err)
		}

		asset.cancelAccountMixer = nil
		if asset.accountMixerNotificationListener != nil {
			asset.publishAccountMixerEnded(asset.ID)
		}
	}()

	return nil
}

func (asset *DCRAsset) readCSPPConfig() *CSPPConfig {
	mixedAccount := asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, -1)
	unmixedAccount := asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)

	if mixedAccount == -1 || unmixedAccount == -1 {
		// not configured for mixing
		return nil
	}

	shufflePort := TestnetShufflePort
	var dialCSPPServer func(ctx context.Context, network, addr string) (net.Conn, error)
	if asset.chainParams.Net == chaincfg.MainNetParams().Net {
		shufflePort = MainnetShufflePort

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(certs.CSPP))

		csppTLSConfig := new(tls.Config)
		csppTLSConfig.ServerName = ShuffleServer
		csppTLSConfig.RootCAs = pool

		dailer := new(net.Dialer)
		dialCSPPServer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dailer.DialContext(context.Background(), network, addr)
			if err != nil {
				return nil, err
			}

			conn = tls.Client(conn, csppTLSConfig)
			return conn, nil
		}
	}

	return &CSPPConfig{
		CSPPServer:         ShuffleServer + ":" + shufflePort,
		DialCSPPServer:     dialCSPPServer,
		MixedAccount:       uint32(mixedAccount),
		MixedAccountBranch: uint32(MixedAccountBranch),
		ChangeAccount:      uint32(unmixedAccount),
		TicketSplitAccount: uint32(mixedAccount), // upstream desc: Account to derive fresh addresses from for mixed ticket splits; uses mixedaccount if unset
	}
}

// StopAccountMixer stops the active account mixer
func (asset *DCRAsset) StopAccountMixer() error {
	if asset == nil {
		return errors.New(utils.ErrNotExist)
	}

	if asset.cancelAccountMixer == nil {
		return errors.New(utils.ErrInvalid)
	}

	asset.cancelAccountMixer()
	asset.cancelAccountMixer = nil
	return nil
}

func (asset *DCRAsset) accountHasMixableOutput(accountNumber int32) (bool, error) {
	policy := w.OutputSelectionPolicy{
		Account:               uint32(accountNumber),
		RequiredConfirmations: asset.RequiredConfirmations(),
	}

	// fetch all utxos in account to extract details for the utxos selected by user
	// use targetAmount = 0 to fetch ALL utxos in account
	ctx, _ := asset.ShutdownContextWithCancel()
	inputDetail, err := asset.Internal().DCR.SelectInputs(ctx, dcrutil.Amount(0), policy)
	if err != nil {
		return false, nil
	}

	hasMixableOutput := false
	for _, input := range inputDetail.Inputs {
		if asset.ToAmount(input.ValueIn).ToCoin() > smalletSplitPoint {
			hasMixableOutput = true
			break
		}
	}

	if !hasMixableOutput {
		accountName, err := asset.AccountName(accountNumber)
		if err != nil {
			return hasMixableOutput, nil
		}

		ctx, _ := asset.ShutdownContextWithCancel()
		lockedOutpoints, err := asset.Internal().DCR.LockedOutpoints(ctx, accountName)
		if err != nil {
			return hasMixableOutput, nil
		}
		hasMixableOutput = len(lockedOutpoints) > 0
	}

	return hasMixableOutput, nil
}

// IsAccountMixerActive returns true if account mixer is active
func (asset *DCRAsset) IsAccountMixerActive() bool {
	return asset.cancelAccountMixer != nil
}

func (asset *DCRAsset) publishAccountMixerStarted(walletID int) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, accountMixerNotificationListener := range asset.accountMixerNotificationListener {
		accountMixerNotificationListener.OnAccountMixerStarted(walletID)
	}
}

func (asset *DCRAsset) publishAccountMixerEnded(walletID int) {
	asset.notificationListenersMu.RLock()
	defer asset.notificationListenersMu.RUnlock()

	for _, accountMixerNotificationListener := range asset.accountMixerNotificationListener {
		accountMixerNotificationListener.OnAccountMixerEnded(walletID)
	}
}
