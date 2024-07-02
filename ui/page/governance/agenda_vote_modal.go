package governance

import (
	"gioui.org/font"
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

	accountSelector *components.WalletAndAccountSelector
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
	avm.accountSelector = components.NewWalletAndAccountSelector(l).
		Title(values.String(values.StrSelectAcc)).
		AccountSelected(func(selectedAccount *sharedW.Account) {
			avm.accountSelected = selectedAccount
		}).
		AccountValidator(func(_ *sharedW.Account) bool {
			return true
		})

	return avm
}

func (avm *agendaVoteModal) OnResume() {
	_ = avm.accountSelector.SelectFirstValidAccount(avm.dcrImpl)
}

// - Layout
func (avm *agendaVoteModal) Layout(gtx layout.Context) D {
	w := []layout.Widget{
		func(gtx C) D {
			t := avm.Theme.H6(values.String(values.StrSettings))
			t.Font.Weight = font.SemiBold
			return t.Layout(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			return avm.accountSelector.Layout(avm.ParentWindow(), gtx)
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
