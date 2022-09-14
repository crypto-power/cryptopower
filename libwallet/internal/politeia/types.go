package politeia

type Proposal struct {
	ID               int    `storm:"id,increment"`
	Token            string `json:"token" storm:"unique"`
	Category         int32  `json:"category" storm:"index"`
	Name             string `json:"name"`
	State            int32  `json:"state"`
	Status           int32  `json:"status"`
	Timestamp        int64  `json:"timestamp"`
	UserID           string `json:"userid"`
	Username         string `json:"username"`
	NumComments      int32  `json:"numcomments"`
	Version          string `json:"version"`
	PublishedAt      int64  `json:"publishedat"`
	IndexFile        string `json:"indexfile"`
	IndexFileVersion string `json:"fileversion"`
	VoteStatus       int32  `json:"votestatus"`
	VoteApproved     bool   `json:"voteapproved"`
	YesVotes         int32  `json:"yesvotes"`
	NoVotes          int32  `json:"novotes"`
	EligibleTickets  int32  `json:"eligibletickets"`
	QuorumPercentage int32  `json:"quorumpercentage"`
	PassPercentage   int32  `json:"passpercentage"`
}

type ProposalOverview struct {
	All        int32
	Discussion int32
	Voting     int32
	Approved   int32
	Rejected   int32
	Abandoned  int32
}

type ProposalVoteDetails struct {
	EligibleTickets []*EligibleTicket
	Votes           []*ProposalVote
	YesVotes        int32
	NoVotes         int32
}

type EligibleTicket struct {
	Hash    string
	Address string
}

type ProposalVote struct {
	Ticket *EligibleTicket
	Bit    string
}

type ProposalNotificationListener interface {
	OnProposalsSynced()
	OnNewProposal(proposal interface{})
	OnProposalVoteStarted(proposal interface{})
	OnProposalVoteFinished(proposal interface{})
}
