package dcr

import (
	"gitlab.com/raedah/cryptopower/libwallet/internal/politeia"
)

// This file exports politeia implementation in internal/politeia.
// As described below all implementation in internal politeia folder must be exported here.
// https://docs.google.com/document/d/1e8kOo3r51b2BWtTs_1uADIA5djfXhPT36s6eHVRIvaU/edit

// The of politeia implementation was put in a folder called internal. This makes
// all that functionality unusable to packages that don't share the parent folder with the internal folder.

// Without reverting back to the previous implementation that worked, we came up with a
// brilliant implementation to export the types within the internal/politeia implementation
// so that the functionality can be exported. The exported types inherited the structure
// from the type defined in politieia and thus exporting the politeia implementation that was hidden.

const (
	PoliteiaMainnetHost = politeia.PoliteiaMainnetHost
	PoliteiaTestnetHost = politeia.PoliteiaTestnetHost

	VoteBitYes = politeia.VoteBitYes
	VoteBitNo  = politeia.VoteBitNo

	ProposalCategoryAll       = politeia.ProposalCategoryAll
	ProposalCategoryPre       = politeia.ProposalCategoryPre
	ProposalCategoryActive    = politeia.ProposalCategoryActive
	ProposalCategoryApproved  = politeia.ProposalCategoryApproved
	ProposalCategoryRejected  = politeia.ProposalCategoryRejected
	ProposalCategoryAbandoned = politeia.ProposalCategoryAbandoned
)

type Proposal struct {
	politeia.Proposal
}

type Politeia struct {
	politeia.Politeia
}

type ProposalOverview struct {
	politeia.ProposalOverview
}

type ProposalVoteDetails struct {
	politeia.ProposalVoteDetails
}

type EligibleTicket struct {
	politeia.EligibleTicket
}

type ProposalVote struct {
	politeia.ProposalVote
}

func ConvertVotes(votes []*ProposalVote) []*politeia.ProposalVote {
	var eligibleTickets = make([]*politeia.ProposalVote, len(votes))
	for i, value := range votes {
		eligibleTickets[i] = &politeia.ProposalVote{
			Bit: value.Bit,
			Ticket: &politeia.EligibleTicket{
				Hash:    value.Ticket.Hash,
				Address: value.Ticket.Address,
			},
		}
	}
	return eligibleTickets
}
