package listeners

import (
	"gitlab.com/raedah/cryptopower/wallet"
	"gitlab.com/raedah/libwallet"
)

// ProposalNotificationListener satisfies libwallet
// ProposalNotificationListener interface contract.
type ProposalNotificationListener struct {
	ProposalNotifChan chan wallet.Proposal
}

func NewProposalNotificationListener() *ProposalNotificationListener {
	return &ProposalNotificationListener{
		ProposalNotifChan: make(chan wallet.Proposal, 4),
	}
}

func (pn *ProposalNotificationListener) OnProposalsSynced() {
	pn.sendNotification(wallet.Proposal{
		ProposalStatus: wallet.Synced,
	})
}

func (pn *ProposalNotificationListener) OnNewProposal(proposal interface{}) {
	update := wallet.Proposal{
		ProposalStatus: wallet.NewProposalFound,
		Proposal:       proposal.(*libwallet.Proposal),
	}
	pn.sendNotification(update)
}

func (pn *ProposalNotificationListener) OnProposalVoteStarted(proposal interface{}) {
	update := wallet.Proposal{
		ProposalStatus: wallet.VoteStarted,
		Proposal:       proposal.(*libwallet.Proposal),
	}
	pn.sendNotification(update)
}
func (pn *ProposalNotificationListener) OnProposalVoteFinished(proposal interface{}) {
	update := wallet.Proposal{
		ProposalStatus: wallet.VoteFinished,
		Proposal:       proposal.(*libwallet.Proposal),
	}
	pn.sendNotification(update)
}

func (pn *ProposalNotificationListener) sendNotification(signal wallet.Proposal) {
	if signal.Proposal != nil {
		pn.ProposalNotifChan <- signal
	}
}
