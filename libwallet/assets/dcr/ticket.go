package dcr

import (
	"context"
	"fmt"
	"runtime/trace"
	"sync"
	"time"

	"decred.org/dcrwallet/v4/errors"
	w "decred.org/dcrwallet/v4/wallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	vspd "github.com/decred/vspd/types/v2"
)

func (asset *Asset) TotalStakingRewards() (int64, error) {
	voteTransactions, err := asset.GetTransactionsRaw(0, 0, TxFilterVoted, true, "")
	if err != nil {
		return 0, err
	}

	var totalRewards int64
	for _, tx := range voteTransactions {
		totalRewards += tx.VoteReward
	}

	return totalRewards, nil
}

func (asset *Asset) TicketMaturity() int32 {
	return int32(asset.chainParams.TicketMaturity)
}

func (asset *Asset) TicketExpiry() int32 {
	return int32(asset.chainParams.TicketExpiry)
}

func (asset *Asset) StakingOverview() (stOverview *StakingOverview, err error) {
	stOverview = &StakingOverview{}

	stOverview.Voted, err = asset.CountTransactions(TxFilterVoted)
	if err != nil {
		return nil, err
	}

	stOverview.Revoked, err = asset.CountTransactions(TxFilterRevoked)
	if err != nil {
		return nil, err
	}

	stOverview.Live, err = asset.CountTransactions(TxFilterLive)
	if err != nil {
		return nil, err
	}

	stOverview.Immature, err = asset.CountTransactions(TxFilterImmature)
	if err != nil {
		return nil, err
	}

	stOverview.Expired, err = asset.CountTransactions(TxFilterExpired)
	if err != nil {
		return nil, err
	}

	stOverview.Unmined, err = asset.CountTransactions(TxFilterUnmined)
	if err != nil {
		return nil, err
	}

	stOverview.All = stOverview.Unmined + stOverview.Immature + stOverview.Live + stOverview.Voted +
		stOverview.Revoked + stOverview.Expired

	return stOverview, nil
}

// TicketPrice returns the price of a ticket for the next block, also known as
// the stake difficulty. May be incorrect if blockchain sync is ongoing or if
// blockchain is not up-to-date.
func (asset *Asset) TicketPrice() (*TicketPriceResponse, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	sdiff, err := asset.Internal().DCR.NextStakeDifficulty(ctx)
	if err != nil {
		return nil, err
	}

	_, tipHeight := asset.Internal().DCR.MainChainTip(ctx)
	resp := &TicketPriceResponse{
		TicketPrice: int64(sdiff),
		Height:      tipHeight,
	}
	return resp, nil
}

// PurchaseTickets purchases tickets from the asset.
// Returns a slice of hashes for tickets purchased.
func (asset *Asset) PurchaseTickets(account, numTickets int32, vspHost, passphrase string, vspPubKey []byte) ([]*chainhash.Hash, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	vspClient, err := asset.VSPClient(account, vspHost, vspPubKey)
	if err != nil {
		return nil, fmt.Errorf("VSP Server instance failed to start: %v", err)
	}

	networkBackend, err := asset.Internal().DCR.NetworkBackend()
	if err != nil {
		return nil, err
	}

	err = asset.UnlockWallet(passphrase)
	if err != nil {
		return nil, utils.TranslateError(err)
	}
	defer asset.LockWallet()

	request := &w.PurchaseTicketsRequest{
		Count:                int(numTickets),
		SourceAccount:        uint32(account),
		MinConf:              asset.RequiredConfirmations(),
		VSPFeePercent:        vspClient.FeePercentage,
		VSPFeePaymentProcess: vspClient.Process,

		// VotingAccount used to derive addresses for specifying voting rights.
		// It is used when VotingAddress == nil, or Mixing == true
		VotingAccount: uint32(account),
	}

	csppCfg := asset.readCSPPConfig()
	if csppCfg == nil {
		return nil, utils.ErrStakingAccountsMissing
	}

	// Mixed split buying through CoinShuffle++, if configured.
	request.Mixing = csppCfg.Mixing
	request.MixedAccount = csppCfg.MixedAccount
	request.MixedAccountBranch = csppCfg.MixedAccountBranch
	request.ChangeAccount = csppCfg.ChangeAccount
	request.MixedSplitAccount = csppCfg.TicketSplitAccount

	ctx, _ := asset.ShutdownContextWithCancel()
	ticketsResponse, err := asset.Internal().DCR.PurchaseTickets(ctx, networkBackend, request)
	if err != nil {
		return nil, err
	}

	return ticketsResponse.TicketHashes, err
}

