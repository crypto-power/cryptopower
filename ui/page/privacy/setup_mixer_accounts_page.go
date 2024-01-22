package privacy

import (
	"gioui.org/layout"
	"gioui.org/unit"
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
	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.backButton = components.GetBackButtons(l)
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
		inset := layout.UniformInset(24)
		var headingTextSize, normalTextSize unit.Sp = 20, 16
		verticalSpace1 := values.MarginPadding12

		if pg.IsMobileView() {
			inset = layout.Inset{Top: 24, Bottom: 24, Left: 16, Right: 16}
			headingTextSize, normalTextSize = 16, 14
			verticalSpace1 = values.MarginPadding16
		}

		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return inset.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleAutoOrManualA))
					label.TextSize = headingTextSize
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: verticalSpace1}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoOrManualB))
					label.TextSize = normalTextSize
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: verticalSpace1}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					// TODO: Find a way to make Mixed and Unmixed bold while keeping to the theme.
					label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoOrManualC))
					label.TextSize = normalTextSize
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: verticalSpace1}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleAutoOrManualD))
					label.TextSize = normalTextSize
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if pg.IsMobileView() {
						return layout.Spacer{Height: values.MarginPadding40}.Layout(gtx)
					}
					return layout.Spacer{Height: values.MarginPadding80}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					line := pg.Theme.Separator()
					return line.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: values.MarginPadding24}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.autoSetupClickable.Layout(gtx, func(gtx C) D {
						return pg.layoutHeadingDescAndNextIcon(gtx,
							values.String(values.StrSetUpStakeShuffleAutoTitle),
							values.String(values.StrSetUpStakeShuffleAutoDesc))
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: values.MarginPadding24}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.manualSetupClickable.Layout(gtx, func(gtx C) D {
						return pg.layoutHeadingDescAndNextIcon(gtx,
							values.String(values.StrSetUpStakeShuffleManualTitle),
							values.String(values.StrSetUpStakeShuffleManualDesc))
					})
				}),
			)
		})
	})
}

func (pg *SetupMixerAccountsPage) layoutHeadingDescAndNextIcon(gtx C, heading, desc string) D {
	var headingTextSize, normalTextSize unit.Sp = 20, 16
	spacingBtwTexts := values.MarginPadding8
	if pg.IsMobileView() {
		headingTextSize, normalTextSize = 16, 14
		spacingBtwTexts = values.MarginPadding4
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.H6(heading)
					label.TextSize = headingTextSize
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Spacer{Height: spacingBtwTexts}.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Body1(desc)
					label.TextSize = normalTextSize
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Spacer{Width: values.MarginPadding24}.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.nextIcon.Layout(gtx, values.MarginPadding24)
		}),
	)
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
