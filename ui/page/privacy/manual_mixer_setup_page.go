package privacy

import (
	"context"

	"gioui.org/layout"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/renderers"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const ManualMixerSetupPageID = "ManualMixerSetup"

type ManualMixerSetupPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	mixedAccountSelector   *components.WalletAndAccountSelector
	unmixedAccountSelector *components.WalletAndAccountSelector

	backButton     cryptomaterial.IconButton
	infoButton     cryptomaterial.IconButton
	toPrivacySetup cryptomaterial.Button

	dcrImpl *dcr.DCRAsset
}

func NewManualMixerSetupPage(l *load.Load) *ManualMixerSetupPage {
	impl := l.WL.SelectedWallet.Wallet.(*dcr.DCRAsset)
	if impl == nil {
		log.Warn(values.ErrDCRSupportedOnly)
		return nil
	}

	pg := &ManualMixerSetupPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ManualMixerSetupPageID),
		toPrivacySetup:   l.Theme.Button("Set up"),
		dcrImpl:          impl,
	}

	// Mixed account picker
	pg.mixedAccountSelector = components.NewWalletAndAccountSelector(l).
		Title("Mixed account").
		AccountSelected(func(selectedAccount *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			wal := pg.Load.WL.MultiWallet.WalletWithID(account.WalletID)

			var unmixedAccNo int32 = -1
			if unmixedAcc := pg.unmixedAccountSelector.SelectedAccount(); unmixedAcc != nil {
				unmixedAccNo = unmixedAcc.Number
			}

			// Imported, watch only and default wallet accounts are invalid to use as a mixed account
			accountIsValid := account.Number != load.MaxInt32 && !wal.IsWatchingOnlyWallet() && account.Number != dcr.DefaultAccountNum

			if !accountIsValid || account.Number == unmixedAccNo {
				return false
			}

			return true
		})
	pg.mixedAccountSelector.SelectFirstValidAccount(l.WL.SelectedWallet.Wallet)
	// Unmixed account picker
	pg.unmixedAccountSelector = components.NewWalletAndAccountSelector(l).
		Title("Unmixed account").
		AccountSelected(func(selectedAccount *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			wal := pg.Load.WL.MultiWallet.WalletWithID(account.WalletID)

			var mixedAccNo int32 = -1
			if mixedAcc := pg.mixedAccountSelector.SelectedAccount(); mixedAcc != nil {
				mixedAccNo = mixedAcc.Number
			}

			// Imported, watch only and default wallet accounts are invalid to use as an unmixed account
			accountIsValid := account.Number != load.MaxInt32 && !wal.IsWatchingOnlyWallet() && account.Number != dcr.DefaultAccountNum

			// Account is invalid if already selected by mixed account selector.
			if !accountIsValid || account.Number == mixedAccNo {
				return false
			}

			return true
		})
	pg.unmixedAccountSelector.SelectFirstValidAccount(l.WL.SelectedWallet.Wallet)

	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.mixedAccountSelector.SelectFirstValidAccount(pg.WL.SelectedWallet.Wallet)
	pg.unmixedAccountSelector.SelectFirstValidAccount(pg.WL.SelectedWallet.Wallet)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) Layout(gtx layout.Context) layout.Dimensions {
	body := func(gtx C) D {
		page := components.SubPage{
			Load:       pg.Load,
			Title:      "Manual setup",
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return pg.Theme.Card().Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return pg.mixerAccountSections(gtx, "Mixed account", func(gtx layout.Context) layout.Dimensions {
										return pg.mixedAccountSelector.Layout(pg.ParentWindow(), gtx)
									})
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Top: values.MarginPaddingMinus15}.Layout(gtx, func(gtx C) D {
										return pg.mixerAccountSections(gtx, "Unmixed account", func(gtx layout.Context) layout.Dimensions {
											return pg.unmixedAccountSelector.Layout(pg.ParentWindow(), gtx)
										})
									})
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Top: values.MarginPadding10, Left: values.MarginPadding16, Right: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
										return layout.Flex{
											Axis: layout.Horizontal,
										}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												return pg.Theme.Icons.ActionInfo.Layout(gtx, pg.Theme.Color.Gray1)
											}),
											layout.Rigid(func(gtx C) D {
												txt := `<span style="text-color: grayText2">
											<b>Make sure to select the same accounts from the previous privacy setup. </b><br>Failing to do so could compromise wallet privacy.<br> You may not select the same account for mixed and unmixed.
										</span>`
												return layout.Inset{
													Left: values.MarginPadding8,
												}.Layout(gtx, renderers.RenderHTML(txt, pg.Theme).Layout)
											}),
										)
									})
								}),
							)
						}),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.UniformInset(values.MarginPadding15).Layout(gtx, pg.toPrivacySetup.Layout)
						}),
					)
				})
			},
		}
		return page.Layout(pg.ParentWindow(), gtx)
	}

	return components.UniformPadding(gtx, body)
}

func (pg *ManualMixerSetupPage) mixerAccountSections(gtx layout.Context, title string, body layout.Widget) layout.Dimensions {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding8,
					}.Layout(gtx, pg.Theme.Body1(title).Layout)
				}),
				layout.Rigid(body),
			)
		})
	})
}

func (pg *ManualMixerSetupPage) showModalSetupMixerAcct() {
	if pg.mixedAccountSelector.SelectedAccount().Number == pg.unmixedAccountSelector.SelectedAccount().Number {
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrNotSameAccoutMixUnmix), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	passwordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title("Confirm to set mixer accounts").
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			errfunc := func(err error) bool {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			mixedAcctNumber := pg.mixedAccountSelector.SelectedAccount().Number
			unmixedAcctNumber := pg.unmixedAccountSelector.SelectedAccount().Number
			err := pg.dcrImpl.SetAccountMixerConfig(mixedAcctNumber, unmixedAcctNumber, password)
			if err != nil {
				return errfunc(err)
			}
			pg.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, true)

			// rename mixed account
			err = pg.WL.SelectedWallet.Wallet.RenameAccount(mixedAcctNumber, "mixed")
			if err != nil {
				return errfunc(err)
			}

			// rename unmixed account
			err = pg.WL.SelectedWallet.Wallet.RenameAccount(unmixedAcctNumber, "unmixed")
			if err != nil {
				return errfunc(err)
			}

			pm.Dismiss()

			pg.ParentNavigator().Display(NewAccountMixerPage(pg.Load))

			return true
		})
	pg.ParentWindow().ShowModal(passwordModal)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) HandleUserInteractions() {
	if pg.toPrivacySetup.Clicked() {
		go pg.showModalSetupMixerAcct()
	}

	if pg.mixedAccountSelector.SelectedAccount().Number == pg.unmixedAccountSelector.SelectedAccount().Number {
		pg.toPrivacySetup.SetEnabled(false)
	} else {
		pg.toPrivacySetup.SetEnabled(true)
	}

	// Disable set up button if either mixed or unmixed account is the default account.
	if pg.mixedAccountSelector.SelectedAccount().Number == dcr.DefaultAccountNum ||
		pg.unmixedAccountSelector.SelectedAccount().Number == dcr.DefaultAccountNum {
		pg.toPrivacySetup.SetEnabled(false)
	}

}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) OnNavigatedFrom() {
	pg.ctxCancel()
}
