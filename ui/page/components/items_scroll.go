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

const maxListSize = 150

// ScrollFunc is a query function that accepts offset and pagesize parameters and
// returns data interface and an error.
type ScrollFunc[T any] func(offset, pageSize int32) (data []T, err error)

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

	isLoadingItems bool
	loadedAllItems bool

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
	}
}

// FetchScrollData is a mutex protected fetchScrollData function. At the end of
// the function call a window reload is triggered. Returns that latest records.
// isReset is used to reset the offset value.
func (s *Scroll[T]) FetchScrollData(isReset bool, window app.WindowNavigator) {
	s.mu.Lock()
	// s.data is not nil when moving from details page to list page.
	if s.data != nil {
		s.isLoadingItems = false
		// offset will be added back so that the earlier page is recreated.
		s.offset -= s.pageSize
	}
	s.mu.Unlock()
	// set isReverse to default false as callers of this method are not
	// perform a reverse scroll action
	s.fetchScrollData(false, isReset, window)
}

// fetchScrollData fetchs the scroll data and manages data returned depending on
// on the up or downwards scrollbar movement. Unless the data fetched is less than
// the page, all the old data is replaced by the new fetched data making it
// easier and smoother to scroll on the UI. At the end of the function call
// a window reload is triggered.
func (s *Scroll[T]) fetchScrollData(isReverse, isReset bool, window app.WindowNavigator) {
	s.mu.Lock()

	if isReset {
		// resets the values for use on the next iteration.
		s.resetList()
	}

	if s.isLoadingItems || s.loadedAllItems || s.queryFunc == nil {
		return
	}

	if isReverse {
		s.offset -= s.pageSize
	} else {
		if s.data != nil && !isReset {
			s.offset += s.pageSize
		}
	}

	s.isLoadingItems = true
	offset := s.offset
	tempSize := s.pageSize

	s.mu.Unlock()

	items, err := s.queryFunc(offset, tempSize)

	s.mu.Lock()

	if err != nil {
		errModal := modal.NewErrorModal(s.load, err.Error(), modal.DefaultClickFunc())
		window.ShowModal(errModal)
		s.isLoadingItems = false
		s.mu.Unlock()

		return
	}

	itemsLen := len(items)
	if itemsLen < int(tempSize) {
		// Since this is the last page set of items, prevent further scroll down queries.
		s.loadedAllItems = true
	}

	if isReverse {
		s.data = append(items, s.data...)
		s.data = s.data[:len(s.data)-int(s.pageSize)]
	} else {
		s.data = append(s.data, items...) // append to existing record
		if len(s.data) > maxListSize {
			s.data = s.data[int(s.pageSize):]
		}
	}

	// set default scroll position to half the page to make navigation fluid
	s.list.Position.Offset = int(float32(s.list.Position.Length) * 0.5)

	s.itemsCount = itemsLen
	s.isLoadingItems = false
	s.mu.Unlock()
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
		// s.list.ScrollToEnd = false
		// s.list.Position.BeforeEnd = false

		s.mu.Unlock()

		go s.fetchScrollData(false, false, window)
	}

	if isScrollingUp {
		// Enforce the first item to be at the list bottom.
		// s.list.ScrollToEnd = true
		// s.list.Position.BeforeEnd = true

		s.mu.Unlock()

		go s.fetchScrollData(true, false, window)
	}

	if !isScrollingUp && !isScrollingDown {
		s.mu.Unlock()
	}
}
