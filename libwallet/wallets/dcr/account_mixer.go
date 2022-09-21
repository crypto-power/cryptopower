package dcr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"decred.org/dcrwallet/v2/ticketbuyer"
	w "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/udb"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/internal/certs"
)

const (
	smalletSplitPoint  = 000.00262144
	ShuffleServer      = "mix.decred.org"
	MainnetShufflePort = "5760"
	TestnetShufflePort = "15760"
	MixedAccountBranch = int32(udb.ExternalBranch)
)

func (wallet *Wallet) AddAccountMixerNotificationListener(accountMixerNotificationListener AccountMixerNotificationListener, uniqueIdentifier string) error {
	wallet.notificationListenersMu.Lock()
	defer wallet.notificationListenersMu.Unlock()

	if _, ok := wallet.accountMixerNotificationListener[uniqueIdentifier]; ok {
		return errors.New(ErrListenerAlreadyExist)
	}

	wallet.accountMixerNotificationListener[uniqueIdentifier] = accountMixerNotificationListener
	return nil
}

func (wallet *Wallet) RemoveAccountMixerNotificationListener(uniqueIdentifier string) {
	wallet.notificationListenersMu.Lock()
	defer wallet.notificationListenersMu.Unlock()

	delete(wallet.accountMixerNotificationListener, uniqueIdentifier)
}

// CreateMixerAccounts creates the two accounts needed for the account mixer. This function
// is added to ease unlocking the wallet before creating accounts. This function should be
// used with auto cspp mixer setup.
func (wallet *Wallet) CreateMixerAccounts(mixedAccount, unmixedAccount, privPass string) error {
	accountMixerConfigSet := wallet.ReadBoolConfigValueForKey(AccountMixerConfigSet, false)
	if accountMixerConfigSet {
		return errors.New(ErrInvalid)
	}

	if wallet.HasAccount(mixedAccount) || wallet.HasAccount(unmixedAccount) {
		return errors.New(ErrExist)
	}

	err := wallet.UnlockWallet([]byte(privPass))
	if err != nil {
		return err
	}

	defer wallet.LockWallet()

	mixedAccountNumber, err := wallet.NextAccount(mixedAccount)
	if err != nil {
		return err
	}

	unmixedAccountNumber, err := wallet.NextAccount(unmixedAccount)
	if err != nil {
		return err
	}

	wallet.SetInt32ConfigValueForKey(AccountMixerMixedAccount, mixedAccountNumber)
	wallet.SetInt32ConfigValueForKey(AccountMixerUnmixedAccount, unmixedAccountNumber)
	wallet.SetBoolConfigValueForKey(AccountMixerConfigSet, true)

	return nil
}

// SetAccountMixerConfig sets the config for mixed and unmixed account. Private passphrase is verifed
// for security even if not used. This function should be used with manual cspp mixer setup.
func (wallet *Wallet) SetAccountMixerConfig(mixedAccount, unmixedAccount int32, privPass string) error {

	if mixedAccount == unmixedAccount {
		return errors.New(ErrInvalid)
	}

	// Verify that account numbers are correct
	_, err := wallet.GetAccount(mixedAccount)
	if err != nil {
		return errors.New(ErrNotExist)
	}

	_, err = wallet.GetAccount(unmixedAccount)
	if err != nil {
		return errors.New(ErrNotExist)
	}

	err = wallet.UnlockWallet([]byte(privPass))
	if err != nil {
		return err
	}
	wallet.LockWallet()

	wallet.SetInt32ConfigValueForKey(AccountMixerMixedAccount, mixedAccount)
	wallet.SetInt32ConfigValueForKey(AccountMixerUnmixedAccount, unmixedAccount)
	wallet.SetBoolConfigValueForKey(AccountMixerConfigSet, true)

	return nil
}

func (wallet *Wallet) AccountMixerMixChange() bool {
	return wallet.ReadBoolConfigValueForKey(AccountMixerMixTxChange, false)
}

func (wallet *Wallet) AccountMixerConfigIsSet() bool {
	return wallet.ReadBoolConfigValueForKey(AccountMixerConfigSet, false)
}

func (wallet *Wallet) MixedAccountNumber() int32 {
	return wallet.ReadInt32ConfigValueForKey(AccountMixerMixedAccount, -1)
}

func (wallet *Wallet) UnmixedAccountNumber() int32 {
	return wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)
}

func (wallet *Wallet) ClearMixerConfig() {
	wallet.SetInt32ConfigValueForKey(AccountMixerMixedAccount, -1)
	wallet.SetInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)
	wallet.SetBoolConfigValueForKey(AccountMixerConfigSet, false)
}

