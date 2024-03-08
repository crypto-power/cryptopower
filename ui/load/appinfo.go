package load

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	giouiApp "gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/assets"
	"github.com/crypto-power/cryptopower/ui/values"
)

type AssetsManagerInitFn func(utils.NetworkType) (*libwallet.AssetsManager, error)

// TODO: This should ultimately replace Load, acting as the container for all
// properties and methods that every app page and window relies on. Should
// probably rename as well.
type AppInfo struct {
	version     string
	buildDate   time.Time
	startUpTime time.Time

	cfg                   *AppConfig
	allowNetTypeSwitching bool
	initAssetsManager     AssetsManagerInitFn
	AssetsManager         *libwallet.AssetsManager

	windowMtx sync.Mutex
	window    *giouiApp.Window
	startPage app.Page

	currentAppWidth unit.Dp
	isMobileView    bool
}

// StartApp returns an instance of AppInfo with the startUpTime set to the
// current time. If netType is empty, the netType to use will be read from the
// appCfg. If a netType is provided, it'll be used instead of the netType in the
// appCfg; and in-app network type switching will be disabled.
func StartApp(version string, buildDate time.Time, netType string, appCfg *AppConfig, initAssetsManager AssetsManagerInitFn) (*AppInfo, error) {
	var allowNetTypeSwitching bool
	if netType == "" {
		netType = appCfg.Values().NetType
		allowNetTypeSwitching = true
	}

	net := utils.ToNetworkType(netType)
	if net == utils.Unknown {
		return nil, fmt.Errorf("invalid netType: %s", netType)
	}

	assetsManager, err := initAssetsManager(net)
	if err != nil {
		return nil, err
	}

	return &AppInfo{
		version:               version,
		buildDate:             buildDate,
		startUpTime:           time.Now(),
		cfg:                   appCfg,
		allowNetTypeSwitching: allowNetTypeSwitching,
		initAssetsManager:     initAssetsManager,
		AssetsManager:         assetsManager,
	}, nil
}

// BuildDate returns the app's build date.
func (app *AppInfo) BuildDate() time.Time {
	return app.buildDate
}

// Version returns the app's version.
func (app *AppInfo) Version() string {
	return app.version
}

// StartupTime returns the app's startup time.
func (app *AppInfo) StartupTime() time.Time {
	return app.startUpTime
}

// ReadyForDisplay marks the app as display-ready by storing the window and
// startPage used by the app.
//
// TODO: Is it possible to create Load here and bring the actual display
// functionality over from the ui package?
func (app *AppInfo) ReadyForDisplay(window *giouiApp.Window, startPage app.Page) {
	app.windowMtx.Lock()
	defer app.windowMtx.Unlock()

	if app.window != nil || app.startPage != nil {
		panic("duplicate call to AppInfo.ReadyForDisplay()")
	}

	app.window = window
	app.startPage = startPage
}

// Window returns the gio app window that hosts this app and is used to display
// different pages and modals.
//
// TODO: Is it possible for Window to be a custom type that has navigation
// methods?
func (app *AppInfo) Window() *giouiApp.Window {
	app.windowMtx.Lock()
	defer app.windowMtx.Unlock()
	return app.window
}

// StartPage returns the first page that is displayed when the app is launched.
// This page would be re-displayed if the app is restarted, e.g. when the
// network type is changed.
func (app *AppInfo) StartPage() app.Page {
	app.windowMtx.Lock()
	defer app.windowMtx.Unlock()
	return app.startPage
}

// CanChangeNetworkType is true if it is possible to change the network type
// used by the app.
func (app *AppInfo) CanChangeNetworkType() bool {
	return app.allowNetTypeSwitching
}

// ChangeAssetsManagerNetwork changes the network type used by the app to the
// value provided. A new AssetsManager for the specified network type is
// initialized and returned, but not used. Call ChangeAssetsManager to use the
// new AssetsManager.
func (app *AppInfo) ChangeAssetsManagerNetwork(netType utils.NetworkType) (*libwallet.AssetsManager, error) {
	if !app.allowNetTypeSwitching {
		return nil, fmt.Errorf("this operation is not permitted")
	}

	currentNetType := app.AssetsManager.NetType()
	if netType == currentNetType {
		return nil, fmt.Errorf("new network type is the same as current network type")
	}

	// Initialize a new AssetsManager for the new netType. This will be used to
	// replace the current AssetsManager after the current one is shutdown.
	newAssetsManager, err := app.initAssetsManager(netType)
	if err != nil {
		return nil, fmt.Errorf("failed to init new AssetsManager for %s network: %v", netType, err)
	}

	// Save the new netType to appCfg and only proceed with the network switch
	// if updating the cfg is successful.
	err = app.cfg.Update(func(appCfgValues *AppConfigValues) {
		appCfgValues.NetType = string(netType)
	})
	if err != nil {
		newAssetsManager.Shutdown()
		return nil, fmt.Errorf("update app config error: %v", err)
	}

	return newAssetsManager, nil
}

