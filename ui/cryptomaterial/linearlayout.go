package cryptomaterial

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/crypto-power/cryptopower/libwallet/utils"
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

						if ll.Clickable.Hoverable && ll.Clickable.IsHovered() {
							background = ll.Clickable.style.HoverColor
						}
						fill(gtx, background)

						for _, c := range ll.Clickable.button.History() {
							drawInk(gtx, c, ll.Clickable.style.Color)
						}

						return ll.Clickable.button.Layout(gtx, func(gtx C) D {
							semantic.Button.Add(gtx.Ops)
							return D{Size: gtx.Constraints.Min}
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
				cScheme, ok := utils.GradientColorSchemes()[assetType]
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
						stop1.X *= float32(cScheme.Blend1) / 100

						stop2 := layout.FPt(dr.Max)
						stop2.X *= float32(cScheme.Blend2) / 100

						paint.LinearGradientOp{
							Stop1:  stop1,
							Stop2:  stop2,
							Color1: color.NRGBAModel.Convert(cScheme.Color1.NRGBAColor()).(color.NRGBA),
							Color2: color.NRGBAModel.Convert(cScheme.Color2.NRGBAColor()).(color.NRGBA),
						}.Add(gtx.Ops)

						defer clip.RRect{
							Rect: dr,
							NW:   tl, NE: tr, SE: br, SW: bl,
						}.Push(gtx.Ops).Pop()
						paint.PaintOp{}.Add(gtx.Ops)

						if ll.Clickable == nil {
							return D{
								Size: gtx.Constraints.Min,
							}
						}

						if ll.Clickable.Hoverable && ll.Clickable.button.Hovered() {
							fill(gtx, ll.Background)
						}

						for _, c := range ll.Clickable.button.History() {
							drawInk(gtx, c, ll.Clickable.style.Color)
						}

						return ll.Clickable.button.Layout(gtx, func(gtx C) D {
							semantic.Button.Add(gtx.Ops)

							return D{
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
