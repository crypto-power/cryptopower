package privacy

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
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

	mixedAccountSelector   *components.AccountDropdown
	unmixedAccountSelector *components.AccountDropdown

	backButton     cryptomaterial.IconButton
	infoButton     cryptomaterial.IconButton
	backClickable  *cryptomaterial.Clickable
	toPrivacySetup cryptomaterial.Button
	backIcon       *cryptomaterial.Icon

	pageContainer *widget.List

	dcrWallet *dcr.Asset
}

func NewManualMixerSetupPage(l *load.Load, dcrWallet *dcr.Asset) *ManualMixerSetupPage {
	pg := &ManualMixerSetupPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ManualMixerSetupPageID),
		toPrivacySetup:   l.Theme.Button(values.String(values.StrSetUp)),
		dcrWallet:        dcrWallet,
		pageContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
	pg.backClickable = pg.Theme.NewClickable(true)
	pg.backIcon = cryptomaterial.NewIcon(pg.Theme.Icons.NavigationArrowBack)
	pg.backIcon.Color = pg.Theme.Color.Gray1

	// Mixed account picker
	pg.mixedAccountSelector = components.NewAccountDropdown(l).
		SetChangedCallback(func(_ *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			if pg.unmixedAccountSelector == nil {
				return true
			}
			wal := pg.Load.AssetsManager.WalletWithID(account.WalletID)

			var unmixedAccNo int32 = -1
			if unmixedAcc := pg.unmixedAccountSelector.SelectedAccount(); unmixedAcc != nil {
				unmixedAccNo = unmixedAcc.Number
			}

			// Imported, watch only and default wallet accounts are invalid to use as a mixed account
			// TODO: Watching only wallet should not even see this page, if they can't select an account!
			accountIsValid := account.Number != load.MaxInt32 && !wal.IsWatchingOnlyWallet() && account.Number != dcr.DefaultAccountNum

			if !accountIsValid || account.Number == unmixedAccNo {
				return false
			}

			return true
		}).
		Setup(dcrWallet)

	// Unmixed account picker
	pg.unmixedAccountSelector = components.NewAccountDropdown(l).
		SetChangedCallback(func(_ *sharedW.Account) {}).
		AccountValidator(func(account *sharedW.Account) bool {
			wal := pg.Load.AssetsManager.WalletWithID(account.WalletID)

			var mixedAccNo int32 = -1
			if mixedAcc := pg.mixedAccountSelector.SelectedAccount(); mixedAcc != nil {
				mixedAccNo = mixedAcc.Number
			}

			// Imported, watch only and default wallet accounts are invalid to use as an unmixed account
			accountIsValid := account.Number != load.MaxInt32 && !wal.IsWatchingOnlyWallet() && account.Number != dcr.DefaultAccountNum && !utils.IsImportedAccount(dcrWallet.GetAssetType(), account)

			// Account is invalid if already selected by mixed account selector.
			if !accountIsValid || account.Number == mixedAccNo {
				return false
			}

			return true
		}).
		Setup(dcrWallet)

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.backButton = components.GetBackButton(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) OnNavigatedTo() {
	_ = pg.mixedAccountSelector.Setup(pg.dcrWallet)
	_ = pg.unmixedAccountSelector.Setup(pg.dcrWallet)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ManualMixerSetupPage) Layout(gtx C) D {
	pg.toPrivacySetup.TextSize = values.TextSize16
	if pg.IsMobileView() {
		pg.toPrivacySetup.TextSize = values.TextSize14
	}

	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		inset := layout.UniformInset(values.MarginPadding24)
		if pg.IsMobileView() {
			inset.Left, inset.Right = values.MarginPadding16, values.MarginPadding16
		}

		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return inset.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
				// back button - "Manual Setup" label
				layout.Rigid(pg.backButtonAndPageHeading),
				// 24px/16px space (mobile/desktop)
				layout.Rigid(func(gtx C) D {
					return pg.Theme.List(pg.pageContainer).Layout(gtx, 1, func(gtx C, _ int) D {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if pg.IsMobileView() {
									return layout.Spacer{Height: values.MarginPadding24}.Layout(gtx)
								}
								return layout.Spacer{Height: values.MarginPadding16}.Layout(gtx)
							}),
							// "Mixed account" label, 4px space, mixed account dropdown
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
									return pg.mixedAccountSelector.Layout(gtx, values.String(values.StrMixedAccount))
								})
							}),

							// 16px/12px space (mobile/desktop)
							layout.Rigid(func(gtx C) D {
								if pg.IsMobileView() {
									return layout.Spacer{Height: values.MarginPadding16}.Layout(gtx)
								}
								return layout.Spacer{Height: values.MarginPadding12}.Layout(gtx)
							}),
							// "Unmixed account" label, 4px space, unmixed account dropdown
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
									return pg.unmixedAccountSelector.Layout(gtx, values.String(values.StrUnmixedAccount))
								})
							}),
							// 24px space, then warning/caution text
							layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout),
							layout.Rigid(pg.cautionCard),
							// 40px/60px space (mobile/desktop), then "Set up" button.
							layout.Rigid(func(gtx C) D {
								if pg.IsMobileView() {
									return layout.Spacer{Height: values.MarginPadding40}.Layout(gtx)
								}
								return layout.Spacer{Height: values.MarginPadding60}.Layout(gtx)
							}),
							layout.Rigid(pg.toPrivacySetup.Layout),
						)
					})
				}),
			)
		})
	})
}

