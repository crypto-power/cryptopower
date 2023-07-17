package root

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/exchange"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletDexServerSelectorID = "wallet_dex_server_selector"

type (
	C = layout.Context
	D = layout.Dimensions
)

type badWalletListItem struct {
	*sharedW.Wallet
	deleteBtn cryptomaterial.Button
}

type WalletDexServerSelector struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	isListenerAdded bool

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	startSpvSync uint32
	isConnected  bool

	scrollContainer *widget.List
	shadowBox       *cryptomaterial.Shadow
	addWalClickable *cryptomaterial.Clickable
	exchangeBtn     *cryptomaterial.Clickable
	dcrdexBtn       *cryptomaterial.Clickable
	settings        *cryptomaterial.Clickable

	// wallet selector options
	listLock       sync.RWMutex
	walletsList    []*load.WalletItem
	badWalletsList []*badWalletListItem

	walletComponents *cryptomaterial.ClickableList
	walletSelected   func()

	// dex selector options
	dexServerSelected func(server string)
}

func NewWalletDexServerSelector(l *load.Load, onWalletSelected func(), onDexServerSelected func(server string)) *WalletDexServerSelector {
	pg := &WalletDexServerSelector{
		GenericPageModal: app.NewGenericPageModal(WalletDexServerSelectorID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		Load:      l,
		shadowBox: l.Theme.Shadow(),

		walletSelected:    onWalletSelected,
		dexServerSelected: onDexServerSelected,
	}

	rad := cryptomaterial.Radius(14)
	pg.addWalClickable = l.Theme.NewClickable(false)
	pg.addWalClickable.Radius = rad

	pg.exchangeBtn = l.Theme.NewClickable(false)
	pg.exchangeBtn.Radius = rad

	pg.dcrdexBtn = l.Theme.NewClickable(false)
	pg.dcrdexBtn.Radius = rad

	pg.settings = l.Theme.NewClickable(false)

	go func() {
		pg.isConnected = libutils.IsOnline()
	}()

	pg.initWalletSelectorOptions()

	// init shared page functions
	toggleSync := func(unlock load.NeedUnlockRestore) {
		if pg.WL.SelectedWallet.Wallet.IsConnectedToNetwork() {
			go pg.WL.SelectedWallet.Wallet.CancelSync()
			unlock(false)
		} else {
			pg.startSyncing(pg.WL.SelectedWallet.Wallet, unlock)
		}
	}
	l.ToggleSync = toggleSync

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *WalletDexServerSelector) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.listenForNotifications()
	pg.loadWallets()
	pg.loadBadWallets()

	// Initiate the auto sync for all the DCR wallets with set autosync.
	for _, wallet := range pg.WL.SortedWalletList(libutils.DCRWalletAsset) {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			pg.startSyncing(wallet, func(isUnlock bool) {})
		}
	}

	// Initiate the auto sync for all the BTC wallets with set autosync.
	for _, wallet := range pg.WL.SortedWalletList(libutils.BTCWalletAsset) {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			pg.startSyncing(wallet, func(isUnlock bool) {})
		}
	}

	// Initiate the auto sync for all the LTC wallets with set autosync.
	for _, wallet := range pg.WL.SortedWalletList(libutils.LTCWalletAsset) {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			pg.startSyncing(wallet, func(isUnlock bool) {})
		}
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletDexServerSelector) HandleUserInteractions() {
	pg.listLock.Lock()
	defer pg.listLock.Unlock()

	if ok, selectedItem := pg.walletComponents.ItemClicked(); ok {
		pg.WL.SelectedWallet = pg.walletsList[selectedItem]
		pg.walletSelected()
	}

	for _, badWallet := range pg.badWalletsList {
		if badWallet.deleteBtn.Clicked() {
			pg.deleteBadWallet(badWallet.ID)
			pg.ParentWindow().Reload()
		}
	}

	if pg.addWalClickable.Clicked() {
		pg.ParentNavigator().Display(NewCreateWallet(pg.Load))
	}

	if pg.exchangeBtn.Clicked() {
		pg.ParentNavigator().Display(exchange.NewCreateOrderPage(pg.Load))
	}

	if pg.settings.Clicked() {
		pg.ParentNavigator().Display(settings.NewSettingsPage(pg.Load))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletDexServerSelector) OnNavigatedFrom() {
	pg.ctxCancel()
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *WalletDexServerSelector) Layout(gtx C) D {
	pg.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *WalletDexServerSelector) layoutDesktop(gtx C) D {
	return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageHeaderLayout),
			layout.Rigid(func(gtx C) D {
				return pg.pageContentLayout(gtx)
			}),
		)
	})
}

func (pg *WalletDexServerSelector) layoutMobile(gtx C) D {
	return components.UniformMobile(gtx, false, false, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageHeaderLayout),
			layout.Rigid(pg.pageContentLayout),
		)
	})
}

func (pg *WalletDexServerSelector) pageHeaderLayout(gtx C) D {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Inset{
					Right:  values.MarginPadding15,
					Bottom: values.MarginPadding30,
				}.Layout(gtx, func(gtx C) D {
					return pg.settings.Layout(gtx, pg.Theme.Icons.SettingsIcon.Layout24dp)
				})
			})
		}),
	)
}

func (pg *WalletDexServerSelector) sectionTitle(title string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.Theme.Label(values.TextSize20, title).Layout)
	}
}

