package ui

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	giouiApp "gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/crypto-power/cryptopower/app"
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

	// clicker used to create click events for application
	clicker gesture.Click
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
func CreateWindow(appInfo *load.AppInfo) (*Window, error) {
	appTitle := giouiApp.Title(values.String(values.StrAppName))
	// appSize overwrites gioui's default app size of 'Size(800, 600)'
	appSize := giouiApp.Size(values.AppWidth, values.AppHeight)
	// appMinSize is the minimum size the app.
	appMinSize := giouiApp.MinSize(values.MobileAppWidth, values.MobileAppHeight)
	// Display network on the app title if its not on mainnet.
	if net := appInfo.AssetsManager.NetType(); net != libutils.Mainnet {
		appTitle = giouiApp.Title(values.StringF(values.StrAppTitle, net.Display()))
	}

	ctx, cancel := context.WithCancel(context.Background())
	giouiWindow := new(giouiApp.Window)
	giouiWindow.Option(appSize, appMinSize, appTitle)
	win := &Window{
		ctx:        ctx,
		ctxCancel:  cancel,
		Window:     giouiWindow,
		navigator:  app.NewSimpleWindowNavigator(giouiWindow.Invalidate),
		Quit:       make(chan struct{}, 1),
		IsShutdown: make(chan struct{}, 1),
	}

	l, err := win.NewLoad(appInfo)
	if err != nil {
		return nil, err
	}

	win.load = l

	startPage := page.NewStartPage(win.ctx, win.load)
	win.load.AppInfo.ReadyForDisplay(win.Window, startPage)

	return win, nil
}

func (win *Window) NewLoad(appInfo *load.AppInfo) (*load.Load, error) {
	th := cryptomaterial.NewTheme(assets.FontCollection(), assets.DecredIcons, false)
	if th == nil {
		return nil, errors.New("unexpected error while loading theme")
	}

	// fetch status of the wallet if its online.
	go libutils.IsOnline()

	// Set the user-configured theme colors on app load.
	var isDarkModeOn bool
	if appInfo.AssetsManager.LoadedWalletsCount() > 0 {
		// A valid DB interface must have been set. Otherwise no valid wallet exists.
		isDarkModeOn = appInfo.AssetsManager.IsDarkModeOn()
	}
	th.SwitchDarkMode(isDarkModeOn, assets.DecredIcons)

	l := &load.Load{
		AppInfo: appInfo,

		Theme: th,

		// NB: Toasts implementation is maintained here for the cases where its
		// very essential to have a toast UI component implementation otherwise
		// restraints should be exercised when planning to reuse it else where.
		Toast: notification.NewToast(th),

		Printer: message.NewPrinter(language.English),
	}

	appInfo.AssetsManager.SetToast(l.Toast)

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

	events := make(chan event.Event)
	acks := make(chan struct{})
	go func() {
		for {
			ev := win.Event()
			events <- ev
			<-acks
			if _, ok := ev.(giouiApp.DestroyEvent); ok {
				return
			}
		}
	}()

	for {
		// Select either the os interrupt or the window event, whichever becomes
		// ready first.
		select {
		case <-done:
			displayShutdownPage()
		case <-win.IsShutdown:
			// backend processes shutdown is complete, exit UI process too.
			return
		case e := <-events:
			switch evt := e.(type) {
			case giouiApp.DestroyEvent:
				displayShutdownPage()
				acks <- struct{}{}
			case giouiApp.FrameEvent:
				ops := win.handleFrameEvent(evt)
				evt.Frame(ops)
			default:
				log.Tracef("Unhandled window event %v\n", e)
			}
			acks <- struct{}{}
		}
	}
}

