package dcr

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"decred.org/dcrwallet/v2/errors"

	"github.com/asdine/storm"
	tkv1 "github.com/decred/politeia/politeiawww/api/ticketvote/v1"
	www "github.com/decred/politeia/politeiawww/api/www/v1"
)

const (
	retryInterval = 15 // 15 seconds

	// VoteBitYes is the string value for identifying "yes" vote bits
	VoteBitYes = tkv1.VoteOptionIDApprove
	// VoteBitNo is the string value for identifying "no" vote bits
	VoteBitNo = tkv1.VoteOptionIDReject
)

// Sync fetches all proposals from the server and
func (p *Politeia) Sync() error {

	p.mu.Lock()

	if p.cancelSync != nil {
		p.mu.Unlock()
		return errors.New(ErrSyncAlreadyInProgress)
	}

	log.Info("Politeia sync: started")

	p.ctx, p.cancelSync = p.WalletRef.contextWithShutdownCancel()
	defer p.resetSyncData()

	p.mu.Unlock()

	for {
		_, err := p.getClient()
		if err != nil {
			log.Errorf("Error fetching for politeia server policy: %v", err)
			time.Sleep(retryInterval * time.Second)
			continue
		}

		if done(p.ctx) {
			return errors.New(ErrContextCanceled)
		}

		log.Info("Politeia sync: checking for updates")

		err = p.checkForUpdates()
		if err != nil {
			log.Errorf("Error checking for politeia updates: %v", err)
			time.Sleep(retryInterval * time.Second)
			continue
		}

		log.Info("Politeia sync: update complete")
		p.saveLastSyncedTimestamp(time.Now().Unix())
		p.publishSynced()
		return nil
	}
}

func (p *Politeia) GetLastSyncedTimeStamp() int64 {
	return p.getLastSyncedTimestamp()
}

func (p *Politeia) IsSyncing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cancelSync != nil
}

// this function requres p.mu unlocked.
func (p *Politeia) resetSyncData() {
	p.cancelSync = nil
}

func (p *Politeia) StopSync() {
	p.mu.Lock()
	if p.cancelSync != nil {
		p.cancelSync()
		p.resetSyncData()
	}
	p.mu.Unlock()
	log.Info("Politeia sync: stopped")
}

func (p *Politeia) checkForUpdates() error {
	offset := 0
	p.mu.RLock()
	limit := int32(p.Client.policy.ProposalListPageSize)
	p.mu.RUnlock()

	for {
		proposals, err := p.getProposalsRaw(ProposalCategoryAll, int32(offset), limit, true, true)
		if err != nil && err != storm.ErrNotFound {
			return err
		}

		if len(proposals) == 0 {
			break
		}

		offset += len(proposals)

		err = p.handleProposalsUpdate(proposals)
		if err != nil {
			return err
		}
	}

	// include abandoned proposals
	allProposals, err := p.getProposalsRaw(ProposalCategoryAll, 0, 0, true, false)
	if err != nil && err != storm.ErrNotFound {
		return err
	}

	err = p.handleNewProposals(allProposals)
	if err != nil {
		return err
	}

	return nil
}

func (p *Politeia) handleNewProposals(proposals []Proposal) error {
	loadedTokens := make([]string, len(proposals))
	for i := range proposals {
		loadedTokens[i] = proposals[i].Token
	}

	p.mu.RLock()
	tokenInventory, err := p.Client.tokenInventory()
	p.mu.RUnlock()
	if err != nil {
		return err
	}

	return p.fetchAllUnfetchedProposals(tokenInventory, loadedTokens)
}

