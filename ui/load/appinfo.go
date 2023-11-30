package load

import (
	"time"

	"gioui.org/unit"
	"github.com/crypto-power/cryptopower/ui/values"
)

// TODO: This should ultimately replace Load, acting as the container for all
// properties and methods that every app page and window relies on. Should
// probably rename as well.
type AppInfo struct {
	version     string
	buildDate   time.Time
	startUpTime time.Time

	currentAppWidth unit.Dp
	isMobileView    bool
}

// StartApp returns an instance of AppInfo with the startUpTime set to the current time.
func StartApp(version string, buildDate time.Time) *AppInfo {
	return &AppInfo{
		version:     version,
		buildDate:   buildDate,
		startUpTime: time.Now(),
	}
}

func (app *AppInfo) BuildDate() time.Time {
	return app.buildDate
}

func (app *AppInfo) Version() string {
	return app.version
}

func (app *AppInfo) StartupTime() time.Time {
	return app.startUpTime
}

// SetCurrentAppWidth sets the specified value as the app's current width, using
// the provided device-dependent metric unit conversion.
func (app *AppInfo) SetCurrentAppWidth(appWidth int, metric unit.Metric) {
	app.currentAppWidth = metric.PxToDp(appWidth)
	app.isMobileView = app.currentAppWidth <= values.StartMobileView
}

// CurrentAppWidth returns the current width of the app's window.
func (app *AppInfo) CurrentAppWidth() unit.Dp {
	return app.currentAppWidth
}

// IsMobileView returns true if the app's window width is less than the mobile
// view width.
func (app *AppInfo) IsMobileView() bool {
	return app.isMobileView
}
