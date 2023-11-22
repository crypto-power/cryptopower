package cryptomaterial

import (
	"sync"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/values"
)

type SegmentType int

const (
	Group SegmentType = iota
	Split
)

type SegmentedControl struct {
	theme *Theme
	list  *ClickableList

	leftNavBtn,
	rightNavBtn *Clickable

	selectedIndex int
	segmentTitles []string

	changed bool
	mu      sync.Mutex

	isSwipeActionEnabled bool
	sliceAction          SliceAction
	segmentType          SegmentType
}

func (t *Theme) SegmentedControl(segmentTitles []string, segmentType SegmentType) *SegmentedControl {
	list := t.NewClickableList(layout.Horizontal)
	list.IsHoverable = false

	sc := &SegmentedControl{
		list:                 list,
		theme:                t,
		segmentTitles:        segmentTitles,
		leftNavBtn:           t.NewClickable(false),
		rightNavBtn:          t.NewClickable(false),
		isSwipeActionEnabled: true,
		segmentType:          segmentType,
	}

	sc.sliceAction.Draged(func(dragDirection SwipeDirection) {
		isNext := dragDirection == SwipeLeft
		sc.handleActionEvent(isNext)
	})

	return sc
}

func (sc *SegmentedControl) SetEnableSwipe(enable bool) {
	sc.isSwipeActionEnabled = enable
}

func (sc *SegmentedControl) Layout(gtx C, body func(gtx C) D) D {
	return UniformPadding(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if sc.segmentType == Group {
					return sc.GroupTileLayout(gtx)
				}
				return sc.splitTileLayout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					if sc.isSwipeActionEnabled {
						return sc.sliceAction.DragLayout(gtx, func(gtx C) D {
							return sc.sliceAction.TransformLayout(gtx, body)
						}, true)
					}
					return body(gtx)
				})
			}),
		)
	})
}

func (sc *SegmentedControl) GroupTileLayout(gtx C) D {
	sc.handleEvents()

	return LinearLayout{
		Width:      WrapContent,
		Height:     WrapContent,
		Background: sc.theme.Color.Background,
		Border:     Border{Radius: Radius(8)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
				isSelectedSegment := sc.SelectedIndex() == i
				return layout.Center.Layout(gtx, func(gtx C) D {
					bg := sc.theme.Color.SurfaceHighlight
					txt := sc.theme.DecoratedText(values.TextSize16, sc.segmentTitles[i], sc.theme.Color.GrayText1, font.SemiBold)
					border := Border{Radius: Radius(0)}
					if isSelectedSegment {
						bg = sc.theme.Color.Surface
						txt.Color = sc.theme.Color.Text
						border = Border{Radius: Radius(8)}
					}
					return LinearLayout{
						Width:      WrapContent,
						Height:     WrapContent,
						Padding:    layout.UniformInset(values.MarginPadding8),
						Background: bg,
						Margin:     layout.UniformInset(values.MarginPadding5),
						Border:     border,
					}.Layout2(gtx, txt.Layout)
				})
			})
		}),
	)
}

func (sc *SegmentedControl) splitTileLayout(gtx C) D {
	sc.handleEvents()
	return LinearLayout{
		Width:       gtx.Dp(values.MarginPadding700),
		Height:      WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
	}.Layout(gtx,
		layout.Flexed(.035, func(gtx C) D {
			return sc.leftNavBtn.Layout(gtx, sc.theme.Icons.ChevronLeft.Layout24dp)
		}),
		layout.Flexed(.8, func(gtx C) D {
			return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
				isSelectedSegment := sc.SelectedIndex() == i
				return layout.Center.Layout(gtx, func(gtx C) D {
					bg := sc.theme.Color.Gray2
					txt := sc.theme.DecoratedText(values.TextSize14, sc.segmentTitles[i], sc.theme.Color.GrayText2, font.SemiBold)
					border := Border{Radius: Radius(8)}
					if isSelectedSegment {
						bg = sc.theme.Color.LightBlue8
						txt.Color = sc.theme.Color.Primary
					}
					paddingTB := values.MarginPadding8
					paddingLR := values.MarginPadding32
					pr := values.MarginPadding6
					if i == len(sc.segmentTitles) { // no need to add padding to the last item
						pr = values.MarginPadding0
					}

					return layout.Inset{Right: pr}.Layout(gtx, func(gtx C) D {
						return LinearLayout{
							Width:  WrapContent,
							Height: WrapContent,
							Padding: layout.Inset{
								Top:    paddingTB,
								Bottom: paddingTB,
								Left:   paddingLR,
								Right:  paddingLR,
							},
							Background: bg,
							Border:     border,
						}.Layout2(gtx, txt.Layout)
					})
				})
			})
		}),
		layout.Flexed(.035, func(gtx C) D {
			return sc.rightNavBtn.Layout(gtx, sc.theme.Icons.ChevronRight.Layout24dp)
		}),
	)
}

func (sc *SegmentedControl) handleEvents() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if segmentItemClicked, clickedSegmentIndex := sc.list.ItemClicked(); segmentItemClicked {
		if sc.selectedIndex != clickedSegmentIndex {
			sc.changed = true
		}
		sc.selectedIndex = clickedSegmentIndex
	}

	if sc.leftNavBtn.Clicked() {
		sc.list.Position.First = 0
		sc.list.Position.Offset = 0
		sc.list.Position.BeforeEnd = true
		sc.list.ScrollToEnd = false
	}
	if sc.rightNavBtn.Clicked() {
		sc.list.Position.OffsetLast = 0
		sc.list.Position.BeforeEnd = false
		sc.list.ScrollToEnd = true
	}
}

func (sc *SegmentedControl) SelectedIndex() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.selectedIndex
}

func (sc *SegmentedControl) SelectedSegment() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.segmentTitles[sc.selectedIndex]
}

func (sc *SegmentedControl) Changed() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	changed := sc.changed
	sc.changed = false
	return changed
}

func (sc *SegmentedControl) SetSelectedSegment(segment string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for i, item := range sc.segmentTitles {
		if item == segment {
			sc.selectedIndex = i
			break
		}
	}
}

func (sc *SegmentedControl) handleActionEvent(isNext bool) {
	l := len(sc.segmentTitles) - 1 // index starts at 0
	if isNext {
		if sc.selectedIndex == l {
			sc.selectedIndex = 0
		} else {
			sc.selectedIndex++
		}
		sc.sliceAction.PushLeft()
	} else {
		if sc.selectedIndex == 0 {
			sc.selectedIndex = l
		} else {
			sc.selectedIndex--
		}
		sc.sliceAction.PushRight()
	}
	sc.changed = true
}
