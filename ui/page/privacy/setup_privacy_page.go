package privacy

import (
	"gioui.org/layout"
	"gioui.org/text"

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
	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

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
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{
			Top:    values.MarginPadding22,
			Bottom: values.MarginPadding22,
			Left:   values.MarginPadding24,
			Right:  values.MarginPadding24,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.W.Layout(gtx, func(gtx C) D {
						return pg.Theme.H6(values.String(values.StrStakeShuffle)).Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top:    values.MarginPadding24,
						Bottom: values.MarginPadding24,
					}.Layout(gtx, func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Left: values.MarginPadding5,
									}.Layout(gtx, pg.Theme.Icons.TransactionFingerprint.Layout48dp)
								}),
								layout.Rigid(func(gtx C) D {
									return pg.Theme.Icons.ArrowForward.LayoutSize2(gtx, values.MarginPadding24, values.MarginPadding10)
								}),
								layout.Rigid(func(gtx C) D {
									return pg.Theme.Icons.Mixer.LayoutSize(gtx, values.MarginPadding120)
								}),
								layout.Rigid(func(gtx C) D {
									return pg.Theme.Icons.ArrowForward.LayoutSize2(gtx, values.MarginPadding24, values.MarginPadding10)
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Left: values.MarginPadding5,
									}.Layout(gtx, pg.Theme.Icons.TransactionsIcon.Layout48dp)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left:  values.MarginPadding24,
						Right: values.MarginPadding24,
					}.Layout(gtx, func(gtx C) D {
						introA := pg.Theme.H6(values.String(values.StrSetUpStakeShuffleIntro))
						introB := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleIntroDesc))
						introC := pg.Theme.Body1(values.String(values.StrSetUpStakeShuffleIntroSubDesc))
						introA.Alignment, introB.Alignment, introC.Alignment = text.Middle, text.Middle, text.Middle
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(introA.Layout),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, introB.Layout)
							}),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, introC.Layout)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.UniformInset(values.MarginPadding30).Layout(gtx, pg.toPrivacySetup.Layout)
				}),
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
