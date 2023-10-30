package components

import (
	"fmt"
	"math"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
)

// ScrollFunc is a query function that accepts offset and pagesize parameters and
// returns data interface, count of the items in the data interface, isReset and an error.
// isReset is used to reset the offset value.
type ScrollFunc func(offset, pageSize int32) (data interface{}, count int, isReset bool, err error)

type scrollDerection int

const (
	down scrollDerection = 1
	up   scrollDerection = 2
)

type Scroll struct {
	load      *load.Load
	list      *widget.List
	listStyle *cryptomaterial.ListStyle

	prevListOffset int
	prevScrollView int

	pageSize   int32
	offset     int32
	itemsCount int
	queryFunc  ScrollFunc
	data       interface{}

	// scrollView defines the scroll view length in pixels.
	scrollView int
	direction  scrollDerection

	isLoadingItems bool
	loadedAllItems bool

	mu sync.RWMutex
}

// NewScroll returns a new scroll items component.
func NewScroll(load *load.Load, pageSize int32, queryFunc ScrollFunc) *Scroll {
	return &Scroll{
		list: &widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		load:           load,
		pageSize:       pageSize,
		queryFunc:      queryFunc,
		itemsCount:     -1,
		prevListOffset: -1,
		direction:      down,
	}
}

// FetchScrollData is a mutex protected fetchScrollData function. At the end of
// the function call a window reload is triggered. Returns that latest records.
func (s *Scroll) FetchScrollData(isScrollUp bool, window app.WindowNavigator) {
	s.mu.Lock()
	// s.data is not nil when moving from details page to list page.
	if s.data != nil {
		s.isLoadingItems = false
		// offset will be added back so that the earlier page is recreated.
		s.offset -= s.pageSize
	}
	s.mu.Unlock()

	s.fetchScrollData(isScrollUp, window)
}

// fetchScrollData fetchs the scroll data and manages data returned depending on
// on the up or downwards scrollbar movement. Unless the data fetched is less than
// the page, all the old data is replaced by the new fetched data making it
// easier and smoother to scroll on the UI. At the end of the function call
// a window reload is triggered.
func (s *Scroll) fetchScrollData(isScrollUp bool, window app.WindowNavigator) {
	temDirection := down
	if isScrollUp {
		s.direction = up
	}
	if s.loadedAllItems && temDirection != s.direction {
		s.loadedAllItems = false
	}

	s.direction = temDirection
	s.mu.Lock()
	fmt.Println("-fetchScrollData---isScrollUp->", isScrollUp)
	if s.isLoadingItems || s.loadedAllItems || s.queryFunc == nil {
		fmt.Println("-fetchScrollData------000000>")
		s.mu.Unlock()
		return
	}

	if isScrollUp {
		s.list.Position.Offset = s.scrollView*-1 + 1
		s.list.Position.OffsetLast = 1
		s.offset -= s.pageSize
		fmt.Println("-scroll---Up--->", s.offset)
	} else {
		s.list.Position.Offset = 1
		s.list.Position.OffsetLast = s.scrollView*-1 + 1
		if s.data != nil {
			s.offset += s.pageSize
		}
		fmt.Println("-scroll---down--->", s.offset)
	}

	fmt.Println("-fetchScrollData------11111>")

	s.isLoadingItems = true
	// s.itemsCount = -1 // should trigger loading icon
	offset := s.offset
	tempSize := s.pageSize

	s.mu.Unlock()

	items, itemsLen, isReset, err := s.queryFunc(offset, tempSize)
	// Check if enough list items exists to fill the next page. If they do only query
	// enough items to fit the current page otherwise return all the queried items.
	// if itemsLen > int(tempSize) && itemsLen%int(tempSize) == 0 {
	// 	items, itemsLen, isReset, err = s.queryFunc(offset, tempSize)
	// }
	fmt.Println("-fetchScrollData------22222>")

	s.mu.Lock()

	if err != nil {
		errModal := modal.NewErrorModal(s.load, err.Error(), modal.DefaultClickFunc())
		window.ShowModal(errModal)
		s.isLoadingItems = false
		s.mu.Unlock()
		return
	}

	if itemsLen < int(tempSize) || itemsLen == 0 {
		// Since this is the last page set of items, prevent further scroll down queries.
		s.loadedAllItems = true
	}

	if itemsLen > 0 {
		s.data = items
		s.itemsCount = itemsLen
	}
	s.isLoadingItems = false
	s.mu.Unlock()

	fmt.Println("-fetchScrollData------33333>")

	if isReset {
		// resets the values for use on the next iteration.
		s.resetList()
	}
}

