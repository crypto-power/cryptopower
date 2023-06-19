package listeners

import (
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/wallet"
)

// ProposalNotificationListener satisfies dcr
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
		Proposal:       &libwallet.Proposal{},
		ProposalStatus: wallet.Synced,
	})
}

func (pn *ProposalNotificationListener) OnNewProposal(proposal interface{}) {
	p, ok := proposal.(*libwallet.Proposal)
	if !ok {
		p = &libwallet.Proposal{}
	}
	update := wallet.Proposal{
		ProposalStatus: wallet.NewProposalFound,
		Proposal:       p,
	}
	pn.sendNotification(update)
}

func (pn *ProposalNotificationListener) OnProposalVoteStarted(proposal interface{}) {
	p, ok := proposal.(*libwallet.Proposal)
	if !ok {
		p = &libwallet.Proposal{}
	}
	update := wallet.Proposal{
		ProposalStatus: wallet.VoteStarted,
		Proposal:       p,
	}
	pn.sendNotification(update)
}
func (pn *ProposalNotificationListener) OnProposalVoteFinished(proposal interface{}) {
	p, ok := proposal.(*libwallet.Proposal)
	if !ok {
		p = &libwallet.Proposal{}
	}
	update := wallet.Proposal{
		ProposalStatus: wallet.VoteFinished,
		Proposal:       p,
	}
	pn.sendNotification(update)
}

func (pn *ProposalNotificationListener) sendNotification(signal wallet.Proposal) {
	if signal.Proposal != nil {
		select {
		case pn.ProposalNotifChan <- signal:
		default:
		}
	}
}