// VSPTicketInfo returns vsp-related info for a given ticket. Returns an error
// if the ticket is not yet assigned to a VSP.
func (asset *Asset) VSPTicketInfo(hash string) (*VSPTicketInfo, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	// Cannot query an VSPTicketInfo api if the current instance wallet is locked.
	if asset.IsLocked() {
		log.Warnf("cannot query any ticket info when the wallet is locked")
		return nil, errors.New(utils.ErrWalletLocked)
	}

	ticketHash, err := chainhash.NewHashFromStr(hash)
	if err != nil {
		return nil, err
	}

	// Read the VSP info for this ticket from the wallet db.
	ctx, _ := asset.ShutdownContextWithCancel()
	ticket, err := asset.Internal().DCR.NewVSPTicket(ctx, ticketHash)
	if err != nil {
		return nil, err
	}

	walletTicketInfo, err := ticket.VSPTicketInfo(ctx)
	if err != nil {
		log.Warnf("unable to getWallet info using ticket: %s Error: %v", hash, err)
		return nil, err
	}

	ticketInfo := &VSPTicketInfo{
		VSP:         walletTicketInfo.Host,
		FeeTxHash:   walletTicketInfo.FeeHash.String(),
		FeeTxStatus: VSPFeeStatus(walletTicketInfo.FeeTxStatus),
	}

	// Account being set to -1 means the default ticket purchase account will be
	// used in the ticket policy configuration.
	vspClient, err := asset.VSPClient(-1, walletTicketInfo.Host, walletTicketInfo.PubKey)
	if err != nil {
		log.Warnf("unable to connect to host: %s Error: %v", walletTicketInfo.Host, err)
		return ticketInfo, nil
	}

	req := vspd.TicketStatusRequest{
		TicketHash: ticket.Hash().String(),
	}

	vspTicketStatus, err := vspClient.TicketStatus(ctx, req, ticket.CommitmentAddr())
	if err != nil {
		log.Warnf("unable to get vsp ticket: %s Error: %v", hash, err)
		return ticketInfo, nil
	}

	// Sanity check and log any observed discrepancies.
	if ticketInfo.FeeTxHash != vspTicketStatus.FeeTxHash {
		log.Warnf("wallet fee tx hash %s differs from vsp fee tx hash %s for ticket %s",
			ticketInfo.FeeTxHash, vspTicketStatus.FeeTxHash, ticketHash)
	}

	ticketInfo.VSPTicket = ticket
	ticketInfo.Client = vspClient
	ticketInfo.FeeTxHash = vspTicketStatus.FeeTxHash
	ticketInfo.ConfirmedByVSP = vspTicketStatus.TicketConfirmed

	return ticketInfo, nil
}

