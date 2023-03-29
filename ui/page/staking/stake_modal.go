package staking

import (
	"context"
	"strconv"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type ticketBuyerModal struct {
	*load.Load
	*cryptomaterial.Modal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	settingsSaved func()
	onCancel      func()

	cancel          cryptomaterial.Button
	saveSettingsBtn cryptomaterial.Button

	balToMaintainEditor cryptomaterial.Editor

	accountSelector *components.WalletAndAccountSelector
	vspSelector     *components.VSPSelector

	dcrImpl *dcr.DCRAsset
}

func newTicketBuyerModal(l *load.Load) *ticketBuyerModal {
	impl := l.WL.SelectedWallet.Wallet.(*dcr.DCRAsset)
	if impl == nil {
		log.Warn(values.ErrDCRSupportedOnly)
		return nil
	}

	tb := &ticketBuyerModal{
		Load:  l,
		Modal: l.Theme.ModalFloatTitle("staking_modal"),

		cancel:          l.Theme.OutlineButton(values.String(values.StrCancel)),
		saveSettingsBtn: l.Theme.Button(values.String(values.StrSave)),
		vspSelector:     components.NewVSPSelector(l).Title(values.String(values.StrSelectVSP)),
		dcrImpl:         impl,
	}

	tb.balToMaintainEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrBalToMaintain))
	tb.balToMaintainEditor.Editor.SingleLine = true

	tb.saveSettingsBtn.SetEnabled(false)

	return tb
}

func (tb *ticketBuyerModal) OnSettingsSaved(settingsSaved func()) *ticketBuyerModal {
	tb.settingsSaved = settingsSaved
	return tb
}

func (tb *ticketBuyerModal) OnCancel(cancel func()) *ticketBuyerModal {
	tb.onCancel = cancel
	return tb
}

func (tb *ticketBuyerModal) SetError(err string) {
	tb.balToMaintainEditor.SetError(values.TranslateErr(err))
}

func (tb *ticketBuyerModal) OnResume() {
	if tb.dcrImpl == nil {
		log.Error("Only DCR implementation is supportted")
		return
	}

	tb.initializeAccountSelector()
	tb.ctx, tb.ctxCancel = context.WithCancel(context.TODO())
	tb.accountSelector.ListenForTxNotifications(tb.ctx, tb.ParentWindow())

	if len(tb.dcrImpl.KnownVSPs()) == 0 {
		// TODO: Does this modal need this list?
		go tb.dcrImpl.ReloadVSPList(context.TODO())
	}

	wl := load.NewWalletMapping(tb.WL.SelectedWallet.Wallet)

	// loop through all available wallets and select the one with ticket buyer config.
	// if non, set the selected wallet to the first.
	// temporary work around for only one wallet.
	if tb.dcrImpl.TicketBuyerConfigIsSet() {
		tbConfig := tb.dcrImpl.AutoTicketsBuyerConfig()

		account, err := components.GetTicketPurchaseAccount(tb.dcrImpl)
		if err != nil {
			errModal := modal.NewErrorModal(tb.Load, err.Error(), modal.DefaultClickFunc())
			tb.ParentWindow().ShowModal(errModal)
		}

		if account != nil {
			tb.accountSelector.SetSelectedAccount(account)
		} else {
			// If a valid account is not set, choose one from available the valid accounts.
			if err := tb.accountSelector.SelectFirstValidAccount(wl); err != nil {
				errModal := modal.NewErrorModal(tb.Load, err.Error(), modal.DefaultClickFunc())
				tb.ParentWindow().ShowModal(errModal)
			}
		}

		tb.vspSelector.SelectVSP(tbConfig.VspHost)
		w := tb.WL.SelectedWallet.Wallet
		tb.balToMaintainEditor.Editor.SetText(strconv.FormatFloat(w.ToAmount(tbConfig.BalanceToMaintain).ToCoin(), 'f', 0, 64))
	}

	if tb.accountSelector.SelectedAccount() == nil {
		err := tb.accountSelector.SelectFirstValidAccount(wl)
		if err != nil {
			errModal := modal.NewErrorModal(tb.Load, err.Error(), modal.DefaultClickFunc())
			tb.ParentWindow().ShowModal(errModal)
		}
	}
}

func (tb *ticketBuyerModal) Layout(gtx layout.Context) layout.Dimensions {
	l := []layout.Widget{
		func(gtx C) D {
			t := tb.Theme.H6(values.String(values.StrAutoTicketPurchase))
			t.Font.Weight = text.SemiBold
			return t.Layout(gtx)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top:    values.MarginPadding8,
						Bottom: values.MarginPadding16,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return tb.accountSelector.Layout(tb.ParentWindow(), gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return tb.balToMaintainEditor.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top:    values.MarginPadding16,
						Bottom: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return tb.vspSelector.Layout(tb.ParentWindow(), gtx)
					})
				}),
			)
		},
		func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding4,
						}.Layout(gtx, tb.cancel.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						return tb.saveSettingsBtn.Layout(gtx)
					}),
				)
			})
		},
	}

	return tb.Modal.Layout(gtx, l)
}

func (tb *ticketBuyerModal) canSave() bool {
	if tb.vspSelector.SelectedVSP() == nil {
		return false
	}

	if tb.balToMaintainEditor.Editor.Text() == "" {
		return false
	}

	return true
}

func (tb *ticketBuyerModal) initializeAccountSelector() {
	tb.accountSelector = components.NewWalletAndAccountSelector(tb.Load).
		Title(values.String(values.StrPurchasingAcct)).
		AccountSelected(func(selectedAccount *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			// Imported and watch only wallet accounts are invalid for sending
			accountIsValid := account.Number != dcr.ImportedAccountNumber && !tb.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet()

			if tb.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
				!tb.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) {
				// Spending from unmixed accounts is disabled for the selected wallet
				dcrImpl := tb.WL.SelectedWallet.Wallet.(*dcr.DCRAsset)
				accountIsValid = account.Number == dcrImpl.MixedAccountNumber()
			}

			return accountIsValid
		})
	wl := load.NewWalletMapping(tb.WL.SelectedWallet.Wallet)
	tb.accountSelector.SelectFirstValidAccount(wl)
}

func (tb *ticketBuyerModal) OnDismiss() {
	tb.ctxCancel()
}

func (tb *ticketBuyerModal) Handle() {
	tb.saveSettingsBtn.SetEnabled(tb.canSave())

	if tb.cancel.Clicked() || tb.Modal.BackdropClicked(true) {
		tb.onCancel()
		tb.Dismiss()
	}

	if tb.saveSettingsBtn.Clicked() {
		vspHost := tb.vspSelector.SelectedVSP().Host
		amount, err := strconv.ParseFloat(tb.balToMaintainEditor.Editor.Text(), 64)
		if err != nil {
			tb.SetError(err.Error())
			return
		}

		balToMaintain := dcr.AmountAtom(amount)
		account := tb.accountSelector.SelectedAccount()

		tb.dcrImpl.SetAutoTicketsBuyerConfig(vspHost, account.Number, balToMaintain)
		tb.settingsSaved()
		tb.Dismiss()
	}
}
