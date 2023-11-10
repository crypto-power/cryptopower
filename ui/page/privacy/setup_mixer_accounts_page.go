package privacy

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const SetupMixerAccountsPageID = "SetupMixerAccounts"

type SetupMixerAccountsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	dcrWallet *dcr.Asset

	backButton           cryptomaterial.IconButton
	infoButton           cryptomaterial.IconButton
	autoSetupClickable   *cryptomaterial.Clickable
	manualSetupClickable *cryptomaterial.Clickable
	nextIcon             *cryptomaterial.Icon

	manualEnabled bool
}

func NewSetupMixerAccountsPage(l *load.Load, dcrWallet *dcr.Asset) *SetupMixerAccountsPage {
	pg := &SetupMixerAccountsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(SetupMixerAccountsPageID),
		dcrWallet:        dcrWallet,
	}
	pg.nextIcon = cryptomaterial.NewIcon(pg.Theme.Icons.NavigationArrowForward)
	pg.nextIcon.Color = pg.Theme.Color.Gray1
	pg.autoSetupClickable = pg.Theme.NewClickable(true)
	pg.manualSetupClickable = pg.Theme.NewClickable(true)
	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)
	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SetupMixerAccountsPage) OnNavigatedTo() {
	accts, err := pg.dcrWallet.GetAccountsRaw()
	if err != nil {
		log.Errorf("Unable to get accounts to set up mixer: %v", err)
		return
	}
	pg.manualEnabled = len(accts.Accounts) > 2
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SetupMixerAccountsPage) Layout(gtx C) D {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{
			Top:   values.MarginPadding25,
			Left:  values.MarginPadding24,
			Right: values.MarginPadding24,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleAutoOrManualA))
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoOrManualB))
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					// TODO: Find a way to make Mixed and Unmixed bold while keeping to the theme.
					label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoOrManualC))
					return layout.Inset{
						Top: values.MarginPadding10,
					}.Layout(gtx, label.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoOrManualD))
					return layout.Inset{
						Top: values.MarginPadding10,
					}.Layout(gtx, label.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: values.MarginPadding80}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					line := pg.Theme.Separator()
					return line.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.autoSetupClickable.Layout(gtx, pg.autoSetupLayout)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.manualSetupClickable.Layout(gtx, pg.manualSetupLayout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: values.MarginPadding10}.Layout(gtx)
				}),
			)
		})
	})
}

func (pg *SetupMixerAccountsPage) autoSetupLayout(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding70)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return layout.Inset{
		Top:    values.MarginPadding10,
		Bottom: values.MarginPadding10,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Flexed(5, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleAutoTitle))
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoDesc))
						return label.Layout(gtx)
					}),
				)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Spacing: layout.SpaceAround}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding24,
						}.Layout(gtx, func(gtx C) D {
							return pg.nextIcon.Layout(gtx, values.MarginPadding30)
						})
					}),
				)
			}),
		)
	})
}

func (pg *SetupMixerAccountsPage) manualSetupLayout(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding70)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return layout.Inset{
		Top:    values.MarginPadding10,
		Bottom: values.MarginPadding10,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Flexed(5, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleManualTitle))
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleManualDesc))
						return label.Layout(gtx)
					}),
				)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Spacing: layout.SpaceAround}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding24,
						}.Layout(gtx, func(gtx C) D {
							return pg.nextIcon.Layout(gtx, values.MarginPadding30)
						})
					}),
				)
			}),
		)
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *SetupMixerAccountsPage) HandleUserInteractions() {
	if pg.autoSetupClickable.Clicked() {
		showModalSetupMixerInfo(&sharedModalConfig{
			Load:          pg.Load,
			window:        pg.ParentWindow(),
			pageNavigator: pg.ParentNavigator(),
			checkBox:      pg.Theme.CheckBox(new(widget.Bool), values.String(values.StrMoveFundsFrmDefaultToUnmixed)),
		}, pg.dcrWallet)
	}

	if pg.manualSetupClickable.Clicked() {
		if !pg.manualEnabled {
			notEnoughAccounts := values.String(values.StrNotEnoughAccounts)
			info := modal.NewErrorModal(pg.Load, notEnoughAccounts, modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(info)
		} else {
			pg.ParentNavigator().Display(NewManualMixerSetupPage(pg.Load, pg.dcrWallet))
		}
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *SetupMixerAccountsPage) OnNavigatedFrom() {}
