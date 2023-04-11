package cryptomaterial

import (
	"image"
	"image/color"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	WrapContent = -1
	MatchParent = -2
)

type LinearLayout struct {
	Width       int
	Height      int
	Orientation layout.Axis
	Background  color.NRGBA
	Shadow      *Shadow
	Border      Border
	Margin      layout.Inset
	Padding     layout.Inset
	Direction   layout.Direction
	Spacing     layout.Spacing
	Alignment   layout.Alignment
	Clickable   *Clickable
}

type colorScheme struct {
	R uint8   // Red Subpixel
	G uint8   // Green Subpixel
	B uint8   // Blue Subpixel
	O float64 // Opacity; value range 0-1
}

type gradientcolorScheme struct {
	color1 colorScheme
	color2 colorScheme
	blend1 float64 // percent of position along X axis where color1 blend ends.
	blend2 float64 // percent of position along X axis where color2 blend ends.
}

// nrgbaColor converts figma color scheme to gioui nrgba color scheme.
func (c *colorScheme) nrgbaColor() color.NRGBA {
	transparency := 127.0 - (127.0 * c.O) // opacity = (127 - transparency) / 127
	return color.NRGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: uint8(transparency),
	}
}

var assetsGradientColorSchemes = map[utils.AssetType]gradientcolorScheme{
	utils.BTCWalletAsset: {
		color1: colorScheme{R: 196, G: 203, B: 210, O: 0.3}, // rgba(196, 203, 210, 0.3)
		blend1: 34.76,                                       // 34.76%
		color2: colorScheme{R: 248, G: 152, B: 36, O: 0.3},  // rgba(248, 152, 36, 0.3)
		blend2: 65.88,                                       // 65.88 %
	},
	utils.DCRWalletAsset: {
		color1: colorScheme{R: 41, G: 112, B: 255, O: 0.3}, // rgba(41, 112, 255, 0.3)
		blend1: 34.76,                                      // 34.76%
		color2: colorScheme{R: 45, G: 216, B: 163, O: 0.3}, // rgba(45, 216, 163, 0.3)
		blend2: 65.88,                                      // 65.88 %
	},
	utils.LTCWalletAsset: {
		color1: colorScheme{R: 224, G: 224, B: 224, O: 0.3}, // rgba(224, 224, 224, 0.3)
		blend1: 34.76,                                       // 34.76%
		color2: colorScheme{R: 56, G: 115, B: 223, O: 0.3},  // rgba(56, 115, 223, 0.3)
		blend2: 65.88,                                       // 65.88 %
	},
}

// Layout2 displays a linear layout with a single child.
func (ll LinearLayout) Layout2(gtx C, wdg layout.Widget) D {
	return ll.Layout(gtx, layout.Rigid(wdg))
}

func (ll LinearLayout) Layout(gtx C, children ...layout.FlexChild) D {
	// draw layout direction
	dims := ll.Direction.Layout(gtx, func(gtx C) D {
		// draw margin
		return ll.Margin.Layout(gtx, func(gtx C) D {
			wdg := func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						ll.applyDimension(&gtx)
						// draw background and clip the background to border radius
						tr := gtx.Dp(unit.Dp(ll.Border.Radius.TopRight))
						tl := gtx.Dp(unit.Dp(ll.Border.Radius.TopLeft))
						br := gtx.Dp(unit.Dp(ll.Border.Radius.BottomRight))
						bl := gtx.Dp(unit.Dp(ll.Border.Radius.BottomLeft))
						defer clip.RRect{
							Rect: image.Rectangle{Max: image.Point{
								X: gtx.Constraints.Min.X,
								Y: gtx.Constraints.Min.Y,
							}},
							NW: tl, NE: tr, SE: br, SW: bl,
						}.Push(gtx.Ops).Pop()

						background := ll.Background
						if ll.Clickable == nil {
							return fill(gtx, background)
						}

						if ll.Clickable.Hoverable && ll.Clickable.button.Hovered() {
							background = ll.Clickable.style.HoverColor
						}
						fill(gtx, background)

						for _, c := range ll.Clickable.button.History() {
							drawInk(gtx, c, ll.Clickable.style.Color)
						}

						return ll.Clickable.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							semantic.Button.Add(gtx.Ops)
							return layout.Dimensions{Size: gtx.Constraints.Min}
						})
					}),
					layout.Stacked(func(gtx C) D {
						ll.applyDimension(&gtx)
						return ll.Border.Layout(gtx, func(gtx C) D {
							// draw padding
							return ll.Padding.Layout(gtx, func(gtx C) D {
								// draw layout direction
								return ll.Direction.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: ll.Orientation, Alignment: ll.Alignment, Spacing: ll.Spacing}.Layout(gtx, children...)
								})
							})
						})
					}),
				)
			}

			if ll.Shadow != nil {
				if ll.Clickable != nil && ll.Clickable.Hoverable {
					if ll.Clickable.button.Hovered() {
						return ll.Shadow.Layout(gtx, wdg)
					}
					return wdg(gtx)
				}
				return ll.Shadow.Layout(gtx, wdg)
			}
			return wdg(gtx)
		})
	})

	if ll.Width > 0 {
		dims.Size.X = ll.Width
	}
	return dims
}

