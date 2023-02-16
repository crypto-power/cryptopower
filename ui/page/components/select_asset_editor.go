package components

import (
	"image/color"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

type SelectAssetEditor struct {
	*load.Load
	Edit              cryptomaterial.Editor
	AssetTypeSelector *AssetTypeSelector
}

func NewSelectAssetEditor(l *load.Load) *SelectAssetEditor {
	sae := &SelectAssetEditor{
		Edit:              l.Theme.Editor(new(widget.Editor), ""),
		Load:              l,
		AssetTypeSelector: NewAssetTypeSelector(l),
	}
	sae.Edit.Bordered = false
	sae.Edit.SelectionColor = color.NRGBA{}
	sae.AssetTypeSelector.DisableBorder()
	sae.AssetTypeSelector.SetHint("--")
	return sae
}

func (sae SelectAssetEditor) Layout(window app.WindowNavigator, gtx C) D {
	l := sae.Theme.SeparatorVertical(int(gtx.Metric.PxPerDp)*31, int(gtx.Metric.PxPerDp)*2)
	l.Color = sae.Theme.Color.Gray2
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Alignment: layout.Middle,
		Border: cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  sae.Theme.Color.Gray2,
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding100)
			return sae.AssetTypeSelector.Layout(window, gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: unit.Dp(-3), Right: unit.Dp(5)}.Layout(gtx, l.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return sae.Edit.Layout(gtx)
		}),
	)
}
