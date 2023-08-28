package libwallet

import (
	"github.com/crypto-power/cryptopower/libwallet/internal/politeia"
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
	// PoliteiaMainnetHost is the politeia mainnet host.
	PoliteiaMainnetHost = politeia.PoliteiaMainnetHost
	// PoliteiaTestnetHost is the politeia testnet host.
	PoliteiaTestnetHost = politeia.PoliteiaTestnetHost

	// VoteBitYes is the string value for identifying "yes" vote bits.
	VoteBitYes = politeia.VoteBitYes
	// VoteBitNo is the string value for identifying "no" vote bits.
	VoteBitNo = politeia.VoteBitNo

	// ProposalCategoryAll is the int value for identifying all proposals.
	ProposalCategoryAll = politeia.ProposalCategoryAll
	// ProposalCategoryPre is the int value for identifying pre proposals.
	ProposalCategoryPre = politeia.ProposalCategoryPre
	// ProposalCategoryActive is the int value for identifying active proposals.
	ProposalCategoryActive = politeia.ProposalCategoryActive
	// ProposalCategoryApproved is the int value for identifying approved proposals.
	ProposalCategoryApproved = politeia.ProposalCategoryApproved
	// ProposalCategoryRejected is the int value for identifying rejected proposals.
	ProposalCategoryRejected = politeia.ProposalCategoryRejected
	// ProposalCategoryAbandoned is the int value for identifying abandoned proposals.
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

// WrapVote, wraps vote type of politeia.ProposalVote into libwallet.ProposalVote
func WrapVote(hash, address, bit string) *ProposalVote {
	return &ProposalVote{
		ProposalVote: politeia.ProposalVote{
			Bit: bit,
			Ticket: &politeia.EligibleTicket{
				Hash:    hash,
				Address: address,
			},
		},
	}
}

func ConvertVotes(votes []*ProposalVote) []*politeia.ProposalVote {
	eligibleTickets := make([]*politeia.ProposalVote, len(votes))
	for i, value := range votes {
		eligibleTickets[i] = &WrapVote(value.Ticket.Hash,
			value.Ticket.Address, value.Bit).ProposalVote
	}
	return eligibleTickets
}
