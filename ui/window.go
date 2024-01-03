package ui

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	giouiApp "gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/assets"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/notification"
	"github.com/crypto-power/cryptopower/ui/page"
	"github.com/crypto-power/cryptopower/ui/values"
)

// Window represents the app window (and UI in general). There should only be one.
// Window maintains an internal state of variables to determine what to display at
// any point in time.
type Window struct {
	*giouiApp.Window
	navigator app.WindowNavigator

	ctx       context.Context
	ctxCancel context.CancelFunc

	load *load.Load

	// Quit channel used to trigger background process to begin implementing the
	// shutdown protocol.
	Quit chan struct{}
	// IsShutdown channel is used to report that background processes have
	// completed shutting down, therefore the UI processes can finally stop.
	IsShutdown chan struct{}
}

type (
	C = layout.Context
	D = layout.Dimensions
)

type WriteClipboard struct {
	Text string
}

// CreateWindow creates and initializes a new window with start
// as the first page displayed.
// Should never be called more than once as it calls
// app.NewWindow() which does not support being called more
// than once.
func CreateWindow(mw *libwallet.AssetsManager, version string, buildDate time.Time) (*Window, error) {
	appTitle := giouiApp.Title(values.String(values.StrAppName))
	// appSize overwrites gioui's default app size of 'Size(800, 600)'
	appSize := giouiApp.Size(values.AppWidth, values.AppHeight)
	// appMinSize is the minimum size the app.
	appMinSize := giouiApp.MinSize(values.MobileAppWidth, values.MobileAppHeight)
	// Display network on the app title if its not on mainnet.
	if net := mw.NetType(); net != libutils.Mainnet {
		appTitle = giouiApp.Title(values.StringF(values.StrAppTitle, net.Display()))
	}

	ctx, cancel := context.WithCancel(context.Background())
	giouiWindow := giouiApp.NewWindow(appSize, appMinSize, appTitle)
	win := &Window{
		ctx:        ctx,
		ctxCancel:  cancel,
		Window:     giouiWindow,
		navigator:  app.NewSimpleWindowNavigator(giouiWindow.Invalidate),
		Quit:       make(chan struct{}, 1),
		IsShutdown: make(chan struct{}, 1),
	}

	l, err := win.NewLoad(mw, version, buildDate)
	if err != nil {
		return nil, err
	}
	win.load = l

	return win, nil
}

func (win *Window) NewLoad(mw *libwallet.AssetsManager, version string, buildDate time.Time) (*load.Load, error) {
	th := cryptomaterial.NewTheme(assets.FontCollection(), assets.DecredIcons, false)
	if th == nil {
		return nil, errors.New("unexpected error while loading theme")
	}

	// fetch status of the wallet if its online.
	go libutils.IsOnline()

	// Set the user-configured theme colors on app load.
	var isDarkModeOn bool
	if mw.LoadedWalletsCount() > 0 {
		// A valid DB interface must have been set. Otherwise no valid wallet exists.
		isDarkModeOn = mw.IsDarkModeOn()
	}
	th.SwitchDarkMode(isDarkModeOn, assets.DecredIcons)

	l := &load.Load{
		AppInfo: load.StartApp(version, buildDate, mw),

		Theme: th,

		// NB: Toasts implementation is maintained here for the cases where its
		// very essential to have a toast UI component implementation otherwise
		// restraints should be exercised when planning to reuse it else where.
		Toast: notification.NewToast(th),

		Printer: message.NewPrinter(language.English),
	}

	// DarkModeSettingChanged checks if any page or any
	// modal implements the AppSettingsChangeHandler
	l.DarkModeSettingChanged = func(isDarkModeOn bool) {
		if page, ok := win.navigator.CurrentPage().(load.AppSettingsChangeHandler); ok {
			page.OnDarkModeChanged(isDarkModeOn)
		}
		if modal := win.navigator.TopModal(); modal != nil {
			if modal, ok := modal.(load.AppSettingsChangeHandler); ok {
				modal.OnDarkModeChanged(isDarkModeOn)
			}
		}
	}

	l.LanguageSettingChanged = func() {
		if page, ok := win.navigator.CurrentPage().(load.AppSettingsChangeHandler); ok {
			page.OnLanguageChanged()
		}
	}

	l.CurrencySettingChanged = func() {
		if page, ok := win.navigator.CurrentPage().(load.AppSettingsChangeHandler); ok {
			page.OnCurrencyChanged()
		}
	}

	return l, nil
}

// HandleEvents runs main event handling and page rendering loop.
func (win *Window) HandleEvents() {
	done := make(chan os.Signal, 1)
	if runtime.GOOS == "windows" {
		// For controlled shutdown to work on windows, the channel has to be
		// listening to all signals.
		// https://github.com/golang/go/commit/8cfa01943a7f43493543efba81996221bb0f27f8
		signal.Notify(done)
	} else {
		// Signals are primarily used on Unix-like systems.
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	}

	var isShuttingDown bool

	displayShutdownPage := func() {
		if isShuttingDown {
			return
		}
		isShuttingDown = true

		log.Info("...Initiating the app shutdown protocols...")
		// clear all stack and display the shutdown page as backend processes are
		// terminating.
		win.navigator.ClearStackAndDisplay(page.NewStartPage(win.ctx, win.load, true))
		win.ctxCancel()
		// Trigger the backend processes shutdown.
		win.Quit <- struct{}{}
	}

	for {
		// Select either the os interrupt or the window event, whichever becomes
		// ready first.
		select {
		case <-done:
			displayShutdownPage()
		case <-win.IsShutdown:
			// backend processes shutdown is complete, exit UI process too.
			return
		case e := <-win.Events():
			switch evt := e.(type) {

			case system.DestroyEvent:
				displayShutdownPage()

			case system.FrameEvent:
				ops := win.handleFrameEvent(evt)
				evt.Frame(ops)

			default:
				log.Tracef("Unhandled window event %v\n", e)
			}
		}
	}
}

