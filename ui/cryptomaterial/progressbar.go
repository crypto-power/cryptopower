// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/ui/values"
)

type ProgressBarStyle struct {
	Radius    CornerRadius
	Height    unit.Dp
	Width     unit.Dp
	Direction layout.Direction
	material.ProgressBarStyle
}

type ProgressCircleStyle struct {
	material.ProgressCircleStyle
}

type ProgressBarItem struct {
	Value float64
	Color color.NRGBA
	Label Label
}

// MultiLayerProgressBar shows the percentage of the mutiple progress layer
// against the total/expected progress.
type MultiLayerProgressBar struct {
	t *Theme

	items                []ProgressBarItem
	Radius               CornerRadius
	Height               unit.Dp
	Width                unit.Dp
	total                float64
	ShowOverLayValue     bool
	ShowOtherWidgetFirst bool
}

func (t *Theme) ProgressBar(progress int) ProgressBarStyle {
	return ProgressBarStyle{ProgressBarStyle: material.ProgressBar(t.Base, float32(progress)/100)}
}

func (t *Theme) MultiLayerProgressBar(total float64, items []ProgressBarItem) *MultiLayerProgressBar {
	mp := &MultiLayerProgressBar{
		t: t,

		total:  total,
		Height: values.MarginPadding8,
		items:  items,
	}

	return mp
}

// This achieves a progress bar using linear layouts.
func (p ProgressBarStyle) Layout2(gtx C) D {
	if p.Width <= unit.Dp(0) {
		p.Width = unit.Dp(gtx.Constraints.Max.X)
	}

	return p.Direction.Layout(gtx, func(gtx C) D {
		return LinearLayout{
			Width:      gtx.Dp(p.Width),
			Height:     gtx.Dp(p.Height),
			Background: p.TrackColor,
			Border:     Border{Radius: p.Radius},
		}.Layout2(gtx, func(gtx C) D {
			return LinearLayout{
				Width:      int(float32(p.Width) * clamp1(p.Progress)),
				Height:     gtx.Dp(p.Height),
				Background: p.Color,
				Border:     Border{Radius: p.Radius},
			}.Layout(gtx)
		})
	})
}

// This achieves a progress bar using linear layouts.
func (p ProgressBarStyle) TextLayout(gtx C, lbl layout.Widget) D {
	if p.Width <= unit.Dp(0) {
		p.Width = unit.Dp(gtx.Constraints.Max.X)
	}

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			return p.Direction.Layout(gtx, func(gtx C) D {
				return LinearLayout{
					Width:      MatchParent,
					Height:     gtx.Dp(p.Height),
					Background: p.TrackColor,
					Border:     Border{Radius: p.Radius},
				}.Layout2(gtx, func(gtx C) D {
					return LinearLayout{
						Width:      int(float32(p.Width) * clamp1(p.Progress)),
						Height:     gtx.Dp(p.Height),
						Background: p.Color,
						Border:     Border{Radius: p.Radius},
						Direction:  layout.Center,
					}.Layout(gtx)
				})
			})
		}),
		layout.Expanded(func(gtx C) D {
			return layout.Center.Layout(gtx, lbl)
		}),
	)
}

func (p ProgressBarStyle) Layout(gtx layout.Context) layout.Dimensions {
	shader := func(width int, color color.NRGBA) layout.Dimensions {
		maxHeight := p.Height
		if p.Height <= 0 {
			maxHeight = unit.Dp(4)
		}

		d := image.Point{X: width, Y: gtx.Dp(maxHeight)}
		height := gtx.Dp(maxHeight)

		tr := gtx.Dp(unit.Dp(p.Radius.TopRight))
		tl := gtx.Dp(unit.Dp(p.Radius.TopLeft))
		br := gtx.Dp(unit.Dp(p.Radius.BottomRight))
		bl := gtx.Dp(unit.Dp(p.Radius.BottomLeft))

		defer clip.RRect{
			Rect: image.Rectangle{Max: image.Pt(width, height)},
			NW:   tl, NE: tr, SE: br, SW: bl,
		}.Push(gtx.Ops).Pop()

		paint.ColorOp{Color: color}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)

		return layout.Dimensions{Size: d}
	}

	if p.Width <= 0 {
		p.Width = unit.Dp(gtx.Constraints.Max.X)
	}

	progressBarWidth := int(p.Width)
	return layout.Stack{Alignment: layout.W}.Layout(gtx,
		layout.Stacked(func(_ layout.Context) layout.Dimensions {
			return shader(progressBarWidth, p.TrackColor)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			fillWidth := int(float32(progressBarWidth) * clamp1(p.Progress))
			fillColor := p.Color
			if gtx.Queue == nil {
				fillColor = Disabled(fillColor)
			}
			return shader(fillWidth, fillColor)
		}),
	)
}

