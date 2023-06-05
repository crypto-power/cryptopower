package wallet

import (
	"gitlab.com/cryptopower/cryptopower/libwallet"
)

type ProposalStatus int

const (
	Synced ProposalStatus = iota
	VoteStarted
	NewProposalFound
	VoteFinished
)

type Proposal struct {
	Proposal       *libwallet.Proposal
	ProposalStatus ProposalStatus
}