// handleFrameEvent is called when a FrameEvent is received by the active
// window. It expects a new frame in the form of a list of operations that
// describes what to display and how to handle input. This operations list
// is returned to the caller for displaying on screen.
func (win *Window) handleFrameEvent(evt system.FrameEvent) *op.Ops {
	win.load.SetCurrentAppWidth(evt.Size.X, evt.Metric)

	switch {
	case win.navigator.CurrentPage() == nil:
		// Prepare to display the StartPage if no page is currently displayed.
		win.navigator.Display(page.NewStartPage(win.ctx, win.load))

	default:
		// The app window may have received some user interaction such as key
		// presses, a button click, etc which triggered this FrameEvent. Handle
		// such interactions before re-displaying the UI components. This
		// ensures that the proper interface is displayed to the user based on
		// the action(s) they just performed.
		win.handleRelevantKeyPresses(evt)
		win.navigator.CurrentPage().HandleUserInteractions()
		if modal := win.navigator.TopModal(); modal != nil {
			modal.Handle()
		}
	}

	// Generate an operations list with instructions for drawing the window's UI
	// components onto the screen. Use the generated ops to request key events.
	ops := win.prepareToDisplayUI(evt)
	win.addKeyEventRequestsToOps(ops)

	return ops
}

// handleRelevantKeyPresses checks if any open modal or the current page is a
// load.KeyEventHandler AND if the provided system.FrameEvent contains key press
// events for the modal or page.
func (win *Window) handleRelevantKeyPresses(evt system.FrameEvent) {
	handleKeyPressFor := func(tag string, maybeHandler interface{}) {
		handler, ok := maybeHandler.(load.KeyEventHandler)
		if !ok {
			return
		}
		for _, event := range evt.Queue.Events(tag) {
			if keyEvent, isKeyEvent := event.(key.Event); isKeyEvent && keyEvent.State == key.Press {
				handler.HandleKeyPress(&keyEvent)
			}
		}
	}

	// Handle key events on the top modal first, if there's one.
	// Only handle key events on the current page if no modal is displayed.
	if modal := win.navigator.TopModal(); modal != nil {
		handleKeyPressFor(modal.ID(), modal)
	} else {
		handleKeyPressFor(win.navigator.CurrentPageID(), win.navigator.CurrentPage())
	}
}

// prepareToDisplayUI creates an operation list and writes the layout of all the
// window UI components into it. The created ops is returned and may be used to
// record further operations before finally being rendered on screen via
// system.FrameEvent.Frame(ops).
func (win *Window) prepareToDisplayUI(evt system.FrameEvent) *op.Ops {
	backgroundWidget := layout.Expanded(func(gtx C) D {
		return win.load.Theme.DropdownBackdrop.Layout(gtx, func(gtx C) D {
			return cryptomaterial.Fill(gtx, win.load.Theme.Color.Gray4)
		})
	})

	currentPageWidget := layout.Stacked(func(gtx C) D {
		if modal := win.navigator.TopModal(); modal != nil {
			gtx = gtx.Disabled()
		}
		if win.navigator.CurrentPage() == nil {
			win.navigator.Display(page.NewStartPage(win.ctx, win.load))
		}
		return win.load.Theme.DropdownBackdrop.Layout(gtx, func(gtx C) D {
			return win.navigator.CurrentPage().Layout(gtx)
		})
	})

	topModalLayout := layout.Stacked(func(gtx C) D {
		modal := win.navigator.TopModal()
		if modal == nil {
			return layout.Dimensions{}
		}
		return modal.Layout(gtx)
	})

	// Use a StackLayout to write the above UI components into an operations
	// list via a graphical context that is linked to the ops.
	ops := &op.Ops{}
	gtx := layout.NewContext(ops, evt)
	layout.Stack{Alignment: layout.N}.Layout(
		gtx,
		backgroundWidget,
		currentPageWidget,
		topModalLayout,
		layout.Stacked(win.load.Toast.Layout),
	)

	return ops
}

// addKeyEventRequestsToOps checks if the current page or any modal has
// registered to be notified of certain key events and updates the provided
// operations list with instructions to generate a FrameEvent if any of the
// desired keys is pressed on the window.
func (win *Window) addKeyEventRequestsToOps(ops *op.Ops) {
	requestKeyEvents := func(tag string, desiredKeys key.Set) {
		if desiredKeys == "" {
			return
		}

		// Execute the key.InputOP{}.Add operation after all other operations.
		// This is particularly important because some pages call op.Defer to
		// signify that some operations should be executed after all other
		// operations, which has an undesirable effect of discarding this key
		// operation unless it's done last, after all other defers are done.
		m := op.Record(ops)
		key.InputOp{Tag: tag, Keys: desiredKeys}.Add(ops)
		op.Defer(ops, m.Stop())
	}

	// Request key events on the top modal, if necessary.
	// Only request key events on the current page if no modal is displayed.
	if modal := win.navigator.TopModal(); modal != nil {
		if handler, ok := modal.(load.KeyEventHandler); ok {
			requestKeyEvents(modal.ID(), handler.KeysToHandle())
		}
	} else {
		if handler, ok := win.navigator.CurrentPage().(load.KeyEventHandler); ok {
			requestKeyEvents(win.navigator.CurrentPageID(), handler.KeysToHandle())
		}
	}
}
