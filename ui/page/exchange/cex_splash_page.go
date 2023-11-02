package exchange

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *CreateOrderPage) initSplashPageWidgets() {
	_, pg.splashPageInfoButton = components.SubpageHeaderButtons(pg.Load)
	pg.enableDEXBtn = pg.Theme.Button(values.String(values.StrBack))
}

func (pg *CreateOrderPage) splashPage(gtx layout.Context) layout.Dimensions {
	return cryptomaterial.LinearLayout{
		Orientation: layout.Vertical,
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Background:  pg.Theme.Color.Surface,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
		Padding:     layout.UniformInset(values.MarginPadding20),
	}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.Theme.Icons.TradeExchange.LayoutSize(gtx, values.MarginPadding100)
						}),
						layout.Rigid(func(gtx C) D {
							pgTitle := pg.Theme.Label(values.TextSize24, values.String(values.StrWhatIsCex))
							pgTitle.Font.Weight = font.SemiBold

							return layout.Inset{
								Top:    values.MarginPadding26,
								Bottom: values.MarginPadding12,
							}.Layout(gtx, pgTitle.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							pgContent := pg.Theme.Label(values.TextSize16, values.String(values.StrCexContent))
							pgContent.Alignment = text.Middle
							return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, pgContent.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							pgQuestion := pg.Theme.Label(values.TextSize16, values.String(values.StrWouldTradeCex))
							return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, pgQuestion.Layout)
						}),
					)
				}),
				layout.Stacked(pg.splashPageInfoButton.Layout),
			)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding350)
			return layout.Inset{
				Top:   values.MarginPadding20,
				Right: values.MarginPadding16,
			}.Layout(gtx, pg.navToSettingsBtn.Layout)
		}),
	)
}

func (pg *CreateOrderPage) showInfoModal() {
	info := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrCentralizedExchangeCex)).
		Body(values.String(values.StrDexInfo)).
		SetPositiveButtonText(values.String(values.StrGotIt))
	pg.ParentWindow().ShowModal(info)
}
