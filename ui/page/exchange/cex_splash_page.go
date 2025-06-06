package exchange

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *CreateOrderPage) splashPage(gtx C) D {
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
			return layout.Inset{Bottom: values.MarginPadding30}.Layout(gtx, func(gtx C) D {
				return layout.Stack{Alignment: layout.NE}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, func(gtx C) D {
									size := values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding100)
									return pg.Theme.Icons.TradeExchangeIcon.LayoutSize(gtx, size)
								})
							}),
							layout.Rigid(func(gtx C) D {
								pgTitle := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize24), values.String(values.StrWhatIsCex))
								pgTitle.Font.Weight = font.SemiBold
								return layout.Inset{Top: values.MarginPadding26, Bottom: values.MarginPadding12}.Layout(gtx, pgTitle.Layout)
							}),
							layout.Rigid(func(gtx C) D {
								pgContent := pg.Theme.Label(textSize16, values.String(values.StrCexContent))
								pgContent.Alignment = text.Middle
								return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, pgContent.Layout)
							}),
							layout.Rigid(func(gtx C) D {
								pgQuestion := pg.Theme.Label(textSize16, values.String(values.StrWouldTradeCex))
								return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, pgQuestion.Layout)
							}),
						)
					}),
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