func (pg *WalletDexServerSelector) pageContentLayout(gtx C) D {
	// Set dcrdex button as disabled since its not yet implemented.
	pg.dcrdexBtn.SetEnabled(false, &gtx)

	pageContent := []func(gtx C) D{
		pg.sectionTitle(values.String(values.StrSelectWalletToOpen)),
		pg.walletListLayout,
		pg.layoutAddMoreRowSection(pg.addWalClickable, values.String(values.StrAddWallet), pg.Theme.Icons.NewWalletIcon.Layout24dp),
		pg.layoutAddBottomSpace(),
		pg.sectionTitle(values.String(values.StrExchangeIntro)),
		pg.layoutAddMoreRowSection(pg.exchangeBtn, values.String(values.StrExchange), pg.Theme.Icons.AddExchange.Layout16dp),
		pg.layoutAddMoreRowSection(pg.dcrdexBtn, values.String(values.StrDcrDex), pg.Theme.Icons.DcrDex.Layout16dp),
		pg.layoutAddBottomSpace(),
	}

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:  gtx.Dp(values.MarginPadding550),
			Height: cryptomaterial.MatchParent,
			Margin: layout.Inset{
				Bottom: values.MarginPadding30,
			},
		}.Layout2(gtx, func(gtx C) D {
			return pg.Theme.List(pg.scrollContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
				return layout.Inset{
					Right: values.MarginPadding48,
				}.Layout(gtx, pageContent[i])
			})
		})
	})
}

func (pg *WalletDexServerSelector) layoutAddBottomSpace() layout.Widget {
	return func(gtx C) D {
		return layout.Inset{
			Left:   values.MarginPadding5,
			Bottom: values.MarginPadding48,
		}.Layout(gtx, func(gtx C) D { return D{} })
	}
}

func (pg *WalletDexServerSelector) layoutAddMoreRowSection(clk *cryptomaterial.Clickable, buttonText string, ic func(gtx C) D) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{
			Left: values.MarginPadding5,
			Top:  values.MarginPadding10,
		}.Layout(gtx, func(gtx C) D {
			pg.shadowBox.SetShadowRadius(14)
			return cryptomaterial.LinearLayout{
				Width:      cryptomaterial.WrapContent,
				Height:     cryptomaterial.WrapContent,
				Padding:    layout.UniformInset(values.MarginPadding12),
				Background: pg.Theme.Color.Surface,
				Clickable:  clk,
				Shadow:     pg.shadowBox,
				Border:     cryptomaterial.Border{Radius: clk.Radius},
				Alignment:  layout.Middle,
			}.Layout(gtx,
				layout.Rigid(ic),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left: values.MarginPadding4,
						Top:  values.MarginPadding2,
					}.Layout(gtx, pg.Theme.Body2(buttonText).Layout)
				}),
			)
		})
	}
}

func (pg *WalletDexServerSelector) startSyncing(wallet sharedW.Asset, unlock load.NeedUnlockRestore) {
	// Watchonly wallets do not have any password neithers need one.
	if !wallet.ContainsDiscoveredAccounts() && wallet.IsLocked() && !wallet.IsWatchingOnlyWallet() {
		pg.unlockWalletForSyncing(wallet, unlock)
		return
	}
	unlock(true)

	if pg.isConnected {
		// once network connection has been established proceed to
		// start the wallet sync.
		if err := wallet.SpvSync(); err != nil {
			log.Debugf("Error starting sync: %v", err)
		}
		return
	}

	if !atomic.CompareAndSwapUint32(&pg.startSpvSync, 0, 1) {
		// internet connection checking goroutine is already running.
		return
	}

	// Since internet connectivity isn't available, the goroutine will keep
	// checking for the internet connectivity. on every 5th poll it will keep
	// increasing the wait duration by 5 seconds till the internet connectivity
	// is restored or the app is shutdown.
	go func() {
		count := 0
		counter := 5
		duration := time.Second * 10
		ticker := time.NewTicker(duration)

		for range ticker.C {
			if pg.isConnected {
				log.Info("Internet connection has been established")
				// once network connection has been established proceed to
				// start the wallet sync.
				if err := wallet.SpvSync(); err != nil {
					log.Debugf("Error starting sync: %v", err)
				}

				if pg.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.ExchangeHTTPAPI) {
					err := pg.WL.AssetsManager.InstantSwap.Sync(pg.ctx)
					if err != nil {
						log.Errorf("Error syncing instant swap: %v", err)
					}
				}

				// Trigger UI update
				pg.ParentWindow().Reload()

				ticker.Stop()
				return
			}

			// At the 5th ticker count, increase the duration interval by 5 seconds.
			if count%counter == 0 && count > 0 {
				duration += time.Second * 5
				// reset ticker
				ticker.Reset(duration)
			}
			// Increase the counter
			count++
			log.Debugf("Attempting to check for internet connection in %s", duration.String())

			go func() {
				pg.isConnected = libutils.IsOnline()
			}()
		}

		// Allow another goroutine to be spun up later on if need be.
		atomic.StoreUint32(&pg.startSpvSync, 0)
	}()
}

func (pg *WalletDexServerSelector) unlockWalletForSyncing(wal sharedW.Asset, unlock load.NeedUnlockRestore) {
	spendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrResumeAccountDiscoveryTitle)).
		PasswordHint(values.String(values.StrSpendingPassword)).
		SetPositiveButtonText(values.String(values.StrUnlock)).
		SetCancelable(false).
		SetNegativeButtonCallback(func() {
			unlock(false)
		}).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := wal.UnlockWallet(password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			unlock(true)
			pm.Dismiss()
			pg.startSyncing(wal, unlock)
			return true
		})
	pg.ParentWindow().ShowModal(spendingPasswordModal)
}
