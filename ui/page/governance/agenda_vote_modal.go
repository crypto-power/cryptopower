package governance

import (
	"gioui.org/layout"
	"gioui.org/text"

	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/utils"
	"gitlab.com/raedah/cryptopower/ui/values"
)

type agendaVoteModal struct {
	// This modal inherits most of the CreatePasswordModal implementation
	*modal.CreatePasswordModal

	agenda     *dcr.Agenda
	voteChoice string

	onPreferenceUpdated func()

	accountSelector *components.WalletAndAccountSelector
	accountSelected *sharedW.Account
}

func newAgendaVoteModal(l *load.Load, agenda *dcr.Agenda, votechoice string, onPreferenceUpdated func()) *agendaVoteModal {
	avm := &agendaVoteModal{
		agenda:              agenda,
		CreatePasswordModal: modal.NewCreatePasswordModal(l),
		voteChoice:          votechoice,
		onPreferenceUpdated: onPreferenceUpdated,
	}
	avm.EnableName(false)
	avm.EnableConfirmPassword(false)
	avm.SetPositiveButtonText(values.String(values.StrVote))
	avm.SetPositiveButtonCallback(avm.sendVotes)

	// Source account picker
	avm.accountSelector = components.NewWalletAndAccountSelector(l).
		Title(values.String(values.StrSelectAcc)).
		AccountSelected(func(selectedAccount *sharedW.Account, walletType utils.WalletType) {
			avm.accountSelected = selectedAccount
		}).
		AccountValidator(func(account *sharedW.Account) bool {
			return true
		})

	return avm
}

func (avm *agendaVoteModal) OnResume() {
	wl := components.NewDCRCommonWallet(avm.WL.SelectedWallet.Wallet)
	avm.accountSelector.SelectFirstValidAccount(wl)
}

// - Layout
func (avm *agendaVoteModal) Layout(gtx layout.Context) D {
	w := []layout.Widget{
		func(gtx C) D {
			t := avm.Theme.H6(values.String(values.StrSettings))
			t.Font.Weight = text.SemiBold
			return t.Layout(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			return avm.accountSelector.Layout(avm.ParentWindow(), gtx)
		},
	}

	w = append(w, avm.CreatePasswordModal.LayoutComponents(gtx)...)

	return avm.Modal.Layout(gtx, w)
}

func (avm *agendaVoteModal) sendVotes(_, password string, m *modal.CreatePasswordModal) bool {
	err := avm.CreatePasswordModal.WL.SelectedWallet.Wallet.SetVoteChoice(avm.agenda.AgendaID, avm.voteChoice, "", []byte(password))
	if err != nil {
		avm.CreatePasswordModal.SetError(err.Error())
		avm.CreatePasswordModal.SetLoading(false)
		return false
	}
	successModal := modal.NewSuccessModal(avm.Load, values.String(values.StrVoteUpdated), modal.DefaultClickFunc())
	avm.ParentWindow().ShowModal(successModal)
	avm.Dismiss()
	avm.onPreferenceUpdated()
	return true
}
