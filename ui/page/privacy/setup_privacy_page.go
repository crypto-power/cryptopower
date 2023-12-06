package privacy

import (
	"context"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

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

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	pageContainer  layout.List
	toPrivacySetup cryptomaterial.Button

	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton
}

func NewSetupPrivacyPage(l *load.Load, wallet *dcr.Asset) *SetupPrivacyPage {
	pg := &SetupPrivacyPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(SetupPrivacyPageID),
		wallet:           wallet,
		pageContainer:    layout.List{Axis: layout.Vertical},
		toPrivacySetup:   l.Theme.Button(values.String(values.StrSetupStakeShuffle)),
	}
	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	return pg

}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SetupPrivacyPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SetupPrivacyPage) Layout(gtx layout.Context) layout.Dimensions {
	return cryptomaterial.UniformPadding(gtx, pg.privacyIntroLayout)
}

func (pg *SetupPrivacyPage) privacyIntroLayout(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: values.MarginPadding40}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding25}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Bottom: values.MarginPadding24,
									}.Layout(gtx, func(gtx C) D {
										return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												return layout.Inset{
													Left: values.MarginPadding5,
												}.Layout(gtx, pg.Theme.Icons.TransactionFingerprint.Layout48dp)
											}),
											layout.Rigid(pg.Theme.Icons.ArrowForward.Layout24dp),
											layout.Rigid(func(gtx C) D {
												return pg.Theme.Icons.Mixer.LayoutSize(gtx, values.MarginPadding120)
											}),
											layout.Rigid(pg.Theme.Icons.ArrowForward.Layout24dp),
											layout.Rigid(func(gtx C) D {
												return layout.Inset{
													Left: values.MarginPadding5,
												}.Layout(gtx, pg.Theme.Icons.TransactionsIcon.Layout48dp)
											}),
										)
									})
								}),
								layout.Rigid(func(gtx C) D {
									title := pg.Theme.H6(values.String(values.StrStakeShuffle))
									subtitle := pg.Theme.Body1(values.String(values.StrSetUpPrivacy))

									title.Alignment, subtitle.Alignment = text.Middle, text.Middle

									return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(title.Layout),
										layout.Rigid(func(gtx C) D {
											return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, subtitle.Layout)
										}),
									)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
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
		accounts, err := pg.wallet.GetAccountsRaw()
		if err != nil {
			log.Error(err)
		}

		walCount := len(accounts.Accounts)
		// Filter out imported account and default account.
		for _, v := range accounts.Accounts {
			if v.Number == dcr.ImportedAccountNumber || v.Number == dcr.DefaultAccountNum {
				walCount--
			}
		}

		if walCount <= 1 {
			go showModalSetupMixerInfo(&sharedModalConfig{
				Load:          pg.Load,
				window:        pg.ParentWindow(),
				pageNavigator: pg.ParentNavigator(),
				checkBox:      pg.Theme.CheckBox(new(widget.Bool), values.String(values.StrMoveFundsFrmDefaultToUnmixed)),
			}, pg.wallet)
		} else {
			pg.ParentNavigator().Display(NewSetupMixerAccountsPage(pg.Load, pg.wallet))
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
func (pg *SetupPrivacyPage) OnNavigatedFrom() {
	pg.ctxCancel()
}