// StartTicketBuyer starts the automatic ticket buyer. The wallet
// should already be configured with the required parameters using
// asset.SetAutoTicketsBuyerConfig().
func (asset *Asset) StartTicketBuyer(passphrase string) error {
	if !asset.WalletOpened() {
		return utils.ErrDCRNotInitialized
	}

	// The default value (-1) will only be returned if the cpp staking
	// accounts are missing.
	if asset.MixedAccountNumber() == -1 || asset.UnmixedAccountNumber() == -1 {
		return utils.ErrStakingAccountsMissing
	}

	cfg := asset.AutoTicketsBuyerConfig()
	if cfg.VspHost == "" {
		return errors.New("ticket buyer config not set for this wallet")
	}
	if cfg.BalanceToMaintain < 0 {
		return errors.New("Negative balance to maintain in ticket buyer config")
	}

	if asset.IsAutoTicketsPurchaseActive() {
		return errors.New("Ticket buyer already running")
	}

	// Validate the passphrase.
	if len(passphrase) > 0 && asset.IsLocked() {
		err := asset.UnlockWallet(passphrase)
		if err != nil {
			return utils.TranslateError(err)
		}
	}

	ctx, cancel := asset.ShutdownContextWithCancel()
	asset.cancelAutoTicketBuyerMu.Lock()
	asset.cancelAutoTicketBuyer = cancel
	asset.cancelAutoTicketBuyerMu.Unlock()

	// Check the VSP.
	vspInfo, err := vspInfo(cfg.VspHost)
	if err != nil {
		return fmt.Errorf("error setting up vsp client: %v", err)
	}

	cfg.VspClient, err = asset.VSPClient(cfg.PurchaseAccount, cfg.VspHost, vspInfo.PubKey)
	if err != nil {
		log.Errorf("[%d] VSP Client instance failed error: %v", asset.ID, err)
		return errors.New("VSP Client failed to start due to incorrect configuration")
	}

	go func() {
		log.Infof("[%d] Running ticket buyer", asset.ID)

		if err = asset.runTicketBuyer(ctx, passphrase, cfg); err != nil {
			if ctx.Err() != nil {
				log.Errorf("[%d] Ticket buyer instance canceled", asset.ID)
			} else {
				log.Errorf("[%d] Ticket buyer instance errored: %v", asset.ID, err)
			}
		}

		if err = asset.StopAutoTicketsPurchase(); err != nil {
			log.Errorf("[%d] Stopping auto ticket purchase errored: %v", asset.ID, err)
		}
	}()

	return nil
}

// runTicketBuyer executes the ticket buyer. If the private passphrase is
// incorrect, or ever becomes incorrect due to a wallet passphrase change,
// runTicketBuyer exits with an errors.Passphrase error.
func (asset *Asset) runTicketBuyer(ctx context.Context, passphrase string, cfg *TicketBuyerConfig) error {
	if len(passphrase) > 0 && asset.IsLocked() {
		err := asset.UnlockWallet(passphrase)
		if err != nil {
			return utils.TranslateError(err)
		}
	}

	c := asset.Internal().DCR.NtfnServer.MainTipChangedNotifications()
	defer c.Done()

	ctx, outerCancel := context.WithCancel(ctx)
	defer outerCancel()
	var fatal error
	var fatalMu sync.Mutex

	var nextIntervalStart, expiry int32
	var cancels []func()
	for {
		select {
		case <-ctx.Done():
			defer outerCancel()
			fatalMu.Lock()
			err := fatal
			fatalMu.Unlock()
			if err != nil {
				return err
			}
			return ctx.Err()
		case n := <-c.C:
			if len(n.AttachedBlocks) == 0 {
				continue
			}

			tip := n.AttachedBlocks[len(n.AttachedBlocks)-1]
			w := asset.Internal().DCR

			// Don't perform any actions while transactions are not synced through
			// the tip block.
			rp, err := w.RescanPoint(ctx)
			if err != nil {
				return err
			}
			if rp != nil {
				log.Debugf("[%d] Skipping autobuyer actions: transactions are not synced", asset.ID)
				continue
			}

			tipHeader, err := w.BlockHeader(ctx, tip)
			if err != nil {
				log.Error(err)
				continue
			}
			height := int32(tipHeader.Height)

			// Cancel any ongoing ticket purchases which are buying
			// at an old ticket price or are no longer able to
			// create mined tickets the window.
			if height+2 >= nextIntervalStart {
				for i, cancel := range cancels {
					cancel()
					cancels[i] = nil
				}
				cancels = cancels[:0]

				intervalSize := int32(w.ChainParams().StakeDiffWindowSize)
				currentInterval := height / intervalSize
				nextIntervalStart = (currentInterval + 1) * intervalSize

				// Skip this purchase when no more tickets may be purchased in the interval and
				// the next sdiff is unknown.  The earliest any ticket may be mined is two
				// blocks from now, with the next block containing the split transaction
				// that the ticket purchase spends.
				if height+2 == nextIntervalStart {
					log.Debugf("[%d] Skipping purchase: next sdiff interval starts soon", asset.ID)
					continue
				}
				// Set expiry to prevent tickets from being mined in the next
				// sdiff interval.  When the next block begins the new interval,
				// the ticket is being purchased for the next interval; therefore
				// increment expiry by a full sdiff window size to prevent it
				// being mined in the interval after the next.
				expiry = nextIntervalStart
				if height+1 == nextIntervalStart {
					expiry += intervalSize
				}
			}

			// Get the account balance to determine how many tickets to buy
			bal, err := asset.GetAccountBalance(cfg.PurchaseAccount)
			if err != nil {
				return err
			}

			spendable := bal.Spendable.ToInt()
			if spendable < cfg.BalanceToMaintain {
				log.Debugf("[%d] Skipping purchase: low available balance", asset.ID)
				continue
			}

			spendable -= cfg.BalanceToMaintain
			sdiff, err := asset.Internal().DCR.NextStakeDifficultyAfterHeader(ctx, tipHeader)
			if err != nil {
				return err
			}

			buy := int(dcrutil.Amount(spendable) / sdiff)
			if buy == 0 {
				log.Debugf("[%d] Skipping purchase: low available balance", asset.ID)
				continue
			}

			cancelCtx, cancel := context.WithCancel(ctx)
			cancels = append(cancels, cancel)
			buyTicket := func() {
				err := asset.buyTicket(cancelCtx, passphrase, sdiff, expiry, cfg)
				if err != nil {
					switch {
					// silence these errors
					case errors.Is(err, errors.InsufficientBalance):
					case errors.Is(err, context.Canceled):
					case errors.Is(err, context.DeadlineExceeded):
					default:
						log.Errorf("[%d] Ticket purchasing failed: %v", asset.ID, err)
					}
					if errors.Is(err, errors.Passphrase) {
						fatalMu.Lock()
						fatal = err
						fatalMu.Unlock()
						outerCancel()
					}
				}
			}

			// start separate ticket purchase for as many tickets that can be purchased
			// each purchase only buy 1 ticket.
			for i := 0; i < buy; i++ {
				go buyTicket()
			}
		}
	}
}

