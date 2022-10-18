package load

const Uint32Size = 32 // 32 or 64 ? shifting 32-bit value by 32 bits will always clear it
const MaxInt32 = 1<<(Uint32Size-1) - 1

// SetCurrentAppWidth stores the current width of the app's window.
func (l *Load) SetCurrentAppWidth(appWidth int) {
	l.CurrentAppWidth = appWidth
}

// GetCurrentAppWidth returns the current width of the app's window.
func (l *Load) GetCurrentAppWidth() int {
	return l.CurrentAppWidth
}
