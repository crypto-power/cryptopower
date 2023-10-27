package cryptomaterial

import (
	"sync"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/values"
)

type SegmentedControl struct {
	theme *Theme
	list  *ClickableList

	selectedIndex int
	segmentTitles []string

	changed bool
	mu      sync.Mutex
}

func (t *Theme) SegmentedControl(segmentTitles []string) *SegmentedControl {
	list := t.NewClickableList(layout.Horizontal)
	list.IsHoverable = false

	return &SegmentedControl{
		list:          list,
		theme:         t,
		segmentTitles: segmentTitles,
	}
}

func (sc *SegmentedControl) Layout(gtx C) D {
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

func (sc *SegmentedControl) TransparentLayout(gtx C) D {
	sc.handleEvents()

	return LinearLayout{
		Width:  WrapContent,
		Height: WrapContent,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
				isSelectedSegment := sc.SelectedIndex() == i
				return layout.Center.Layout(gtx, func(gtx C) D {
					bg := sc.theme.Color.Gray2
					txt := sc.theme.DecoratedText(values.TextSize16, sc.segmentTitles[i], sc.theme.Color.GrayText2, font.SemiBold)
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