// buyTicket purchases one ticket with the asset.
func (asset *Asset) buyTicket(ctx context.Context, passphrase string, sdiff dcrutil.Amount, expiry int32, cfg *TicketBuyerConfig) error {
	ctx, task := trace.NewTask(ctx, "ticketbuyer.buy")
	defer task.End()

	if len(passphrase) > 0 && asset.IsLocked() {
		err := asset.UnlockWallet(passphrase)
		if err != nil {
			return utils.TranslateError(err)
		}
	}

	networkBackend, err := asset.Internal().DCR.NetworkBackend()
	if err != nil {
		return err
	}

	if !asset.IsTicketBuyerAccountSet() {
		return utils.ErrTicketPurchaseAccMissing
	}

	// Count is 1 to prevent combining multiple split outputs in one tx,
	// which can be used to link the tickets eventually purchased with the
	// split outputs.
	request := &w.PurchaseTicketsRequest{
		Count:                1,
		SourceAccount:        uint32(cfg.PurchaseAccount),
		Expiry:               expiry,
		MinConf:              asset.RequiredConfirmations(),
		VSPFeePercent:        cfg.VspClient.FeePercentage,
		VSPFeePaymentProcess: cfg.VspClient.Process,

		// VotingAccount used to derive addresses for specifying voting rights.
		// It is used when VotingAddress == nil, or Mixing == true
		VotingAccount: uint32(cfg.PurchaseAccount),
	}

	csppCfg := asset.readCSPPConfig()
	if csppCfg == nil {
		return utils.ErrStakingAccountsMissing
	}

	// Mixed split buying through CoinShuffle++, if configured.
	request.Mixing = csppCfg.Mixing
	request.MixedAccount = csppCfg.MixedAccount
	request.MixedAccountBranch = csppCfg.MixedAccountBranch
	request.ChangeAccount = csppCfg.ChangeAccount
	request.MixedSplitAccount = csppCfg.TicketSplitAccount

	tix, err := asset.Internal().DCR.PurchaseTickets(ctx, networkBackend, request)
	if tix != nil {
		for _, hash := range tix.TicketHashes {
			log.Infof("[%d] Purchased ticket %v at stake difficulty %v", asset.ID, hash, sdiff)
		}
	}

	return err
}

// IsAutoTicketsPurchaseActive returns true if ticket buyer is active.
func (asset *Asset) IsAutoTicketsPurchaseActive() bool {
	asset.cancelAutoTicketBuyerMu.Lock()
	defer asset.cancelAutoTicketBuyerMu.Unlock()
	return asset.cancelAutoTicketBuyer != nil
}

