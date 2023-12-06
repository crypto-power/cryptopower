package send

import (
	// "fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	// libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

func (pg *Page) initLayoutWidgets() {
	pg.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx C) D {
	return pg.layoutDesktop(gtx)
}

func (pg *Page) layoutDesktop(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.sendLayout,
	}

	return pg.Theme.List(pg.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
		return layout.Inset{Bottom: values.MarginPadding32}.Layout(gtx, pageContent[i])
	})
}

func (pg *Page) sendLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Padding:     layout.UniformInset(values.MarginPadding16),
		Background:  pg.Theme.Color.Surface,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Rigid(pg.titleLayout),
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.Label(values.TextSize16, values.String(values.StrFrom))
			lbl.Color = pg.Theme.Color.GrayText2
			return layout.Inset{
				Top:    values.MarginPadding16,
				Bottom: values.MarginPadding16,
			}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding4,
					}.Layout(gtx, pg.Theme.Label(values.TextSize16, values.String(values.StrAccount)).Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.sourceAccountSelector.Layout(pg.ParentWindow(), gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			if pg.selectedWallet.IsSynced() {
				return D{}
			}
			txt := pg.Theme.Label(values.TextSize14, values.String(values.StrFunctionUnavailable))
			txt.Font.Weight = font.SemiBold
			txt.Color = pg.Theme.Color.Danger
			return txt.Layout(gtx)
		}),
	)
}

func (pg *Page) titleLayout(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding6,
			}.Layout(gtx, pg.Theme.Label(values.TextSize20, values.String(values.StrSend)).Layout)
		}),
		layout.Rigid(pg.infoButton.Layout),
	)
}
