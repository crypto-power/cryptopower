// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/crypto-power/cryptopower/ui/values"
)

type SliderItem struct {
	Title    string
	MainText string
	SubText  string

	Image           *Image
	BackgroundImage *Image
}

type Slider struct {
	t          *Theme
	nextButton *Clickable
	prevButton *Clickable
	card       Card

	items []SliderItem

	backgroundImageHeight unit.Dp
	backgroundImageWidth  unit.Dp

	selected int
}

func (t *Theme) Slider(i []SliderItem, width, height unit.Dp) *Slider {
	sl := &Slider{
		t:                     t,
		items:                 i,
		nextButton:            t.NewClickable(false),
		prevButton:            t.NewClickable(false),
		backgroundImageHeight: height,
		backgroundImageWidth:  width,
	}

	sl.card = sl.t.Card()
	sl.card.Radius = Radius(8)

	return sl
}

func (s *Slider) Layout(gtx C) D {
	s.handleClickEvent()
	m8 := values.MarginPadding8
	m4 := values.MarginPadding4

	return s.containerLayout(gtx, func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx C) D {
				return s.items[s.selected].BackgroundImage.LayoutSize2(gtx, s.backgroundImageWidth, s.backgroundImageHeight)
			}),
			layout.Expanded(func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Vertical,
					Alignment: layout.Middle,
					Spacing:   layout.SpaceEvenly,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						lbl := s.t.Body1(s.items[s.selected].Title)
						lbl.Color = s.t.Color.InvText
						return s.centerLayout(gtx, lbl.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						return s.centerLayout(gtx, func(gtx C) D {
							return s.items[s.selected].Image.LayoutSize(gtx, values.MarginPadding65)
						})
					}),
					layout.Rigid(func(gtx C) D {
						lbl := s.t.Body1(s.items[s.selected].MainText)
						lbl.Color = s.t.Color.InvText
						return s.centerLayout(gtx, lbl.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						s.card.Radius = Radius(12)
						s.card.Color = values.TransparentColor(values.TransparentBlack, 0.2)
						return s.centerLayout(gtx, func(gtx C) D {
							return s.containerLayout(gtx, func(gtx C) D {
								return layout.Inset{
									Top:    m4,
									Bottom: m4,
									Right:  m8,
									Left:   m8,
								}.Layout(gtx, func(gtx C) D {
									lbl := s.t.Body2(s.items[s.selected].SubText)
									lbl.Color = s.t.Color.InvText
									return lbl.Layout(gtx)
								})
							})
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{
							Axis:    layout.Horizontal,
							Spacing: layout.SpaceAround,
						}.Layout(gtx,
							layout.Rigid(s.selectedItemIndicatorLayout),
							layout.Rigid(s.t.Body2("         ").Layout), // TODO dummy space for setting the position of the bottom buttons
							layout.Rigid(s.buttonLayout),
						)
					}),
				)
			}),
		)
	})
}

func (s *Slider) buttonLayout(gtx C) D {
	m4 := values.MarginPadding4
	s.card.Radius = Radius(10)
	s.card.Color = values.TransparentColor(values.TransparentWhite, 0.2)
	return s.containerLayout(gtx, func(gtx C) D {
		return layout.Inset{
			Right: m4,
			Left:  m4,
		}.Layout(gtx, func(gtx C) D {
			return LinearLayout{
				Width:       WrapContent,
				Height:      WrapContent,
				Orientation: layout.Horizontal,
				Direction:   layout.Center,
				Alignment:   layout.Middle,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return s.prevButton.Layout(gtx, s.t.Icons.ChevronLeft.Layout20dp)
				}),
				layout.Rigid(func(gtx C) D {
					return s.nextButton.Layout(gtx, s.t.Icons.ChevronRight.Layout20dp)
				}),
			)
		})
	})
}

func (s *Slider) selectedItemIndicatorLayout(gtx C) D {
	m4 := values.MarginPadding4
	s.card.Radius = Radius(10)
	s.card.Color = values.TransparentColor(values.TransparentWhite, 0.2)
	return s.containerLayout(gtx, func(gtx C) D {
		return layout.Inset{
			Right: m4,
			Left:  m4,
		}.Layout(gtx, func(gtx C) D {
			list := &layout.List{Axis: layout.Horizontal}
			return list.Layout(gtx, len(s.items), func(gtx C, i int) D {
				ic := NewIcon(s.t.Icons.ImageBrightness1)
				ic.Color = values.TransparentColor(values.TransparentBlack, 0.2)
				if i == s.selected {
					ic.Color = s.t.Color.Surface
				}
				return layout.Inset{
					Top:    m4,
					Bottom: m4,
					Right:  m4,
					Left:   m4,
				}.Layout(gtx, func(gtx C) D {
					return ic.Layout(gtx, values.MarginPadding12)
				})
			})
		})
	})
}

func (s *Slider) containerLayout(gtx C, content layout.Widget) D {
	return s.card.Layout(gtx, content)
}

func (s *Slider) centerLayout(gtx C, content layout.Widget) D {
	return layout.Center.Layout(gtx, content)
}

func (s *Slider) handleClickEvent() {
	l := len(s.items) - 1
	if s.nextButton.Clicked() {
		if s.selected == l {
			s.selected = 0
		} else {
			s.selected += 1
		}
	}

	if s.prevButton.Clicked() {
		if s.selected == 0 {
			s.selected = l
		} else {
			s.selected -= 1
		}
	}
}
