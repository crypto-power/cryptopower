package listeners

import (
	"gitlab.com/raedah/libwallet"
	"gitlab.com/raedah/cryptopower/wallet"
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

func (pn *ProposalNotificationListener) OnNewProposal(proposal *libwallet.Proposal) {
	update := wallet.Proposal{
		ProposalStatus: wallet.NewProposalFound,
		Proposal:       proposal,
	}
	pn.sendNotification(update)
}

func (pn *ProposalNotificationListener) OnProposalVoteStarted(proposal *libwallet.Proposal) {
	update := wallet.Proposal{
		ProposalStatus: wallet.VoteStarted,
		Proposal:       proposal,
	}
	pn.sendNotification(update)
}
func (pn *ProposalNotificationListener) OnProposalVoteFinished(proposal *libwallet.Proposal) {
	update := wallet.Proposal{
		ProposalStatus: wallet.VoteFinished,
		Proposal:       proposal,
	}
	pn.sendNotification(update)
}

func (pn *ProposalNotificationListener) sendNotification(signal wallet.Proposal) {
	pn.ProposalNotifChan <- signal
}