func (pg *ManualMixerSetupPage) backButtonAndPageHeading(gtx C) D {
	// Setting a minimum Y larger than the label allows it to be centered.
	// gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding50)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.backClickable.Layout(gtx, func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
					iconSize := values.MarginPadding30
					if pg.IsMobileView() {
						iconSize = values.MarginPadding18
					}
					return pg.backIcon.Layout(gtx, iconSize)
				})
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				lbl := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleManualTitle))
				if pg.IsMobileView() {
					lbl.TextSize = values.TextSize16
				}
				return lbl.Layout(gtx)
			})
		}),
	)
}

func (pg *ManualMixerSetupPage) cautionCard(gtx C) D {
	card := pg.Theme.Card()
	card.Color = pg.Theme.Color.Gray4
	return card.Layout(gtx, func(gtx C) D {
		inset := layout.UniformInset(values.MarginPadding16)
		if pg.IsMobileView() {
			inset.Left, inset.Right = values.MarginPadding8, values.MarginPadding8
		}
		return inset.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return pg.Theme.Icons.ActionInfo.Layout(gtx, pg.Theme.Color.Gray1)
				}),
				layout.Rigid(layout.Spacer{Width: values.MarginPadding12}.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							label := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleWarningTitle))
							if pg.IsMobileView() {
								label.TextSize = values.TextSize16
							}
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleWarningDesc))
							if pg.IsMobileView() {
								label.TextSize = values.TextSize14
							}
							return label.Layout(gtx)
						}),
					)
				}),
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
func (pg *ManualMixerSetupPage) HandleUserInteractions(gtx C) {
	if pg.backClickable.Clicked(gtx) {
		pg.ParentNavigator().CloseCurrentPage()
	}

	if pg.toPrivacySetup.Clicked(gtx) {
		go pg.showModalSetupMixerAcct()
	}

	mixed, unmixed := pg.mixedAccountSelector.SelectedAccount(), pg.unmixedAccountSelector.SelectedAccount()
	validMixedAccountSelected := mixed != nil && mixed.Number != dcr.DefaultAccountNum
	validUnmixedAccountSelected := unmixed != nil && unmixed.Number != dcr.DefaultAccountNum
	if validMixedAccountSelected && validUnmixedAccountSelected && mixed.Number != unmixed.Number {
		pg.toPrivacySetup.SetEnabled(true)
	} else {
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
func (pg *ManualMixerSetupPage) OnNavigatedFrom() {}
