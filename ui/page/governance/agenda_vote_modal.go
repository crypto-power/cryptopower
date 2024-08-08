package governance

import (
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type agendaVoteModal struct {
	// This modal inherits most of the CreatePasswordModal implementation
	*modal.CreatePasswordModal

	agenda     *dcr.Agenda
	voteChoice string

	onPreferenceUpdated func()

	accountDropdown *components.AccountDropdown
	accountSelected *sharedW.Account
	dcrImpl         *dcr.Asset
}

func newAgendaVoteModal(l *load.Load, dcrWallet *dcr.Asset, agenda *dcr.Agenda, votechoice string, onPreferenceUpdated func()) *agendaVoteModal {
	avm := &agendaVoteModal{
		agenda:              agenda,
		CreatePasswordModal: modal.NewCreatePasswordModal(l),
		voteChoice:          votechoice,
		onPreferenceUpdated: onPreferenceUpdated,
		dcrImpl:             dcrWallet,
	}
	avm.EnableName(false)
	avm.EnableConfirmPassword(false)
	avm.SetPositiveButtonText(values.String(values.StrVote))
	avm.SetPositiveButtonCallback(avm.sendVotes)

	// Source account picker
	avm.accountDropdown = components.NewAccountDropdown(l).
		SetChangedCallback(func(selectedAccount *sharedW.Account) {
			avm.accountSelected = selectedAccount
		}).
		AccountValidator(func(_ *sharedW.Account) bool {
			return true
		}).
		Setup(dcrWallet, avm.accountSelected)

	return avm
}

func (avm *agendaVoteModal) OnResume() {
	_ = avm.accountDropdown.Setup(avm.dcrImpl, avm.accountSelected)
}

// - Layout
func (avm *agendaVoteModal) Layout(gtx layout.Context) D {
	w := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return avm.accountDropdown.Layout(gtx, values.StrSettings)
		},
	}

	w = append(w, avm.CreatePasswordModal.LayoutComponents()...)

	return avm.Modal.Layout(gtx, w)
}

func (avm *agendaVoteModal) sendVotes(ticketHash, password string, _ *modal.CreatePasswordModal) bool {
	err := avm.dcrImpl.SetVoteChoice(int32(avm.accountSelected.AccountNumber), avm.agenda.AgendaID,
		avm.voteChoice, ticketHash, password)
	if err != nil {
		avm.CreatePasswordModal.SetError(err.Error())
		return false
	}
	successModal := modal.NewSuccessModal(avm.Load, values.String(values.StrVoteUpdated), modal.DefaultClickFunc())
	avm.ParentWindow().ShowModal(successModal)
	avm.onPreferenceUpdated()
	return true
}
