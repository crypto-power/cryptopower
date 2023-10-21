package ext

// RateListener listens for new tickers and rate source change notifications.
type RateListener struct {
	RateUpdateChan chan *struct{}
}

func NewRateListener() *RateListener {
	return &RateListener{
		RateUpdateChan: make(chan *struct{}),
	}
}

func (rl *RateListener) Notify() {
	select {
	case rl.RateUpdateChan <- &struct{}{}:
	default:
	}
}