func (p *Politeia) handleProposalsUpdate(proposals []Proposal) error {
	tokens := make([]string, len(proposals))
	for i := range proposals {
		tokens[i] = proposals[i].Token
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	batchProposals, err := p.Client.batchProposals(tokens)
	if err != nil {
		return err
	}

	batchVotesSummaries, err := p.Client.batchVoteSummary(tokens)
	if err != nil {
		return err
	}

	for i := range batchProposals {
		if voteSummary, ok := batchVotesSummaries[batchProposals[i].Token]; ok {
			batchProposals[i].VoteStatus = int32(voteSummary.Status)
			batchProposals[i].VoteApproved = voteSummary.Approved
			batchProposals[i].PassPercentage = int32(voteSummary.PassPercentage)
			batchProposals[i].EligibleTickets = int32(voteSummary.EligibleTickets)
			batchProposals[i].QuorumPercentage = int32(voteSummary.QuorumPercentage)
			batchProposals[i].YesVotes, batchProposals[i].NoVotes = getVotesCount(voteSummary.Results)
		}

		for k := range proposals {
			if proposals[k].Token == batchProposals[i].Token {

				// proposal category
				batchProposals[i].Category = proposals[k].Category
				err := p.updateProposalDetails(proposals[k], batchProposals[i])
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (p *Politeia) updateProposalDetails(oldProposal, updatedProposal Proposal) error {
	updatedProposal.ID = oldProposal.ID

	if reflect.DeepEqual(oldProposal, updatedProposal) {
		return nil
	}

	var callback func(*Proposal)

	if oldProposal.Status != updatedProposal.Status && www.PropStatusT(updatedProposal.Status) == www.PropStatusAbandoned {
		updatedProposal.Category = ProposalCategoryAbandoned
	} else if oldProposal.VoteStatus != updatedProposal.VoteStatus {
		switch www.PropVoteStatusT(updatedProposal.VoteStatus) {
		case www.PropVoteStatusFinished:
			callback = p.publishVoteFinished
			if updatedProposal.VoteApproved {
				updatedProposal.Category = ProposalCategoryApproved
			} else {
				updatedProposal.Category = ProposalCategoryRejected
			}
		case www.PropVoteStatusStarted:
			callback = p.publishVoteStarted
			updatedProposal.Category = ProposalCategoryActive
		default:
			updatedProposal.Category = ProposalCategoryPre
		}
	}

	err := p.WalletRef.db.Update(&updatedProposal)
	if err != nil {
		return fmt.Errorf("error saving updated proposal: %s", err.Error())
	}

	if callback != nil {
		callback(&updatedProposal)
	}

	return nil
}

func (p *Politeia) fetchAllUnfetchedProposals(tokenInventory *www.TokenInventoryReply, savedTokens []string) error {

	broadcastNotification := len(savedTokens) > 0

	approvedTokens, savedTokens := getUniqueTokens(tokenInventory.Approved, savedTokens)
	rejectedTokens, savedTokens := getUniqueTokens(tokenInventory.Rejected, savedTokens)
	abandonedTokens, savedTokens := getUniqueTokens(tokenInventory.Abandoned, savedTokens)
	preTokens, savedTokens := getUniqueTokens(tokenInventory.Pre, savedTokens)
	activeTokens, _ := getUniqueTokens(tokenInventory.Active, savedTokens)

	inventoryMap := map[int32][]string{
		ProposalCategoryPre:       preTokens,
		ProposalCategoryActive:    activeTokens,
		ProposalCategoryApproved:  approvedTokens,
		ProposalCategoryRejected:  rejectedTokens,
		ProposalCategoryAbandoned: abandonedTokens,
	}

	totalNumProposalsToFetch := 0
	for _, v := range inventoryMap {
		totalNumProposalsToFetch += len(v)
	}

	if totalNumProposalsToFetch > 0 {
		log.Infof("Politeia sync: fetching %d new proposals", totalNumProposalsToFetch)
	} else {
		log.Infof("Politeia sync: no new proposals found")
		return nil
	}

	if done(p.ctx) {
		return errors.New(ErrContextCanceled)
	}

	for category, tokens := range inventoryMap {
		err := p.fetchBatchProposals(category, tokens, broadcastNotification)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Politeia) fetchBatchProposals(category int32, tokens []string, broadcastNotification bool) error {
	for {
		if len(tokens) == 0 {
			break
		}

		p.mu.RLock()
		if done(p.ctx) {
			return errors.New(ErrContextCanceled)
		}

		limit := int(p.Client.policy.ProposalListPageSize)
		if len(tokens) <= limit {
			limit = len(tokens)
		}

		p.mu.RUnlock()

		var tokenBatch []string
		tokenBatch, tokens = tokens[:limit], tokens[limit:]

		proposals, err := p.Client.batchProposals(tokenBatch)
		if err != nil {
			return err
		}

		if done(p.ctx) {
			return errors.New(ErrContextCanceled)
		}

		votesSummaries, err := p.Client.batchVoteSummary(tokenBatch)
		if err != nil {
			return err
		}

		if done(p.ctx) {
			return errors.New(ErrContextCanceled)
		}

		for i := range proposals {
			proposals[i].Category = category
			if voteSummary, ok := votesSummaries[proposals[i].Token]; ok {
				proposals[i].VoteStatus = int32(voteSummary.Status)
				proposals[i].VoteApproved = voteSummary.Approved
				proposals[i].PassPercentage = int32(voteSummary.PassPercentage)
				proposals[i].EligibleTickets = int32(voteSummary.EligibleTickets)
				proposals[i].QuorumPercentage = int32(voteSummary.QuorumPercentage)
				proposals[i].YesVotes, proposals[i].NoVotes = getVotesCount(voteSummary.Results)
			}

			err = p.saveOrOverwiteProposal(&proposals[i])
			if err != nil {
				return fmt.Errorf("error saving new proposal: %s", err.Error())
			}

			p.mu.RLock()
			if broadcastNotification {
				p.publishNewProposal(&proposals[i])
			}
			p.mu.RUnlock()
		}

		log.Infof("Politeia sync: fetched %d proposals", limit)
	}

	return nil
}

func (p *Politeia) FetchProposalDescription(token string) (string, error) {

	proposal, err := p.GetProposalRaw(token)
	if err != nil {
		return "", err
	}

	Client, err := p.getClient()
	if err != nil {
		return "", err
	}

	proposalDetailsReply, err := Client.proposalDetails(token)
	if err != nil {
		return "", err
	}

	for _, file := range proposalDetailsReply.Proposal.Files {
		if file.Name == "index.md" {
			b, err := DecodeBase64(file.Payload)
			if err != nil {
				return "", err
			}

			// save file to db
			proposal.IndexFile = string(b)
			// index file version will be used to determine if the
			// saved file is out of date when compared to version.
			proposal.IndexFileVersion = proposal.Version
			err = p.saveOrOverwiteProposal(proposal)
			if err != nil {
				log.Errorf("error saving new proposal: %s", err.Error())
			}

			return proposal.IndexFile, nil
		}
	}

	return "", errors.New(ErrNotExist)
}

func (p *Politeia) ProposalVoteDetailsRaw(walletID int, token string) (*ProposalVoteDetails, error) {
	wal := p.WalletRef
	if wal == nil {
		return nil, fmt.Errorf(ErrWalletNotFound)
	}

	Client, err := p.getClient()
	if err != nil {
		return nil, err
	}

	detailsReply, err := Client.voteDetails(token)
	if err != nil {
		return nil, err
	}

	votesResults, err := Client.voteResults(token)
	if err != nil {
		return nil, err
	}

	hashes, err := StringsToHashes(detailsReply.Vote.EligibleTickets)
	if err != nil {
		return nil, err
	}

	ticketHashes, addresses, err := wal.Internal().CommittedTickets(wal.ShutdownContext(), hashes)
	if err != nil {
		return nil, err
	}

	castVotes := make(map[string]string)
	for _, v := range votesResults.Votes {
		castVotes[v.Ticket] = v.VoteBit
	}

	var eligibletickets = make([]*EligibleTicket, 0)
	var votedTickets = make([]*ProposalVote, 0)
	var yesVotes, noVotes int32
	for i := 0; i < len(ticketHashes); i++ {

		eligibleticket := &EligibleTicket{
			Hash:    ticketHashes[i].String(),
			Address: addresses[i].String(),
		}

		ainfo, err := wal.AddressInfo(eligibleticket.Address)
		if err != nil {
			return nil, err
		}

		// filter out tickets controlled by imported accounts
		if ainfo.AccountNumber == ImportedAccountNumber {
			continue
		}

		// filter out voted tickets
		if voteBit, ok := castVotes[eligibleticket.Hash]; ok {

			pv := &ProposalVote{
				Ticket: eligibleticket,
			}

			if voteBit == "1" {
				noVotes++
				pv.Bit = VoteBitNo
			} else if voteBit == "2" {
				yesVotes++
				pv.Bit = VoteBitYes
			}

			votedTickets = append(votedTickets, pv)
			continue
		}

		eligibletickets = append(eligibletickets, eligibleticket)
	}

	return &ProposalVoteDetails{
		EligibleTickets: eligibletickets,
		Votes:           votedTickets,
		YesVotes:        yesVotes,
		NoVotes:         noVotes,
	}, nil
}

func (p *Politeia) ProposalVoteDetails(walletID int, token string) (string, error) {
	voteDetails, err := p.ProposalVoteDetailsRaw(walletID, token)
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(voteDetails)
	return string(result), nil
}

func (p *Politeia) CastVotes(walletID int, eligibleTickets []*ProposalVote, token, passphrase string) error {
	wal := p.WalletRef
	if wal == nil {
		return fmt.Errorf(ErrWalletNotFound)
	}

	Client, err := p.getClient()
	if err != nil {
		return err
	}

	detailsReply, err := Client.voteDetails(token)
	if err != nil {
		return err
	}

	err = wal.UnlockWallet([]byte(passphrase))
	if err != nil {
		return translateError(err)
	}
	defer wal.LockWallet()

	votes := make([]tkv1.CastVote, 0)
	for _, eligibleTicket := range eligibleTickets {
		var voteBitHex string
		// Verify vote bit
		for _, vv := range detailsReply.Vote.Params.Options {
			if vv.ID == eligibleTicket.Bit {
				voteBitHex = strconv.FormatUint(vv.Bit, 16)
				break
			}
		}

		if voteBitHex == "" {
			return errors.New(ErrInvalid)
		}

		ticket := eligibleTicket.Ticket

		msg := token + ticket.Hash + voteBitHex

		signature, err := wal.SignMessageDirect(ticket.Address, msg)
		if err != nil {
			return err
		}

		sigHex := hex.EncodeToString(signature)
		singleVote := tkv1.CastVote{
			Token:     token,
			Ticket:    ticket.Hash,
			VoteBit:   voteBitHex,
			Signature: sigHex,
		}

		votes = append(votes, singleVote)
	}

	return Client.sendVotes(votes)
}

func (p *Politeia) AddNotificationListener(notificationListener ProposalNotificationListener, uniqueIdentifier string) error {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	if _, ok := p.NotificationListeners[uniqueIdentifier]; ok {
		return errors.New(ErrListenerAlreadyExist)
	}

	p.NotificationListeners[uniqueIdentifier] = notificationListener
	return nil
}

func (p *Politeia) RemoveNotificationListener(uniqueIdentifier string) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	delete(p.NotificationListeners, uniqueIdentifier)
}

func (p *Politeia) publishSynced() {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.NotificationListeners {
		notificationListener.OnProposalsSynced()
	}
}

func (p *Politeia) publishNewProposal(proposal *Proposal) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.NotificationListeners {
		notificationListener.OnNewProposal(proposal)
	}
}

func (p *Politeia) publishVoteStarted(proposal *Proposal) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.NotificationListeners {
		notificationListener.OnProposalVoteStarted(proposal)
	}
}

func (p *Politeia) publishVoteFinished(proposal *Proposal) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.NotificationListeners {
		notificationListener.OnProposalVoteFinished(proposal)
	}
}

func getVotesCount(options []www.VoteOptionResult) (int32, int32) {
	var yes, no int32

	for i := range options {
		if options[i].Option.Id == "yes" {
			yes = int32(options[i].VotesReceived)
		} else {
			no = int32(options[i].VotesReceived)
		}
	}

	return yes, no
}

func getUniqueTokens(tokenInventory, savedTokens []string) ([]string, []string) {
	var diff []string

	for i := range tokenInventory {
		exists := false

		for k := range savedTokens {
			if savedTokens[k] == tokenInventory[i] {
				exists = true
				savedTokens = append(savedTokens[:k], savedTokens[k+1:]...)
				break
			}
		}

		if !exists {
			diff = append(diff, tokenInventory[i])
		}
	}

	return diff, savedTokens
}