func (ll LinearLayout) GradientLayout(gtx C, assetType utils.AssetType, children ...layout.FlexChild) D {
	// draw layout direction
	return ll.Direction.Layout(gtx, func(gtx C) D {
		// draw margin
		return ll.Margin.Layout(gtx, func(gtx C) D {
			wdg := func(gtx C) D {
				cScheme, ok := assetsGradientColorSchemes[assetType]
				if !ok {
					// Incase of an unaccounted asset empty component.
					return D{}
				}

				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						ll.applyDimension(&gtx)
						// draw background and clip the background to border radius

						tr := gtx.Dp(unit.Dp(ll.Border.Radius.TopRight))
						tl := gtx.Dp(unit.Dp(ll.Border.Radius.TopLeft))
						br := gtx.Dp(unit.Dp(ll.Border.Radius.BottomRight))
						bl := gtx.Dp(unit.Dp(ll.Border.Radius.BottomLeft))

						dr := image.Rectangle{Max: image.Point{
							X: gtx.Constraints.Min.X,
							Y: gtx.Constraints.Min.Y,
						}}

						stop1 := layout.FPt(dr.Max)
						stop1.X *= float32(cScheme.blend1) / 100

						stop2 := layout.FPt(dr.Max)
						stop2.X *= float32(cScheme.blend2) / 100

						paint.LinearGradientOp{
							Stop1:  stop1,
							Stop2:  stop2,
							Color1: color.NRGBAModel.Convert(cScheme.color1.nrgbaColor()).(color.NRGBA),
							Color2: color.NRGBAModel.Convert(cScheme.color2.nrgbaColor()).(color.NRGBA),
						}.Add(gtx.Ops)

						defer clip.RRect{
							Rect: dr,
							NW:   tl, NE: tr, SE: br, SW: bl,
						}.Push(gtx.Ops).Pop()
						paint.PaintOp{}.Add(gtx.Ops)

						if ll.Clickable == nil {
							return layout.Dimensions{
								Size: gtx.Constraints.Min,
							}
						}

						if ll.Clickable.Hoverable && ll.Clickable.button.Hovered() {
							fill(gtx, ll.Background)
						}

						for _, c := range ll.Clickable.button.History() {
							drawInk(gtx, c, ll.Clickable.style.Color)
						}

						return ll.Clickable.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							semantic.Button.Add(gtx.Ops)

							return layout.Dimensions{
								Size: gtx.Constraints.Min,
							}
						})
					}),
					layout.Stacked(func(gtx C) D {
						ll.applyDimension(&gtx)
						return ll.Border.Layout(gtx, func(gtx C) D {
							// draw padding
							return ll.Padding.Layout(gtx, func(gtx C) D {
								// draw layout direction
								return ll.Direction.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: ll.Orientation, Alignment: ll.Alignment, Spacing: ll.Spacing}.Layout(gtx, children...)
								})
							})
						})
					}),
				)
			}

			if ll.Shadow != nil {
				if ll.Clickable != nil && ll.Clickable.Hoverable {
					if ll.Clickable.button.Hovered() {
						return ll.Shadow.Layout(gtx, wdg)
					}
					return wdg(gtx)
				}
				return ll.Shadow.Layout(gtx, wdg)
			}
			return wdg(gtx)
		})
	})
}

func (ll LinearLayout) applyDimension(gtx *C) {
	if ll.Width == MatchParent {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	} else if ll.Width != WrapContent {
		gtx.Constraints.Min.X = ll.Width
		gtx.Constraints.Max.X = ll.Width
	}

	if ll.Height == MatchParent {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	} else if ll.Height != WrapContent {
		gtx.Constraints.Min.Y = ll.Height
		gtx.Constraints.Max.Y = ll.Height
	}
}
