package politeia

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"decred.org/dcrwallet/v3/wallet"
	"decred.org/dcrwallet/v3/wallet/udb"
	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
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
func (p *Politeia) Sync(ctx context.Context) error {
	p.mu.RLock()
	if p.cancelSync != nil {
		p.mu.RUnlock()
		return errors.New(ErrSyncAlreadyInProgress)
	}

	p.ctx, p.cancelSync = context.WithCancel(ctx)
	defer func() {
		p.cancelSync = nil
	}()
	p.mu.RUnlock()

	log.Info("Politeia sync: started")
	for {
		// Check if politeia has been shutdown and exit if true.
		if p.ctx.Err() != nil {
			return p.ctx.Err()
		}

		err := p.getClient()
		if err != nil {
			log.Errorf("Error fetching for politeia server policy: %v", err)
			time.Sleep(retryInterval * time.Second)
			continue
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

func (p *Politeia) StopSync() {
	p.mu.RLock()
	if p.cancelSync != nil {
		p.cancelSync()
		p.cancelSync = nil
	}
	p.mu.RUnlock()
	log.Info("Politeia sync: stopped")
}

func (p *Politeia) checkForUpdates() error {
	// if server's policy is not set at this point the politeia server is not accessible
	p.mu.RLock()
	clientPolicy := p.client.policy
	p.mu.RUnlock()
	if clientPolicy == nil {
		return errors.New("politeia server policy not set")
	}

	offset := 0
	p.mu.RLock()
	limit := int32(p.client.policy.ProposalListPageSize)
	p.mu.RUnlock()

	for {
		// Check if politeia has been shutdown and exit if true.
		if p.ctx.Err() != nil {
			return p.ctx.Err()
		}

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
	tokenInventory, err := p.client.tokenInventory()
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

	batchProposals, err := p.client.batchProposals(tokens)
	if err != nil {
		return err
	}

	batchVotesSummaries, err := p.client.batchVoteSummary(tokens)
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

	var callback func(interface{})

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

	err := p.db.Update(&updatedProposal)
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

	// Check if politeia has been shutdown and exit if true.
	if p.ctx.Err() != nil {
		return p.ctx.Err()
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

		// Check if politeia has been shutdown and exit if true.
		if p.ctx.Err() != nil {
			return p.ctx.Err()
		}

		limit := int(p.client.policy.ProposalListPageSize)
		if len(tokens) <= limit {
			limit = len(tokens)
		}

		p.mu.RUnlock()

		var tokenBatch []string
		tokenBatch, tokens = tokens[:limit], tokens[limit:]

		proposals, err := p.client.batchProposals(tokenBatch)
		if err != nil {
			return err
		}

		// Check if politeia has been shutdown and exit if true.
		if p.ctx.Err() != nil {
			return p.ctx.Err()
		}

		votesSummaries, err := p.client.batchVoteSummary(tokenBatch)
		if err != nil {
			return err
		}

		for i := range proposals {
			// Check if politeia has been shutdown and exit if true.
			if p.ctx.Err() != nil {
				return p.ctx.Err()
			}

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
	// Check if politeia has been shutdown and exit if true.
	if p.ctx.Err() != nil {
		return "", p.ctx.Err()
	}

	proposal, err := p.GetProposalRaw(token)
	if err != nil {
		return "", err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	err = p.getClient()
	if err != nil {
		return "", err
	}

	proposalDetailsReply, err := p.client.proposalDetails(token)
	if err != nil {
		return "", err
	}

	for _, file := range proposalDetailsReply.Proposal.Files {
		if file.Name == "index.md" {
			b, err := base64.StdEncoding.DecodeString(file.Payload)
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

func (p *Politeia) ProposalVoteDetailsRaw(ctx context.Context, wallet *wallet.Wallet, token string) (*ProposalVoteDetails, error) {
	// Check if politeia has been shutdown and exit if true.
	if p.ctx.Err() != nil {
		return nil, p.ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	err := p.getClient()
	if err != nil {
		return nil, err
	}

	detailsReply, err := p.client.voteDetails(token)
	if err != nil {
		return nil, err
	}

	votesResults, err := p.client.voteResults(token)
	if err != nil {
		return nil, err
	}

	eligibleTickets := detailsReply.Vote.EligibleTickets
	hashes := make([]*chainhash.Hash, 0, len(eligibleTickets))
	for _, v := range eligibleTickets {
		hash, err := chainhash.NewHashFromStr(v)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}

	if err != nil {
		return nil, err
	}

	ticketHashes, addresses, err := wallet.CommittedTickets(ctx, hashes)
	if err != nil {
		return nil, err
	}

	castVotes := make(map[string]string)
	for _, v := range votesResults.Votes {
		castVotes[v.Ticket] = v.VoteBit
	}

	eligibletickets := make([]*EligibleTicket, 0)
	votedTickets := make([]*ProposalVote, 0)
	var yesVotes, noVotes int32
	for i := 0; i < len(ticketHashes); i++ {

		eligibleticket := &EligibleTicket{
			Hash:    ticketHashes[i].String(),
			Address: addresses[i].String(),
		}

		isMine, accountNumber, err := walletAddressAccount(ctx, wallet, eligibleticket.Address)
		if err != nil {
			return nil, err
		}

		// filter out tickets controlled by imported accounts or not owned by this wallet
		if !isMine || accountNumber == udb.ImportedAddrAccount {
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

func (p *Politeia) ProposalVoteDetails(ctx context.Context, wallet *wallet.Wallet, token string) (string, error) {
	voteDetails, err := p.ProposalVoteDetailsRaw(ctx, wallet, token)
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(voteDetails)
	return string(result), nil
}

func (p *Politeia) CastVotes(ctx context.Context, wallet *wallet.Wallet, eligibleTickets []*ProposalVote, token, passphrase string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	err := p.getClient()
	if err != nil {
		return err
	}

	detailsReply, err := p.client.voteDetails(token)
	if err != nil {
		return err
	}

	err = wallet.Unlock(ctx, []byte(passphrase), nil)
	if err != nil {
		return translateError(err)
	}
	defer wallet.Lock()

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

		signature, err := walletSignMessage(ctx, wallet, ticket.Address, msg)
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

	return p.client.sendVotes(votes)
}

func (p *Politeia) AddNotificationListener(notificationListener ProposalNotificationListener, uniqueIdentifier string) error {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	if _, ok := p.notificationListeners[uniqueIdentifier]; ok {
		return errors.New(ErrListenerAlreadyExist)
	}

	p.notificationListeners[uniqueIdentifier] = notificationListener
	return nil
}

func (p *Politeia) RemoveNotificationListener(uniqueIdentifier string) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	delete(p.notificationListeners, uniqueIdentifier)
}

func (p *Politeia) publishSynced() {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.notificationListeners {
		notificationListener.OnProposalsSynced()
	}
}

func (p *Politeia) publishNewProposal(proposal interface{}) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.notificationListeners {
		data, _ := proposal.(*Proposal)
		notificationListener.OnNewProposal(data)
	}
}

func (p *Politeia) publishVoteStarted(proposal interface{}) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.notificationListeners {
		data, _ := proposal.(*Proposal)
		notificationListener.OnProposalVoteStarted(data)
	}
}

func (p *Politeia) publishVoteFinished(proposal interface{}) {
	p.notificationListenersMu.Lock()
	defer p.notificationListenersMu.Unlock()

	for _, notificationListener := range p.notificationListeners {
		data, _ := proposal.(*Proposal)
		notificationListener.OnProposalVoteFinished(data)
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

// TODO: When dcr wallets move to a different package, use dcr.Asset from the
// new package rather than wallet.Wallet. Then do dcr.Asset.AddressInfo instead
// of using this function.
func walletAddressAccount(ctx context.Context, wallet *wallet.Wallet, address string) (bool, uint32, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.ChainParams())
	if err != nil {
		return false, 0, err
	}

	known, _ := wallet.KnownAddress(ctx, addr)
	if known != nil {
		accountNumber, err := wallet.AccountNumber(ctx, known.AccountName())
		return true, accountNumber, err
	}

	return false, 0, nil
}

// TODO: When dcr wallets move to a different package, use dcr.Asset from the
// new package rather than wallet.Wallet. Then do dcr.Asset.signMessage instead
// of using this function.
func walletSignMessage(ctx context.Context, wallet *wallet.Wallet, address string, message string) ([]byte, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.ChainParams())
	if err != nil {
		return nil, translateError(err)
	}

	// Addresses must have an associated secp256k1 private key and therefore
	// must be P2PK or P2PKH (P2SH is not allowed).
	switch addr.(type) {
	case *stdaddr.AddressPubKeyEcdsaSecp256k1V0:
	case *stdaddr.AddressPubKeyHashEcdsaSecp256k1V0:
	default:
		return nil, errors.New(ErrInvalidAddress)
	}

	sig, err := wallet.SignMessage(ctx, message, addr)
	if err != nil {
		return nil, translateError(err)
	}

	return sig, nil
}
