package governance

import (
	"gioui.org/layout"
	"gioui.org/text"

	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/renderers"
	"gitlab.com/raedah/cryptopower/ui/values"
)

func (pg *Page) initSplashScreenWidgets() {
	_, pg.splashScreenInfoButton = components.SubpageHeaderButtons(pg.Load)
	pg.enableGovernanceBtn = pg.Theme.Button(values.String(values.StrFetchProposals))
}

func (pg *Page) splashScreenLayout(gtx layout.Context) layout.Dimensions {
	return cryptomaterial.LinearLayout{
		Orientation: layout.Vertical,
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Background:  pg.Theme.Color.Surface,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
		Padding:     layout.UniformInset(values.MarginPadding24)}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.Theme.Icons.GovernanceActiveIcon.LayoutSize(gtx, values.MarginPadding150)
						}),
						layout.Rigid(func(gtx C) D {
							txt := pg.Theme.Label(values.TextSize24, values.String(values.StrHowGovernanceWork))
							txt.Font.Weight = text.SemiBold

							return layout.Inset{
								Top:    values.MarginPadding30,
								Bottom: values.MarginPadding16,
							}.Layout(gtx, txt.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							text := values.StringF(values.StrGovernanceInfo, `<span style="text-color: gray">`, `<br>`, `</span>`)
							return renderers.RenderHTML(text, pg.Theme).Layout(gtx)
						}),
					)
				}),
				layout.Stacked(pg.splashScreenInfoButton.Layout),
			)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding350)
			return layout.Inset{
				Top:   values.MarginPadding24,
				Right: values.MarginPadding16,
			}.Layout(gtx, pg.enableGovernanceBtn.Layout)
		}),
	)
}

func (pg *Page) showInfoModal() {
	info := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrGovernance)).
		Body(values.String(values.StrProposalInfo)).
		PositiveButton(values.String(values.StrGotIt), modal.DefaultClickFunc())
	pg.ParentWindow().ShowModal(info)
}
