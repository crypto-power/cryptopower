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

type dataList[T any] struct {
	items    []T
	idxStart int
	idxEnd   int
}

type Scroll[T any] struct {
	load      *load.Load
	list      *widget.List
	listStyle *cryptomaterial.ListStyle

	prevListOffset int
	prevScrollView int

	pageSize   int32
	listSize   int32
	offset     int32
	itemsCount int
	queryFunc  ScrollFunc[T]
	data       *dataList[T]
	cacheData  []T

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
		listSize:       pageSize * 2,
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
	}

	if isResetList {
		s.loadedAllItems = false
		s.isLoadingItems = false
		s.offset = 0
		s.itemsCount = 0
		s.data = nil
		s.cacheData = nil
	}
	s.mu.Unlock()
	if s.data == nil {
		s.fetchScrollData(isScrollUp, window)
	}
}

// fetchScrollData fetchs the scroll data and manages data returned depending on
// on the up or downwards scrollbar movement. Unless the data fetched is less than
// the page, all the old data is replaced by the new fetched data making it
// easier and smoother to scroll on the UI. At the end of the function call
// a window reload is triggered.
func (s *Scroll[T]) IsLoadingData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isLoadingItems
}

func (s *Scroll[T]) fetchScrollData(isScrollUp bool, window app.WindowNavigator) {
	if s.isLoadingItems || s.queryFunc == nil {
		return
	}

	s.mu.Lock()
	tempSize := s.pageSize
	//Scroll up and down without load more
	if s.data != nil {
		temStartIdx := s.data.idxStart
		if isScrollUp {
			if s.data.idxStart <= 0 {
				s.mu.Unlock()
				return
			}
			itemsUp := s.cacheData[s.data.idxStart-int(s.pageSize) : s.data.idxStart-1]
			itemdata := s.data.items[:int(s.pageSize)-1]
			s.data.items = append(itemsUp, itemdata...)
			if s.data.idxStart < int(s.pageSize) {
				s.data.idxStart = 0
			} else {
				s.data.idxStart = s.data.idxStart - int(s.pageSize)
			}
			s.data.idxEnd = s.data.idxEnd - int(s.pageSize)
			s.itemsCount = len(s.data.items)
			if temStartIdx > 0 {
				s.list.Position.Offset = s.list.Position.Length / len(s.data.items) * (int(s.pageSize) - 4)
			}
			s.mu.Unlock()
			return
		} else {
			if s.data.idxEnd < len(s.cacheData)-1 {
				itemsDown := s.cacheData[s.data.idxEnd+1 : s.data.idxEnd+int(s.pageSize)]
				s.data.items = s.data.items[int(s.pageSize):]
				s.data.items = append(s.data.items, itemsDown...)
				s.data.idxStart = s.data.idxStart + int(s.pageSize)
				s.data.idxEnd = s.data.idxEnd + int(s.pageSize)
				s.itemsCount = len(s.data.items)
				if temStartIdx > 0 {
					s.list.Position.Offset = s.list.Position.Length / len(s.data.items) * (int(s.pageSize) - 4)
				}
				s.mu.Unlock()
				return
			}
		}
	}

	// handle when need to load more items.
	if s.data == nil {
		s.data = &dataList[T]{
			idxEnd: -1,
		}
		tempSize = s.pageSize * 2
	}
	s.mu.Unlock()
	if s.loadedAllItems && !isScrollUp {
		s.isLoadingItems = false
		return
	}

	items, _, _, err := s.queryFunc(s.offset, tempSize)
	if len(items) <= 0 {
		s.isLoadingItems = false
		return
	}

	s.mu.Lock()
	s.isLoadingItems = false
	if err != nil {
		errModal := modal.NewErrorModal(s.load, err.Error(), modal.DefaultClickFunc())
		window.ShowModal(errModal)
		s.mu.Unlock()
		return
	}

	if s.cacheData == nil {
		s.cacheData = make([]T, 0)
	}

	s.cacheData = append(s.cacheData, items...)
	itemCount := len(items)

	if len(s.data.items) > itemCount {
		s.data.items = s.data.items[s.pageSize-1:]
		s.data.idxStart += itemCount
	}
	s.data.items = append(s.data.items, items...)
	s.data.idxEnd += itemCount
	s.offset += int32(itemCount)
	s.itemsCount = len(s.data.items)
	if s.data.idxStart > 0 {
		s.list.Position.Offset = s.list.Position.Length / len(s.data.items) * (int(s.pageSize) - 4)
	}
	if itemCount < int(s.pageSize) {
		s.loadedAllItems = true
	}
	s.mu.Unlock()
}

// FetchedData returns the latest queried data.
func (s *Scroll[T]) FetchedData() []T {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.data.items
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
