package renderers

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/values"
)

func getLabel(lbl cryptomaterial.Label) cryptomaterial.Label {
	return lbl
}

func setStyle(lbl *cryptomaterial.Label, style string) {
	var s text.Style

	switch style {
	case "italic":
		s = text.Italic
	case "regular":
		s = text.Regular
	}

	lbl.Font.Style = s
}

func setWeight(lbl *cryptomaterial.Label, weight string) {
	var w text.Weight

	switch weight {
	case "normal":
		w = text.Normal
	case "medium":
		w = text.Medium
	case "bold", "strong":
		w = text.Bold
	default:
		w = lbl.Font.Weight
	}

	lbl.Font.Weight = w
}

func getHeading(txt string, level int, theme *cryptomaterial.Theme) cryptomaterial.Label {
	textSize := values.TextSize16

	switch level {
	case 1:
		textSize = values.TextSize28
	case 2:
		textSize = values.TextSize24
	case 3:
		textSize = values.TextSize20
	case 4:
		textSize = values.TextSize16
	case 5:
		textSize = values.TextSize14
	case 6:
		textSize = values.TextSize13_6
	}

	lbl := theme.H1(txt)
	lbl.Font.Weight = text.Bold
	lbl.TextSize = textSize
	return lbl
}

func renderStrike(lbl cryptomaterial.Label, theme *cryptomaterial.Theme) layout.Widget {
	return func(gtx C) D {
		var dims D
		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx C) D {
				dims = lbl.Layout(gtx)
				return dims
			}),
			layout.Expanded(func(gtx C) D {
				return layout.Inset{
					Top: unit.Dp((float32(dims.Size.Y) / float32(2))),
				}.Layout(gtx, func(gtx C) D {
					l := theme.Separator()
					l.Color = lbl.Color
					l.Width = dims.Size.X
					return l.Layout(gtx)
				})
			}),
		)
	}
}

func renderBlockQuote(lbl cryptomaterial.Label, theme *cryptomaterial.Theme) layout.Widget {
	words := strings.Fields(lbl.Text)

	return func(gtx C) D {
		var dims D

		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				l := theme.SeparatorVertical(dims.Size.Y, 10)
				l.Color = theme.Color.Gray2
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				dims = layout.Inset{
					Left: unit.Dp(4),
				}.Layout(gtx, func(gtx C) D {
					return cryptomaterial.GridWrap{
						Axis:      layout.Horizontal,
						Alignment: layout.Start,
					}.Layout(gtx, len(words), func(gtx C, i int) D {
						lbl.Text = words[i] + " "
						return lbl.Layout(gtx)
					})
				})

				return dims
			}),
		)
	}
}

func renderHorizontalLine(theme *cryptomaterial.Theme) layout.Widget {
	l := theme.Separator()
	l.Width = 1
	return l.Layout
}

func renderEmptyLine(theme *cryptomaterial.Theme, isList bool) layout.Widget {
	var padding = -5

	if isList {
		padding = -10
	}

	return func(gtx C) D {
		dims := theme.Body2("").Layout(gtx)
		dims.Size.Y = dims.Size.Y + padding
		return dims
	}
}
