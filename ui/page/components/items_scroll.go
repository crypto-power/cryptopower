package components

import (
	"math"
	"sync"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"gioui.org/layout"
	"gioui.org/widget"
)

// ScrollFunc is a query function that accepts offset and pagesize parameters and
// returns data interface, count of the items in the data interface and an error.
type ScrollFunc func(offset, pageSize int32) (data interface{}, count int, err error)

type Scroll struct {
	load       *load.Load
	list       *widget.List
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
func NewScroll(pageSize int32, queryFunc ScrollFunc) *Scroll {
	return &Scroll{
		list: &widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		pageSize:  pageSize,
		queryFunc: queryFunc,
	}
}

// FetchScrollData is a mutex protected fetchScrollData function.
func (s *Scroll) FetchScrollData(isReverse bool, window app.WindowNavigator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetchScrollData(isReverse, window)
}

// fetchScrollData fetchs the scroll data and manages data returned depending on
// on the up or downwards scrollbar movement. Unless the data fetched is less than
// the page, all the old data is replaced by the new fetched data making it
// easier and smoother to scroll on the UI.
func (s *Scroll) fetchScrollData(isReverse bool, window app.WindowNavigator) {
	if s.isLoadingItems || s.loadedAllItems {
		return
	}

	if s.queryFunc == nil {
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

	items, itemsLen, err := s.queryFunc(s.offset, s.pageSize)
	if err != nil {
		errModal := modal.NewErrorModal(s.load, err.Error(), modal.DefaultClickFunc())
		window.ShowModal(errModal)
		return
	}

	if itemsLen > 0 {
		s.data = items
		s.itemsCount = itemsLen
		if itemsLen < int(s.pageSize) {
			s.loadedAllItems = true
		}
	}
}

// FetchedData returns the latest queried data.
func (s *Scroll) FetchedData() interface{} {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.data
}

func (s *Scroll) List() *widget.List {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.list
}

// OnScrollChangeListener listens for the scroll bar movement and update the items
// list view accordingly.
func (s *Scroll) OnScrollChangeListener(window app.WindowNavigator) {
	defer s.mu.Unlock()
	s.mu.Lock()

	if s.itemsCount < int(s.pageSize) && s.itemsCount != -1 {
		return
	}

	if s.list.List.Position.OffsetLast == 0 && !s.list.List.Position.BeforeEnd {
		go s.fetchScrollData(false, window)
	}

	// Fetches preceeding pagesize items if the list scrollbar is at the beginning.
	if s.list.List.Position.BeforeEnd && s.list.Position.Offset == 0 && s.offset >= s.pageSize {
		if s.loadedAllItems {
			s.loadedAllItems = false
		}
		go s.fetchScrollData(true, window)
	}
}