func (wallet *Wallet) ReadyToMix(walletID int) (bool, error) {
	if wallet == nil {
		return false, errors.New(ErrNotExist)
	}

	unmixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)

	hasMixableOutput, err := wallet.accountHasMixableOutput(unmixedAccount)
	if err != nil {
		return false, translateError(err)
	}

	return hasMixableOutput, nil
}

// StartAccountMixer starts the automatic account mixer
func (wallet *Wallet) StartAccountMixer(walletPassphrase string) error {
	if !wallet.IsConnectedToDecredNetwork() {
		return errors.New(ErrNotConnected)
	}

	if wallet == nil {
		return errors.New(ErrNotExist)
	}

	cfg := wallet.readCSPPConfig()
	if cfg == nil {
		return errors.New(ErrFailedPrecondition)
	}

	hasMixableOutput, err := wallet.accountHasMixableOutput(int32(cfg.ChangeAccount))
	if err != nil {
		return translateError(err)
	} else if !hasMixableOutput {
		return errors.New(ErrNoMixableOutput)
	}

	tb := ticketbuyer.New(wallet.Internal())
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

	err = wallet.UnlockWallet([]byte(walletPassphrase))
	if err != nil {
		return translateError(err)
	}

	go func() {
		log.Info("Running account mixer")
		if wallet.accountMixerNotificationListener != nil {
			wallet.publishAccountMixerStarted(wallet.ID)
		}

		ctx, cancel := wallet.contextWithShutdownCancel()
		wallet.CancelAccountMixer = cancel
		err = tb.Run(ctx, []byte(walletPassphrase))
		if err != nil {
			log.Errorf("AccountMixer instance errored: %v", err)
		}

		wallet.CancelAccountMixer = nil
		if wallet.accountMixerNotificationListener != nil {
			wallet.publishAccountMixerEnded(wallet.ID)
		}
	}()

	return nil
}

func (wallet *Wallet) readCSPPConfig() *CSPPConfig {
	mixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerMixedAccount, -1)
	unmixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)

	if mixedAccount == -1 || unmixedAccount == -1 {
		// not configured for mixing
		return nil
	}

	var shufflePort = TestnetShufflePort
	var dialCSPPServer func(ctx context.Context, network, addr string) (net.Conn, error)
	if wallet.chainParams.Net == chaincfg.MainNetParams().Net {
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
func (wallet *Wallet) StopAccountMixer() error {
	if wallet == nil {
		return errors.New(ErrNotExist)
	}

	if wallet.CancelAccountMixer == nil {
		return errors.New(ErrInvalid)
	}

	wallet.CancelAccountMixer()
	wallet.CancelAccountMixer = nil
	return nil
}

func (wallet *Wallet) accountHasMixableOutput(accountNumber int32) (bool, error) {

	policy := w.OutputSelectionPolicy{
		Account:               uint32(accountNumber),
		RequiredConfirmations: wallet.RequiredConfirmations(),
	}

	// fetch all utxos in account to extract details for the utxos selected by user
	// use targetAmount = 0 to fetch ALL utxos in account
	inputDetail, err := wallet.Internal().SelectInputs(wallet.ShutdownContext(), dcrutil.Amount(0), policy)
	if err != nil {
		return false, nil
	}

	hasMixableOutput := false
	for _, input := range inputDetail.Inputs {
		if AmountCoin(input.ValueIn) > smalletSplitPoint {
			hasMixableOutput = true
			break
		}
	}

	if !hasMixableOutput {
		accountName, err := wallet.AccountName(accountNumber)
		if err != nil {
			return hasMixableOutput, nil
		}

		lockedOutpoints, err := wallet.Internal().LockedOutpoints(wallet.ShutdownContext(), accountName)
		if err != nil {
			return hasMixableOutput, nil
		}
		hasMixableOutput = len(lockedOutpoints) > 0
	}

	return hasMixableOutput, nil
}

// IsAccountMixerActive returns true if account mixer is active
func (wallet *Wallet) IsAccountMixerActive() bool {
	return wallet.CancelAccountMixer != nil
}

func (wallet *Wallet) publishAccountMixerStarted(walletID int) {
	wallet.notificationListenersMu.RLock()
	defer wallet.notificationListenersMu.RUnlock()

	for _, accountMixerNotificationListener := range wallet.accountMixerNotificationListener {
		accountMixerNotificationListener.OnAccountMixerStarted(walletID)
	}
}

func (wallet *Wallet) publishAccountMixerEnded(walletID int) {
	wallet.notificationListenersMu.RLock()
	defer wallet.notificationListenersMu.RUnlock()

	for _, accountMixerNotificationListener := range wallet.accountMixerNotificationListener {
		accountMixerNotificationListener.OnAccountMixerEnded(walletID)
	}
}