// handleFrameEvent is called when a FrameEvent is received by the active
// window. It expects a new frame in the form of a list of operations that
// describes what to display and how to handle input. This operations list
// is returned to the caller for displaying on screen.
func (win *Window) handleFrameEvent(evt giouiApp.FrameEvent) *op.Ops {
	win.load.SetCurrentAppWidth(evt.Size.X, evt.Metric)
	ops := &op.Ops{}
	gtx := giouiApp.NewContext(ops, evt)

	switch {
	case win.navigator.CurrentPage() == nil:
		// Prepare to display the StartPage if no page is currently displayed.
		win.navigator.Display(win.load.StartPage())

	default:
		// The app window may have received some user interaction such as key
		// presses, a button click, etc which triggered this FrameEvent. Handle
		// such interactions before re-displaying the UI components. This
		// ensures that the proper interface is displayed to the user based on
		// the action(s) they just performed.
		win.navigator.CurrentPage().HandleUserInteractions(gtx)
		if modal := win.navigator.TopModal(); modal != nil {
			modal.Handle(gtx)
		}
	}

	// Generate an operations list with instructions for drawing the window's UI
	// components onto the screen. Use the generated ops to request key events.
	win.prepareToDisplayUI(gtx)
	win.addListenKeyEvent(gtx)
	return ops
}

// prepareToDisplayUI creates an operation list and writes the layout of all the
// window UI components into it. The created ops is returned and may be used to
// record further operations before finally being rendered on screen via
// system.FrameEvent.Frame(ops).
func (win *Window) prepareToDisplayUI(gtx layout.Context) {
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
	layout.Stack{Alignment: layout.N}.Layout(
		gtx,
		backgroundWidget,
		currentPageWidget,
		topModalLayout,
		layout.Stacked(win.load.Toast.Layout),
	)
	win.handleEvents(gtx)
}

func (win *Window) addListenKeyEvent(gtx C) {
	// Request key events on the top modal, if necessary.
	// Only request key events on the current page if no modal is displayed.
	if modal := win.navigator.TopModal(); modal != nil {
		if handler, ok := modal.(load.KeyEventHandler); ok {
			if len(handler.KeysToHandle()) == 0 || handler.KeysToHandle() == nil {
				return
			}
			for {
				e, ok := gtx.Event(handler.KeysToHandle()...)
				if !ok {
					break
				}
				switch e := e.(type) {
				case key.Event:
					handler.HandleKeyPress(gtx, &e)
				}
			}
		}
	} else {
		if handler, ok := win.navigator.CurrentPage().(load.KeyEventHandler); ok {
			if len(handler.KeysToHandle()) == 0 || handler.KeysToHandle() == nil {
				return
			}
			for {
				e, ok := gtx.Event(handler.KeysToHandle()...)
				if !ok {
					break
				}
				switch e := e.(type) {
				case key.Event:
					handler.HandleKeyPress(gtx, &e)
				}
			}
		}
	}
}

func (win *Window) handleEvents(gtx C) {
	win.handleUserClick(gtx)
	win.listenSoftKey(gtx)
}

// handleUserClick listen touch action of user for mobile.
func (win *Window) handleUserClick(gtx C) {
	for {
		evt, ok := win.clicker.Update(gtx.Source)
		if !ok {
			break
		}
		if evt.Kind == gesture.KindPress {
			win.load.Theme.AutoHideSoftKeyBoardAndMenuButton(gtx)
		}
	}
}

// handleShortKeys listen keys pressed.
func (win *Window) listenSoftKey(gtx C) {
	// clicker use for show and hide soft keyboard and menu button on editor
	win.clicker.Add(gtx.Ops)
	// check for presses of the back key.
	if runtime.GOOS == "android" {
		for {
			event, ok := gtx.Event(key.FocusFilter{Target: win},
				key.Filter{Focus: win, Name: key.NameBack},
			)
			if !ok {
				break
			}

			switch event := event.(type) {
			case key.Event:
				if event.Name == key.NameBack && event.State == key.Press {
					win.load.Theme.OnTapBack()
				}
			}
		}
	}
}
