package send

import (
	"context"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type PageModal struct {
	*load.Load
	*cryptomaterial.Modal

	sendPage *Page

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	okBtn cryptomaterial.Button

	sourceWalletSelector *components.WalletAndAccountSelector
}

func NewPageModal(l *load.Load) *PageModal {
	sm := &PageModal{
		Load:  l,
		Modal: l.Theme.ModalFloatTitle(values.String(values.StrSettings)),
	}

	sm.okBtn = l.Theme.Button(values.String(values.StrOK))
	sm.okBtn.Font.Weight = font.Medium

	// initialize wallet selector
	sm.sourceWalletSelector = components.NewWalletAndAccountSelector(sm.Load).
		Title(values.String(values.StrSelectWallet))
	sm.setSelectedWallet()

	sm.sendPage = NewSendPage(sm.Load)

	sm.initWalletSelectors()

	return sm
}

func (sm *PageModal) OnResume() {
	sm.ctx, sm.ctxCancel = context.WithCancel(context.TODO())
	sm.sourceWalletSelector.ListenForTxNotifications(sm.ctx, sm.ParentWindow())
}

func (sm *PageModal) OnDismiss() {
	sm.ctxCancel()
}

func (sm *PageModal) Handle() {
	if sm.okBtn.Clicked() || sm.Modal.BackdropClicked(true) {
		sm.Dismiss()
	}
	sm.sendPage.HandleUserInteractions()
}

func (sm *PageModal) Layout(gtx C) D {
	walletSelector := func(gtx C) D {
		return sm.sourceWalletSelector.Layout(sm.ParentWindow(), gtx)
	}
	PageModalLayout := []layout.Widget{
		func(gtx C) D {
			return sm.sendPage.layoutDesktop(sm.ParentWindow(), walletSelector, gtx)
		},
	}
	return sm.Modal.Layout(gtx, PageModalLayout, 450)
}

func (sm *PageModal) initWalletSelectors() {
	// Source wallet picker
	sm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		sm.setSelectedWallet()
		sm.sendPage.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})
	sm.setSelectedWallet()
}

func (sm *PageModal) setSelectedWallet() {
	sm.WL.SelectedWallet = &load.WalletItem{
		Wallet: sm.sourceWalletSelector.SelectedWallet().Asset,
	}
	balance, err := sm.WL.TotalWalletsBalance()
	if err == nil {
		sm.WL.SelectedWallet.TotalBalance = balance.String()
	}
}
