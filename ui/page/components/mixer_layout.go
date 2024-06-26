package components

import (
	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type MixerComponent struct {
	*load.Load

	WalletName,
	UnmixedBalance string

	Width,
	Height int

	ForwardButton,
	InfoButton cryptomaterial.IconButton
}

func (mc MixerComponent) MixerLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       mc.Width,
		Height:      mc.Height,
		Orientation: layout.Vertical,
		Padding:     layout.UniformInset(values.MarginPadding15),
		Background:  mc.Theme.Color.Surface,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Rigid(mc.topMixerLayout),
		layout.Rigid(mc.middleMixerLayout),
		layout.Rigid(mc.bottomMixerLayout),
	)
}

func (mc MixerComponent) topMixerLayout(gtx C) D {
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(mc.Theme.Icons.Mixer.Layout24dp),
		layout.Rigid(func(gtx C) D {
			lbl := mc.Theme.Body1(values.String(values.StrMixerRunning))
			lbl.Font.Weight = font.SemiBold
			return layout.Inset{
				Left:  values.MarginPadding8,
				Right: values.MarginPadding8,
			}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(mc.InfoButton.Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, mc.ForwardButton.Layout)
		}),
	)
}

func (mc MixerComponent) middleMixerLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Padding: layout.Inset{
			Left:   values.MarginPadding10,
			Right:  values.MarginPadding10,
			Top:    values.MarginPadding4,
			Bottom: values.MarginPadding4,
		},
		Margin: layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		},
		Background: mc.Theme.Color.LightBlue7,
		Alignment:  layout.Middle,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(7),
		},
	}.Layout(gtx,
		layout.Rigid(mc.Theme.Icons.AlertIcon.Layout20dp),
		layout.Rigid(func(gtx C) D {
			lbl := mc.Theme.Body2(values.String(values.StrKeepAppOpen))
			return layout.Inset{Left: values.MarginPadding6}.Layout(gtx, lbl.Layout)
		}),
	)
}

func (mc MixerComponent) bottomMixerLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Padding:     layout.UniformInset(values.MarginPadding15),
		Background:  mc.Theme.Color.Gray4,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := mc.Theme.Body2(mc.WalletName)
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(mc.Theme.Body1(values.String(values.StrUnmixedBalance)).Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return LayoutBalanceWithUnitSizeBoldText(gtx, mc.Load, mc.UnmixedBalance, values.TextSize14)
					})
				}),
			)
		}),
	)
}
