package wallet

import "gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"

type ProposalStatus int

const (
	Synced ProposalStatus = iota
	VoteStarted
	NewProposalFound
	VoteFinished
)

type Proposal struct {
	Proposal       *dcr.Proposal
	ProposalStatus ProposalStatus
}