// FetchedData returns the latest queried data.
func (s *Scroll) FetchedData() interface{} {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.data
}

// ItemsCount returns the count of the last fetched items.
func (s *Scroll) ItemsCount() int {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.itemsCount
}

// List returns the list theme already in existence or newly created.
// Multiple list instances shouldn't exist for a given scroll component.
func (s *Scroll) List() *cryptomaterial.ListStyle {
	defer s.mu.RUnlock()
	s.mu.RLock()
	if s.listStyle == nil {
		// If no existing instance exists, create it.
		style := s.load.Theme.List(s.list)
		s.listStyle = &style
	}
	return s.listStyle
}

func (s *Scroll) resetList() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offset = 0
	s.loadedAllItems = false
}

// OnScrollChangeListener listens for the scroll bar movement and update the items
// list view accordingly. FetchScrollData needs to be invoked first before calling
// this function.
func (s *Scroll) OnScrollChangeListener(window app.WindowNavigator) {
	s.mu.Lock()

	// Ignore if the list component on the UI hasn't been drawn.
	if s.listStyle == nil {
		s.mu.Unlock()
		return
	}

	scrollPos := s.list.Position
	// Ignore if the query hasn't been invoked to fetch list items.
	if s.itemsCount < int(s.pageSize) && s.itemsCount != -1 {
		s.mu.Unlock()
		return
	}

	// Ignore if the query is in theprocess of fetching the list items.
	if s.itemsCount == -1 && s.isLoadingItems {
		s.mu.Unlock()
		return
	}

	s.scrollView = int(math.Abs(float64(scrollPos.Offset)) + math.Abs(float64(scrollPos.OffsetLast)))

	// Ignore the first time OnScrollChangeListener is called. When loading the
	// page a prexisting list of items to display already exist therefore no need
	// to load it again.
	if s.prevListOffset == -1 {
		s.prevListOffset = scrollPos.Offset
		s.prevScrollView = s.scrollView
		s.mu.Unlock()
		return
	}

	// Ignore duplicate events triggered without moving the scrollbar by checking
	// if the current list offset matches the previously set offset.
	if scrollPos.Offset == s.prevListOffset {
		s.mu.Unlock()
		return
	}

	// isScrollingDown checks when to fetch the Next page items because the scrollbar
	// has reached at the end of the current list loaded.
	isScrollingDown := scrollPos.Offset == s.scrollView && scrollPos.OffsetLast == 0 && s.prevListOffset < scrollPos.Offset && s.prevScrollView == s.scrollView
	// isScrollingUp checks when to fetch the Previous page items because the scrollbar
	// has reached at the beginning of the current list loaded.
	isScrollingUp := scrollPos.Offset == 0 && scrollPos.OffsetLast < 0 && s.offset > 0 && s.prevListOffset > scrollPos.Offset
	s.prevListOffset = scrollPos.Offset
	s.prevScrollView = s.scrollView

	if isScrollingDown {
		// Enforce the first item to be at the list top.
		s.list.ScrollToEnd = false
		s.list.Position.BeforeEnd = false

		s.mu.Unlock()

		go s.fetchScrollData(false, window)
	}

	if isScrollingUp {
		// Enforce the first item to be at the list bottom.
		s.list.ScrollToEnd = true
		s.list.Position.BeforeEnd = true

		s.mu.Unlock()

		go s.fetchScrollData(true, window)
	}

	if !isScrollingUp && !isScrollingDown {
		s.mu.Unlock()
	}
}
