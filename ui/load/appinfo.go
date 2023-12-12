package load

import (
	"time"

	"gioui.org/unit"
	"github.com/crypto-power/cryptopower/libwallet"
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

	AssetsManager *libwallet.AssetsManager
}

// StartApp returns an instance of AppInfo with the startUpTime set to the current time.
func StartApp(version string, buildDate time.Time, assetsManager *libwallet.AssetsManager) *AppInfo {
	return &AppInfo{
		version:       version,
		buildDate:     buildDate,
		startUpTime:   time.Now(),
		AssetsManager: assetsManager,
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

// This function return text size for desktop and mobile
func (app *AppInfo) ConvertTextSize(size unit.Sp) unit.Sp {
	if !app.isMobileView {
		return size
	}
	switch size {
	case values.TextSize20:
		return values.TextSize16
	case values.TextSize18:
		return values.TextSize14
	case values.TextSize16:
		return values.TextSize12
	case values.TextSize14:
		return values.TextSize12
	default:
		return size
	}
}

// This function return icon size for desktop and mobile
func (app *AppInfo) ConvertIconSize(size unit.Dp) unit.Dp {
	if !app.isMobileView {
		return size
	}
	switch size {
	case values.MarginPadding24:
		return values.MarginPadding16
	default:
		return size
	}
}
