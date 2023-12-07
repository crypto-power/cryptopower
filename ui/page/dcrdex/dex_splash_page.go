package dcrdex

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *DEXPage) splashPage(gtx C) D {
	return cryptomaterial.LinearLayout{
		Orientation: layout.Vertical,
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Background:  pg.Theme.Color.Surface,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
		Padding:     layout.Inset{Top: values.MarginPadding30, Bottom: values.MarginPadding30, Right: values.MarginPadding24, Left: values.MarginPadding24},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: values.MarginPadding50}.Layout(gtx, func(gtx C) D {
				return layout.Stack{Alignment: layout.NE}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
									return pg.Theme.Icons.DcrDex.LayoutSize(gtx, values.MarginPadding100)
								})
							}),
							layout.Rigid(func(gtx C) D {
								pgTitle := pg.Theme.Label(values.TextSize24, values.String(values.StrWhatIsDex))
								pgTitle.Font.Weight = font.SemiBold
								return layout.Inset{Top: values.MarginPadding30, Bottom: values.MarginPadding16}.Layout(gtx, pgTitle.Layout)
							}),
							layout.Rigid(func(gtx C) D {
								pgContent := pg.Theme.Label(values.TextSize16, values.String(values.StrDexContent))
								pgContent.Alignment = text.Middle
								return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, pgContent.Layout)
							}),
						)
					}),
					layout.Stacked(pg.splashPageInfoButton.Layout),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: values.MarginPadding15, Left: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx, layout.Flexed(1, pg.startTradingBtn.Layout))
			})
		}),
	)
}

func (pg *DEXPage) showInfoModal() {
	info := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrDecentralized)).
		Body(values.String(values.StrTradeSettingsMsg)).
		SetPositiveButtonText(values.String(values.StrGotIt))
	pg.ParentWindow().ShowModal(info)
}
