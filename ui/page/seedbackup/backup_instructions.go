package seedbackup

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const BackupInstructionsPageID = "backup_instructions"

type (
	C = layout.Context
	D = layout.Dimensions
)

type Redirectfunc func(load *load.Load, pg app.WindowNavigator)

type BackupInstructionsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet sharedW.Asset

	backButton  cryptomaterial.IconButton
	viewSeedBtn cryptomaterial.Button
	checkBoxes  []cryptomaterial.CheckBoxStyle
	infoList    *layout.List

	redirectCallback Redirectfunc
}

func NewBackupInstructionsPage(l *load.Load, wallet sharedW.Asset, redirect Redirectfunc) *BackupInstructionsPage {
	bi := &BackupInstructionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(BackupInstructionsPageID),
		wallet:           wallet,

		viewSeedBtn: l.Theme.Button(values.String(values.StrViewSeedPhrase)),

		redirectCallback: redirect,
	}

	bi.viewSeedBtn.Font.Weight = font.Medium

	bi.backButton, _ = components.SubpageHeaderButtons(l)
	bi.backButton.Icon = l.Theme.Icons.ContentClear

	bi.checkBoxes = []cryptomaterial.CheckBoxStyle{
		l.Theme.CheckBox(new(widget.Bool), values.String(values.StrImportantSeedPhrase)),
		l.Theme.CheckBox(new(widget.Bool), values.String(values.StrSeedPhraseToRestore)),
		l.Theme.CheckBox(new(widget.Bool), values.String(values.StrHowToStoreSeedPhrase)),
		l.Theme.CheckBox(new(widget.Bool), values.String(values.StrHowNotToStoreSeedPhrase)),
		l.Theme.CheckBox(new(widget.Bool), values.String(values.StrHideSeedPhrase)),
	}

	for i := range bi.checkBoxes {
		bi.checkBoxes[i].TextSize = values.TextSize16
	}

	bi.infoList = &layout.List{Axis: layout.Vertical}

	return bi
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *BackupInstructionsPage) OnNavigatedTo() {
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *BackupInstructionsPage) HandleUserInteractions() {
	for pg.viewSeedBtn.Clicked() {
		if pg.verifyCheckBoxes() {
			// TODO: Will repeat the paint cycle, just queue the next fragment to be displayed
			pg.ParentNavigator().Display(NewSaveSeedPage(pg.Load, pg.wallet, pg.redirectCallback))
		}
	}
}

func promptToExit(load *load.Load, window app.WindowNavigator, redirect Redirectfunc) {
	infoModal := modal.NewCustomModal(load).
		Title(values.String(values.StrExit) + "?").
		Body(values.String(values.StrSureToExitBackup)).
		SetNegativeButtonText(values.String(values.StrNo)).
		SetPositiveButtonText(values.String(values.StrYes)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			redirect(load, window)
			return true
		})
	window.ShowModal(infoModal)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *BackupInstructionsPage) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *BackupInstructionsPage) Layout(gtx layout.Context) layout.Dimensions {
	sp := components.SubPage{
		Load:       pg.Load,
		Title:      values.String(values.StrKeepInMind),
		BackButton: pg.backButton,
		Back: func() {
			promptToExit(pg.Load, pg.ParentWindow(), pg.redirectCallback)
		},
		Body: func(gtx C) D {
			return pg.infoList.Layout(gtx, len(pg.checkBoxes), func(gtx C, i int) D {
				return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, pg.checkBoxes[i].Layout)
			})
		},
	}

	pg.viewSeedBtn.SetEnabled(pg.verifyCheckBoxes())

	layout := func(gtx C) D {
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	isMobile := pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView)
	return container(gtx, isMobile, *pg.Theme, layout, "", pg.viewSeedBtn, true)
}

func (pg *BackupInstructionsPage) verifyCheckBoxes() bool {
	for _, cb := range pg.checkBoxes {
		if !cb.CheckBox.Value {
			return false
		}
	}
	return true
}

func container(gtx C, isMobile bool, theme cryptomaterial.Theme, body layout.Widget, infoText string, actionBtn cryptomaterial.Button, showActionBtn bool) D {
	bodyLayout := func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return body(gtx)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				if !showActionBtn {
					return D{}
				}
				gtx.Constraints.Min = gtx.Constraints.Max
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Vertical,
					Direction:   layout.S,
					Alignment:   layout.Baseline,
					Background:  theme.Color.Surface,
					Shadow:      theme.Shadow(),
					Padding:     layout.UniformInset(values.MarginPadding16),
					Margin:      layout.Inset{Left: values.Size0_5},
					Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(4)},
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if !utils.StringNotEmpty(infoText) {
							return D{}
						}
						label := theme.Label(values.TextSize14, infoText)
						label.Color = theme.Color.GrayText1
						return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, label.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return actionBtn.Layout(gtx)
					}))
			}),
		)
	}
	if isMobile {
		return components.UniformMobile(gtx, false, false, bodyLayout)
	}
	return components.UniformPadding(gtx, bodyLayout)
}