// StopAutoTicketsPurchase stops the automatic ticket buyer.
func (asset *Asset) StopAutoTicketsPurchase() error {
	asset.cancelAutoTicketBuyerMu.Lock()
	defer asset.cancelAutoTicketBuyerMu.Unlock()

	if asset.cancelAutoTicketBuyer == nil {
		return errors.New(utils.ErrInvalid)
	}

	asset.cancelAutoTicketBuyer()
	asset.cancelAutoTicketBuyer = nil
	return nil
}

// SetAutoTicketsBuyerConfig sets ticket buyer config for the asset.
func (asset *Asset) SetAutoTicketsBuyerConfig(vspHost string, purchaseAccount int32, amountToMaintain int64) {
	asset.SetLongConfigValueForKey(sharedW.TicketBuyerATMConfigKey, amountToMaintain)
	asset.SetInt32ConfigValueForKey(sharedW.TicketBuyerAccountConfigKey, purchaseAccount)
	asset.SetStringConfigValueForKey(sharedW.TicketBuyerVSPHostConfigKey, vspHost)
}

// AutoTicketsBuyerConfig returns the previously set ticket buyer config for
// the asset.
func (asset *Asset) AutoTicketsBuyerConfig() *TicketBuyerConfig {
	btm := asset.ReadLongConfigValueForKey(sharedW.TicketBuyerATMConfigKey, -1)
	accNum := asset.ReadInt32ConfigValueForKey(sharedW.TicketBuyerAccountConfigKey, -1)
	vspHost := asset.ReadStringConfigValueForKey(sharedW.TicketBuyerVSPHostConfigKey, "")

	return &TicketBuyerConfig{
		VspHost:           vspHost,
		PurchaseAccount:   accNum,
		BalanceToMaintain: btm,
	}
}

// TicketBuyerConfigIsSet checks if ticket buyer config is set for the asset.
func (asset *Asset) TicketBuyerConfigIsSet() bool {
	return asset.ReadStringConfigValueForKey(sharedW.TicketBuyerVSPHostConfigKey, "") != ""
}

// IsTicketBuyerAccountSet checks if ticket buyer account is set for the asset.
func (asset *Asset) IsTicketBuyerAccountSet() bool {
	return asset.ReadInt32ConfigValueForKey(sharedW.TicketBuyerAccountConfigKey, -1) != -1
}

// ClearTicketBuyerConfig clears the wallet's ticket buyer config.
func (asset *Asset) ClearTicketBuyerConfig(_ int) error {
	asset.SetLongConfigValueForKey(sharedW.TicketBuyerATMConfigKey, -1)
	asset.SetInt32ConfigValueForKey(sharedW.TicketBuyerAccountConfigKey, -1)
	asset.SetStringConfigValueForKey(sharedW.TicketBuyerVSPHostConfigKey, "")

	return nil
}

// NextTicketPriceRemaining returns the remaning time in seconds of a ticket for the next block,
// if secs equal 0 is imminent
func (asset *Asset) NextTicketPriceRemaining() (secs int64, err error) {
	params, er := utils.DCRChainParams(asset.NetType())
	if er != nil {
		secs, err = -1, er
		return
	}
	bestBestBlock := asset.GetBestBlock()
	idxBlockInWindow := int(int64(bestBestBlock.Height)%params.StakeDiffWindowSize) + 1
	blockTime := params.TargetTimePerBlock.Nanoseconds()
	windowSize := params.StakeDiffWindowSize
	x := (windowSize - int64(idxBlockInWindow)) * blockTime
	if x == 0 {
		secs, err = 0, nil
		return
	}
	secs, err = int64(time.Duration(x).Seconds()), nil
	return
}

// UnspentUnexpiredTickets returns all Unmined, Immature and Live tickets.
func (asset *Asset) UnspentUnexpiredTickets() ([]*sharedW.Transaction, error) {
	var tickets []*sharedW.Transaction
	for _, filter := range []int32{TxFilterUnmined, TxFilterImmature, TxFilterLive} {
		tx, err := asset.GetTransactionsRaw(0, 0, filter, true, "")
		if err != nil {
			return nil, err
		}

		tickets = append(tickets, tx...)
	}

	return tickets, nil
}
