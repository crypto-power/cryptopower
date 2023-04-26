package components

import (
	"math"
	"sync"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"gioui.org/layout"
	"gioui.org/widget"
)

// ScrollFunc is a query function that accepts offset and pagesize parameters and
// returns data interface, count of the items in the data interface, isReset and an error.
// isReset is used to reset the offset value.
type ScrollFunc func(offset, pageSize int32) (data interface{}, count int, isReset bool, err error)

type Scroll struct {
	load           *load.Load
	list           *widget.List
	listStyle      *cryptomaterial.ListStyle
	prevListOffset int

	pageSize   int32
	offset     int32
	itemsCount int
	queryFunc  ScrollFunc
	data       interface{}

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
	}
}

// FetchScrollData is a mutex protected fetchScrollData function. At the end of
// the function call a window reload is triggered. Returns that latest records.
func (s *Scroll) FetchScrollData(isReverse bool, window app.WindowNavigator) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetList()
	s.fetchScrollData(isReverse, window)
}

// fetchScrollData fetchs the scroll data and manages data returned depending on
// on the up or downwards scrollbar movement. Unless the data fetched is less than
// the page, all the old data is replaced by the new fetched data making it
// easier and smoother to scroll on the UI. At the end of the function call
// a window reload is triggered.
func (s *Scroll) fetchScrollData(isReverse bool, window app.WindowNavigator) {
	if s.isLoadingItems || s.loadedAllItems || s.queryFunc == nil {
		return
	}

	defer func() {
		s.isLoadingItems = false
		if isReverse {
			s.list.Position.Offset = int(math.Abs(float64(s.list.Position.OffsetLast + 4)))
			s.list.Position.OffsetLast = -4
		} else {
			s.list.Position.Offset = 4
			s.list.Position.OffsetLast = s.list.Position.OffsetLast + 4
		}
	}()

	s.isLoadingItems = true
	switch isReverse {
	case true:
		s.offset -= s.pageSize
	default:
		if s.data != nil {
			s.offset += s.pageSize
		}
	}

	tempSize := s.pageSize
	items, itemsLen, isReset, err := s.queryFunc(s.offset, tempSize*2)
	// Check if enough list items exists to fill the next page. If they do only query
	// enough items to fit the current page otherwise return all the queried items.
	if itemsLen > int(s.pageSize) && itemsLen%int(s.pageSize) == 0 {
		items, itemsLen, isReset, err = s.queryFunc(s.offset, s.pageSize)
	}
	if err != nil {
		errModal := modal.NewErrorModal(s.load, err.Error(), modal.DefaultClickFunc())
		window.ShowModal(errModal)
		return
	}

	if itemsLen > int(s.pageSize) {
		// Since this is the last page set of items, prevent further scroll down queries.
		s.loadedAllItems = true
	}

	s.data = items
	s.itemsCount = itemsLen

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
	s.offset = 0
	s.loadedAllItems = false
}

// OnScrollChangeListener listens for the scroll bar movement and update the items
// list view accordingly. FetchScrollData needs to be invoked first before calling
// this function.
func (s *Scroll) OnScrollChangeListener(window app.WindowNavigator) {
	defer s.mu.Unlock()
	s.mu.Lock()

	// Ignore if the query hasn't been invoked to fetch the list items.
	if s.itemsCount < int(s.pageSize) && s.itemsCount != -1 {
		return
	}

	// Ignore duplicate events triggered without moving the scrollbar by checking
	// if the current list offset matches the previously set offset.
	if s.listStyle == nil || s.list.Position.Offset == s.prevListOffset {
		return
	}

	// Ignore the first time OnScrollChangeListener is called. When loading the
	// page a prexisting list of items to display already exist therefore no need
	// to load it again.
	if s.prevListOffset == -1 {
		s.prevListOffset = s.list.Position.Offset
		return
	}

	scrollPos := s.list.Position
	// isScrollingDown checks when to fetch the Next page items because the scrollbar
	// has reached at the end of the current list loaded.
	// The -50 is to load more orders before reaching the end of the list.
	// (-50 is an arbitrary number)
	isScrollingDown := scrollPos.Offset > s.prevListOffset && scrollPos.OffsetLast >= -50
	// isScrollingUp checks when to fetch the Previous page items because the scrollbar
	// has reached at the beginning of the current list loaded.
	isScrollingUp := scrollPos.Offset < s.prevListOffset && scrollPos.Offset == 0 && s.offset > 0
	s.prevListOffset = scrollPos.Offset

	if isScrollingDown {
		// Enforce the first item to be at the list top.
		s.list.ScrollToEnd = false
		go s.fetchScrollData(false, window)
	}

	if isScrollingUp {
		// Enforce the first item to be at the list bottom.
		s.list.ScrollToEnd = true
		s.loadedAllItems = false
		go s.fetchScrollData(true, window)
	}
}