// TODO: Allow more than just 2 layers and make it dynamic
func (mp *MultiLayerProgressBar) progressBarLayout(gtx C) D {
	if mp.Width <= 0 {
		mp.Width = unit.Dp(gtx.Constraints.Max.X)
	}

	pg := func(width int, lbl Label, color color.NRGBA) layout.Dimensions {
		return LinearLayout{
			Width:      width,
			Height:     gtx.Dp(mp.Height),
			Background: color,
		}.Layout2(gtx, func(gtx C) D {
			if mp.ShowOverLayValue {
				lbl.Color = mp.t.Color.Surface
				return LinearLayout{
					Width:      width,
					Height:     gtx.Dp(mp.Height),
					Background: color,
					Direction:  layout.Center,
				}.Layout2(gtx, lbl.Layout)
			}
			return D{}
		})
	}

	calProgressWidth := func(progress float64) float64 {
		if mp.total != 0 {
			val := (progress / mp.total) * 100
			return (float64(mp.Width) / 100) * val
		}

		return 0
	}

	// display empty gray layout when total value passed is zero (0)
	if mp.total == 0 {
		return pg(int(mp.Width), mp.t.Label(values.TextSize14, ""), mp.t.Color.Gray2)
	}

	// This takes only 2 layers
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(_ C) D {
			width := calProgressWidth(mp.items[0].Value)
			if width == 0 {
				return D{}
			}
			return pg(int(width), mp.items[0].Label, mp.items[0].Color)
		}),
		layout.Rigid(func(_ C) D {
			width := calProgressWidth(mp.items[1].Value)
			if width == 0 {
				return D{}
			}
			return pg(int(width), mp.items[1].Label, mp.items[1].Color)
		}),
	)
}

func (mp *MultiLayerProgressBar) Layout(gtx C, isMobileView bool, additionalWidget layout.Widget) D {
	if additionalWidget == nil {
		// We're only displaying the progress bar, no need for flex layout to wrap it.
		// TODO: Verify if a top padding is necessary if we're only displaying the progressbar.
		return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, mp.progressBarLayout)
	}

	progressBarTopPadding, otherWidget := values.MarginPadding24, additionalWidget
	if isMobileView {
		progressBarTopPadding = values.MarginPadding16
	}
	if !mp.ShowOtherWidgetFirst {
		// reduce the top padding if we're showing the progress bar before the other widget
		progressBarTopPadding = values.MarginPadding5
		otherWidget = func(gtx C) D {
			return layout.Center.Layout(gtx, additionalWidget)
		}
	}

	flexWidgets := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: progressBarTopPadding}.Layout(gtx, mp.progressBarLayout)
		}),
		layout.Rigid(otherWidget),
	}

	if mp.ShowOtherWidgetFirst {
		// Swap the label and progress bar...
		flexWidgets[0], flexWidgets[1] = flexWidgets[1], flexWidgets[0]
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, flexWidgets...)
}

func (t *Theme) ProgressBarCircle(progress int) ProgressCircleStyle {
	return ProgressCircleStyle{ProgressCircleStyle: material.ProgressCircle(t.Base, float32(progress)/100)}
}

func (p ProgressCircleStyle) Layout(gtx layout.Context) layout.Dimensions {
	return p.ProgressCircleStyle.Layout(gtx)
}

// clamp1 limits mp.to range [0..1].
func clamp1(v float32) float32 {
	if v >= 1 {
		return 1
	} else if v <= 0 {
		return 0
	}
	return v
}
