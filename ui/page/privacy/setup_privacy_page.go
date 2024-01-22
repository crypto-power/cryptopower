package privacy

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const SetupPrivacyPageID = "SetupPrivacy"

type (
	C = layout.Context
	D = layout.Dimensions
)

type SetupPrivacyPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	wallet *dcr.Asset

	toPrivacySetup cryptomaterial.Button

	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton
}

func NewSetupPrivacyPage(l *load.Load, wallet *dcr.Asset) *SetupPrivacyPage {
	pg := &SetupPrivacyPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(SetupPrivacyPageID),
		wallet:           wallet,
		toPrivacySetup:   l.Theme.Button(values.String(values.StrSetUpStakeShuffleIntroButton)),
	}
	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.backButton = components.GetBackButtons(l)

	return pg

}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SetupPrivacyPage) OnNavigatedTo() {}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SetupPrivacyPage) Layout(gtx C) D {
	stakeShuffleDesc := fmt.Sprintf("%s\n%s",
		values.String(values.StrSetUpStakeShuffleIntroDesc),
		values.String(values.StrSetUpStakeShuffleIntroSubDesc))

	inset := layout.Inset{
		Top:    values.MarginPadding24,
		Bottom: values.MarginPadding24,
		Left:   values.MarginPadding24,
		Right:  values.MarginPadding24,
	}
	spaceBetweenWidgets := layout.Spacer{Height: values.MarginPadding24}
	introA := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleIntro))
	introB := pg.Theme.Body1(stakeShuffleDesc)
	introA.Alignment, introB.Alignment = text.Middle, text.Middle
	spaceBetweenTexts := layout.Spacer{Height: values.MarginPadding24}
	pg.toPrivacySetup.TextSize = values.TextSize16

	if pg.IsMobileView() {
		inset = layout.Inset{
			Top:    values.MarginPadding16,
			Bottom: values.MarginPadding32,
			Left:   values.MarginPadding16,
			Right:  values.MarginPadding16,
		}
		spaceBetweenWidgets = layout.Spacer{Height: values.MarginPadding32}
		introA.TextSize, introB.TextSize = values.TextSize12, values.TextSize12
		spaceBetweenTexts = layout.Spacer{Height: values.MarginPadding16}
		pg.toPrivacySetup.TextSize = values.TextSize14
	}

	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.W.Layout(gtx, func(gtx C) D {
						lbl := pg.Theme.H6(values.String(values.StrStakeShuffle))
						if pg.IsMobileView() {
							lbl.TextSize = values.TextSize16
						}
						return lbl.Layout(gtx)
					})
				}),
				layout.Rigid(spaceBetweenWidgets.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						// image sizes are 20% smaller in mobile view
						imageSizeScale := unit.Dp(1)
						if pg.IsMobileView() {
							imageSizeScale = unit.Dp(0.8)
						}
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
									imageSize := values.MarginPadding48 * imageSizeScale
									return pg.Theme.Icons.TransactionFingerprint.LayoutSize(gtx, imageSize)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Icons.ArrowForward.LayoutSize2(gtx, values.MarginPadding24*imageSizeScale, values.MarginPadding10*imageSizeScale)
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Icons.Mixer.LayoutSize(gtx, values.MarginPadding120*imageSizeScale)
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Icons.ArrowForward.LayoutSize2(gtx, values.MarginPadding24*imageSizeScale, values.MarginPadding10*imageSizeScale)
							}),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
									imageSize := values.MarginPadding48 * imageSizeScale
									return pg.Theme.Icons.TransactionsIcon.LayoutSize(gtx, imageSize)
								})
							}),
						)
					})
				}),
				layout.Rigid(spaceBetweenWidgets.Layout),
				layout.Rigid(introA.Layout),
				layout.Rigid(spaceBetweenTexts.Layout),
				layout.Rigid(introB.Layout),
				layout.Rigid(spaceBetweenWidgets.Layout),
				layout.Rigid(pg.toPrivacySetup.Layout),
			)
		})
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *SetupPrivacyPage) HandleUserInteractions() {
	if pg.toPrivacySetup.Clicked() {
		pg.ParentNavigator().Display(NewSetupMixerAccountsPage(pg.Load, pg.wallet))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *SetupPrivacyPage) OnNavigatedFrom() {}
