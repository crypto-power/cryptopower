package seedbackup

import (
	"gioui.org/font"
	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const BackupSuccessPageID = "backup_success"

type BackupSuccessPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	actionButton     cryptomaterial.Button
	redirectCallback Redirectfunc
}

func NewBackupSuccessPage(l *load.Load, redirect Redirectfunc) *BackupSuccessPage {
	pg := &BackupSuccessPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(BackupSuccessPageID),

		actionButton:     l.Theme.OutlineButton(values.String(values.StrBackToWallets)),
		redirectCallback: redirect,
	}
	pg.actionButton.Font.Weight = font.Medium

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *BackupSuccessPage) OnNavigatedTo() {}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *BackupSuccessPage) HandleUserInteractions() {
	for pg.actionButton.Clicked() {
		pg.redirectCallback(pg.Load, pg.ParentWindow())
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *BackupSuccessPage) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *BackupSuccessPage) Layout(gtx C) D {
	return cryptomaterial.UniformPadding(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.MatchParent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.MatchParent,
					Orientation: layout.Vertical,
					Alignment:   layout.Middle,
					Direction:   layout.Center,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						successIcon := cryptomaterial.NewIcon(pg.Theme.Icons.ActionCheckCircle)
						return successIcon.Layout(gtx, values.MarginPadding64)
					}),
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.Label(values.TextSize24, values.String(values.StrSeedPhraseVerified))
						label.Color = pg.Theme.Color.DeepBlue

						return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, label.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.Label(values.TextSize16, values.String(values.StrSureToSafeStoreSeed))
						label.Color = pg.Theme.Color.GrayText1

						return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, label.Layout)
					}),
				)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X

				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, pg.actionButton.Layout)
			}),
		)
	})
}