// ChangeAssetsManager closes all open pages, shuts down the current
// AssetsManager, switches to the provided AssetsManager and then restarts the
// app by displaying the app's first page.
//
// TODO: If *AppInfo.Window is changed to a custom type that implements
// app.WindowNavigator, this method won't need to take an app.PageNavigator
// parameter. See the TODO comment on *AppInfo.ReadyForDisplay() and
// *AppInfo.Window().
func (app *AppInfo) ChangeAssetsManager(newAssetsManager *libwallet.AssetsManager, pageNav app.PageNavigator) {
	if !app.allowNetTypeSwitching {
		// Should never happen, because *AppInfo.ChangeAssetsManagerNetwork()
		// would not even produce a newAssetsManager if network type change is
		// disabled.
		panic("network type change is not permitted")
	}

	appStartPage := app.StartPage()
	if appStartPage == nil {
		// Something is wrong. Close the app and let the user restart the app.
		// The new netType will be used.
		panic("cannot complete network type change without a pre-set app StartPage")
	}

	// Note when the network type / assets manager switch began. We want to
	// ensure that the temporary "restarting app" page is visible to the user
	// for some seconds at least.
	start := time.Now()

	// Close all pages that are currently open and display a temporary
	// "restarting app" page.
	currentNetType, newNetType := app.AssetsManager.NetType(), newAssetsManager.NetType()
	pageNav.ClearStackAndDisplay(networkSwitchTempPage(currentNetType, newNetType))

	// Display the newNetType on the app title if its not on mainnet.
	appTitle := giouiApp.Title(values.String(values.StrAppName))
	if newNetType != utils.Mainnet {
		appTitle = giouiApp.Title(values.StringF(values.StrAppTitle, newNetType.Display()))
	}
	app.Window().Option(appTitle)

	// Shutdown the current AssetsManager and begin using the new one.
	app.AssetsManager.Shutdown()
	app.AssetsManager = newAssetsManager

	// If the network type / assets manager switch was swift, wait a bit so the
	// user clearly sees the temporary "restarting app" page before displaying
	// the app's start page.
	minWait := 3 * time.Second
	if timeTaken := time.Since(start); timeTaken < minWait {
		<-time.After(minWait - timeTaken)
	}
	pageNav.ClearStackAndDisplay(appStartPage)
}

// SetCurrentAppWidth sets the specified value as the app's current width, using
// the provided device-dependent metric unit conversion.
//
// TODO: If actual display functionality is brought here over from the ui
// package, setting the app window's current width will be more seamless and
// this method can be unexported. See the TODO comment on
// *AppInfo.ReadyForDisplay().
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

func (app *AppInfo) IsIOS() bool {
	return runtime.GOOS == "ios"
}

// ConvertTextSize returns the appropriate text size for desktop and mobile,
// that corresponds to the provided value.
func (app *AppInfo) ConvertTextSize(size unit.Sp) unit.Sp {
	if !app.isMobileView {
		return size
	}
	switch size {
	case values.TextSize60:
		return values.TextSize36
	case values.TextSize20, values.TextSize24:
		return values.TextSize16
	case values.TextSize18:
		return values.TextSize14
	case values.TextSize16, values.TextSize14:
		return values.TextSize12
	default:
		return size
	}
}

// ConvertIconSize returns the appropriate icon size for desktop and mobile,
// that corresponds to the provided value.
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

func networkSwitchTempPage(currentNetType, newNetType utils.NetworkType) app.Page {
	theme := material.NewTheme(assets.FontCollection())
	text := fmt.Sprintf("Switching from %s to %s, please wait...", currentNetType, newNetType)
	lbl := material.Body1(theme, text)
	return app.NewWidgetDisplayPage(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Center.Layout(gtx, lbl.Layout)
	})
}
