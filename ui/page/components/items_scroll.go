package components

import (
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
type ScrollFunc[T any] func(offset, pageSize int32) (data []T, count int, isReset bool, err error)

type scrollDerection int

const (
	down scrollDerection = 1
	up   scrollDerection = 2
)

type Scroll[T any] struct {
	load      *load.Load
	list      *widget.List
	listStyle *cryptomaterial.ListStyle

	prevListOffset int
	prevScrollView int

	pageSize   int32
	offset     int32
	itemsCount int
	queryFunc  ScrollFunc[T]
	data       []T

	// scrollView defines the scroll view length in pixels.
	scrollView int
	direction  scrollDerection

	isLoadingItems bool
	loadedAllItems bool

	isHaveKeySearch bool

	mu sync.RWMutex
}

// NewScroll returns a new scroll items component.
func NewScroll[T any](load *load.Load, pageSize int32, queryFunc ScrollFunc[T]) *Scroll[T] {
	return &Scroll[T]{
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

func (s *Scroll[T]) SetIsHaveKeySearch(isHaveKeySearch bool) {
	s.isHaveKeySearch = isHaveKeySearch
}

// FetchScrollData is a mutex protected fetchScrollData function. At the end of
// the function call a window reload is triggered. Returns that latest records.
func (s *Scroll[T]) FetchScrollData(isScrollUp bool, window app.WindowNavigator, isResetList bool) {
	s.mu.Lock()
	// s.data is not nil when moving from details page to list page.
	if s.data != nil {
		s.isLoadingItems = false
		// offset will be added back so that the earlier page is recreated.
		s.offset -= s.pageSize
	}
	if isResetList {
		s.loadedAllItems = false
		s.isLoadingItems = false
		s.offset = 0
		s.itemsCount = 0
		s.data = nil
	}
	s.mu.Unlock()

	s.fetchScrollData(isScrollUp, window)
}

// fetchScrollData fetchs the scroll data and manages data returned depending on
// on the up or downwards scrollbar movement. Unless the data fetched is less than
// the page, all the old data is replaced by the new fetched data making it
// easier and smoother to scroll on the UI. At the end of the function call
// a window reload is triggered.
func (s *Scroll[T]) fetchScrollData(isScrollUp bool, window app.WindowNavigator) {
	temDirection := down
	if isScrollUp {
		s.direction = up
	}
	if s.loadedAllItems && temDirection != s.direction {
		s.loadedAllItems = false
	}

	s.direction = temDirection
	s.mu.Lock()
	if s.isLoadingItems || s.loadedAllItems || s.queryFunc == nil {
		s.mu.Unlock()
		return
	}

	if isScrollUp {
		s.list.Position.Offset = s.scrollView*-1 + 1
		s.list.Position.OffsetLast = 1
		s.offset -= s.pageSize
	} else {
		s.list.Position.Offset = 1
		s.list.Position.OffsetLast = s.scrollView*-1 + 1
		if s.data != nil {
			s.offset += s.pageSize
		}
	}

	s.isLoadingItems = true
	itemsCountTemp := s.itemsCount
	if s.itemsCount == -1 {
		itemsCountTemp = 0
	}
	s.itemsCount = -1 // should trigger loading icon
	offset := s.offset
	tempSize := s.pageSize

	s.mu.Unlock()

	items, itemsLen, isReset, err := s.queryFunc(offset, tempSize)

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
	} else {
		s.itemsCount = itemsCountTemp
	}
	s.isLoadingItems = false
	s.mu.Unlock()

	if isReset {
		// resets the values for use on the next iteration.
		s.resetList()
	}
}

// FetchedData returns the latest queried data.
func (s *Scroll[T]) FetchedData() []T {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.data
}

// ItemsCount returns the count of the last fetched items.
func (s *Scroll[T]) ItemsCount() int {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.itemsCount
}

// List returns the list theme already in existence or newly created.
// Multiple list instances shouldn't exist for a given scroll component.
func (s *Scroll[T]) List() *cryptomaterial.ListStyle {
	defer s.mu.RUnlock()
	s.mu.RLock()
	if s.listStyle == nil {
		// If no existing instance exists, create it.
		style := s.load.Theme.List(s.list)
		s.listStyle = &style
	}
	return s.listStyle
}

func (s *Scroll[T]) resetList() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offset = 0
	s.loadedAllItems = false
}

// OnScrollChangeListener listens for the scroll bar movement and update the items
// list view accordingly. FetchScrollData needs to be invoked first before calling
// this function.
func (s *Scroll[T]) OnScrollChangeListener(window app.WindowNavigator) {
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
