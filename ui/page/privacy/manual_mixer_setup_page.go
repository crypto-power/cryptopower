package privacy

import (
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const ManualMixerSetupPageID = "ManualMixerSetup"

type ManualMixerSetupPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	mixedAccountSelector   *components.WalletAndAccountSelector
	unmixedAccountSelector *components.WalletAndAccountSelector

	backButton     cryptomaterial.IconButton
	infoButton     cryptomaterial.IconButton
	backClickable  *cryptomaterial.Clickable
	toPrivacySetup cryptomaterial.Button
	backIcon       *cryptomaterial.Icon

	dcrWallet *dcr.Asset
}

func NewManualMixerSetupPage(l *load.Load, dcrWallet *dcr.Asset) *ManualMixerSetupPage {
	pg := &ManualMixerSetupPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ManualMixerSetupPageID),
		toPrivacySetup:   l.Theme.Button(values.String(values.StrSetUp)),
		dcrWallet:        dcrWallet,
	}
	pg.backClickable = pg.Theme.NewClickable(true)
	pg.backIcon = cryptomaterial.NewIcon(pg.Theme.Icons.NavigationArrowBack)
	pg.backIcon.Color = pg.Theme.Color.Gray1

	// Mixed account picker
	pg.mixedAccountSelector = components.NewWalletAndAccountSelector(l).
		Title(values.String(values.StrMixedAccount)).
		AccountSelected(func(selectedAccount *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			wal := pg.Load.AssetsManager.WalletWithID(account.WalletID)

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

	// Unmixed account picker
	pg.unmixedAccountSelector = components.NewWalletAndAccountSelector(l).
		Title(values.String(values.StrUnmixedAccount)).
		AccountSelected(func(selectedAccount *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			wal := pg.Load.AssetsManager.WalletWithID(account.WalletID)

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

	pg.mixedAccountSelector.SelectFirstValidAccount(dcrWallet)
	pg.unmixedAccountSelector.SelectFirstValidAccount(dcrWallet)

	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) OnNavigatedTo() {
	pg.mixedAccountSelector.SelectFirstValidAccount(pg.dcrWallet)
	pg.unmixedAccountSelector.SelectFirstValidAccount(pg.dcrWallet)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) Layout(gtx C) D {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding50)
						return pg.backClickable.Layout(gtx, pg.backLayout)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.mixerAccountSections(gtx, values.String(values.StrMixedAccount), func(gtx C) D {
								return pg.mixedAccountSelector.Layout(pg.ParentWindow(), gtx)
							})
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPaddingMinus15}.Layout(gtx, func(gtx C) D {
								return pg.mixerAccountSections(gtx, values.String(values.StrUnmixedAccount), func(gtx C) D {
									return pg.unmixedAccountSelector.Layout(pg.ParentWindow(), gtx)
								})
							})
						}),
						layout.Rigid(layout.Spacer{Height: values.MarginPadding15}.Layout),
						layout.Rigid(pg.cautionCard),
						layout.Rigid(layout.Spacer{Height: values.MarginPadding15}.Layout),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.UniformInset(values.MarginPadding15).Layout(gtx, pg.toPrivacySetup.Layout)
				}),
			)
		})
	})
}

func (pg *ManualMixerSetupPage) cautionCard(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{
		Left:  values.MarginPadding15,
		Right: values.MarginPadding15,
	}.Layout(gtx, func(gtx C) D {
		card := pg.Theme.Card()
		card.Color = pg.Theme.Color.Gray4
		return card.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding100)
			gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
			return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding40)
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return pg.Theme.Icons.ActionInfo.Layout(gtx, pg.Theme.Color.Gray1)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Left: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									label := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleWarningTitle))
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx C) D {
									label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleWarningDesc))
									return label.Layout(gtx)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func (pg *ManualMixerSetupPage) backLayout(gtx C) D {
	return layout.Inset{Right: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
		// Setting a minimum Y larger than the label allows it to be centered.
		gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding50)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Left:  values.MarginPadding15,
					Right: values.MarginPadding15,
				}.Layout(gtx, func(gtx C) D {
					return pg.backIcon.Layout(gtx, values.MarginPadding30)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return pg.Theme.H6(values.String(values.StrSetUpStakeShuffleManualTitle)).Layout(gtx)
				})
			}),
		)
	})
}

func (pg *ManualMixerSetupPage) mixerAccountSections(gtx C, title string, body layout.Widget) D {
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
		Title(values.String(values.StrConfirmToSetMixer)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			errfunc := func(err error) bool {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			mixedAcctNumber := pg.mixedAccountSelector.SelectedAccount().Number
			unmixedAcctNumber := pg.unmixedAccountSelector.SelectedAccount().Number
			err := pg.dcrWallet.SetAccountMixerConfig(mixedAcctNumber, unmixedAcctNumber, password)
			if err != nil {
				return errfunc(err)
			}

			// rename mixed account
			err = pg.dcrWallet.RenameAccount(mixedAcctNumber, values.String(values.StrMixed))
			if err != nil {
				return errfunc(err)
			}

			// rename unmixed account
			err = pg.dcrWallet.RenameAccount(unmixedAcctNumber, values.String(values.StrUnmixed))
			if err != nil {
				return errfunc(err)
			}

			pm.Dismiss()

			pg.ParentNavigator().Display(NewAccountMixerPage(pg.Load, pg.dcrWallet))

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
	if pg.backClickable.Clicked() {
		pg.ParentNavigator().CloseCurrentPage()
	}

	if pg.toPrivacySetup.Clicked() {
		go pg.showModalSetupMixerAcct()
	}
	enableToPriv := func() {
		mixed, unmixed := pg.mixedAccountSelector.SelectedAccount(), pg.unmixedAccountSelector.SelectedAccount()
		if mixed == nil || unmixed == nil {
			pg.toPrivacySetup.SetEnabled(false)
			return
		}

		if mixed.Number == unmixed.Number {
			pg.toPrivacySetup.SetEnabled(false)
			return
		}

		// Disable set up button if either mixed or unmixed account is the default account.
		if mixed.Number == dcr.DefaultAccountNum ||
			unmixed.Number == dcr.DefaultAccountNum {
			pg.toPrivacySetup.SetEnabled(false)
			return
		}
		pg.toPrivacySetup.SetEnabled(true)
	}
	enableToPriv()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) OnNavigatedFrom() {}
