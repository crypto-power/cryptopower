package root

import (
	"context"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/page/dexclient"
	"gitlab.com/raedah/cryptopower/ui/page/settings"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const WalletDexServerSelectorID = "wallet_dex_server_selector"

type (
	C = layout.Context
	D = layout.Dimensions
)

type badWalletListItem struct {
	*libwallet.Wallet
	deleteBtn cryptomaterial.Button
}

type WalletDexServerSelector struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	*listeners.SyncProgressListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	scrollContainer *widget.List
	shadowBox       *cryptomaterial.Shadow
	addWalClickable *cryptomaterial.Clickable
	addDexClickable *cryptomaterial.Clickable
	settings        *cryptomaterial.Clickable

	// wallet selector options
	listLock             sync.Mutex
	mainWalletList       []*load.WalletItem
	watchOnlyWalletList  []*load.WalletItem
	badWalletsList       []*badWalletListItem
	walletsList          *cryptomaterial.ClickableList
	watchOnlyWalletsList *cryptomaterial.ClickableList
	walletSelected       func()

	// dex selector options
	knownDexServers   *cryptomaterial.ClickableList
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

	pg.addDexClickable = l.Theme.NewClickable(false)
	pg.addDexClickable.Radius = rad

	pg.settings = l.Theme.NewClickable(false)

	pg.initWalletSelectorOptions()
	pg.initDexServerSelectorOption()

	// init shared page functions
	toggleSync := func() {
		if pg.WL.MultiWallet.IsConnectedToDecredNetwork() {
			pg.WL.MultiWallet.CancelSync()
		} else {
			pg.startSyncing()
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
	pg.startDexClient()

	if pg.WL.MultiWallet.ReadBoolConfigValueForKey(load.AutoSyncConfigKey, false) {
		pg.startSyncing()
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletDexServerSelector) HandleUserInteractions() {
	pg.listLock.Lock()
	mainWalletList := pg.mainWalletList
	watchOnlyWalletList := pg.watchOnlyWalletList
	pg.listLock.Unlock()

	if ok, selectedItem := pg.walletsList.ItemClicked(); ok {
		pg.WL.SelectedWallet = mainWalletList[selectedItem]
		pg.walletSelected()
	}

	if ok, selectedItem := pg.watchOnlyWalletsList.ItemClicked(); ok {
		pg.WL.SelectedWallet = watchOnlyWalletList[selectedItem]
		pg.walletSelected()
	}

	for _, badWallet := range pg.badWalletsList {
		if badWallet.deleteBtn.Clicked() {
			pg.deleteBadWallet(badWallet.ID)
		}
	}

	if pg.addWalClickable.Clicked() {
		pg.ParentNavigator().Display(NewCreateWallet(pg.Load))
	}

	if pg.addDexClickable.Clicked() {
		dm := dexclient.NewAddDexModal(pg.Load)
		dm.OnDexAdded(func() {
			// TODO: go to the trade form
			log.Info("TODO: go to the trade form")
		})
		pg.ParentWindow().ShowModal(dm)
	}

	if pg.settings.Clicked() {
		pg.ParentNavigator().Display(settings.NewSettingsPage(pg.Load))
	}

	if ok, index := pg.knownDexServers.ItemClicked(); ok {
		knownDexServers := pg.mapKnowDexServers()
		dexServers := sortDexExchanges(knownDexServers)
		pg.dexServerSelected(dexServers[index])
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
	pageContent := []func(gtx C) D{
		pg.sectionTitle(values.String(values.StrSelectWalletToOpen)),
		pg.walletListLayout,
		pg.layoutAddMoreRowSection(pg.addWalClickable, values.String(values.StrAddWallet), pg.Theme.Icons.NewWalletIcon.Layout24dp),
		pg.sectionTitle(values.String(values.StrSelectWalletToOpen)),
		pg.dexServersLayout,
		pg.layoutAddMoreRowSection(pg.addDexClickable, values.String(values.StrAddDexServer), pg.Theme.Icons.DexIcon.Layout16dp),
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

func (pg *WalletDexServerSelector) layoutAddMoreRowSection(clk *cryptomaterial.Clickable, buttonText string, ic func(gtx C) D) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{
			Left:   values.MarginPadding5,
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding48,
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

func (pg *WalletDexServerSelector) startSyncing() {
	for _, wal := range pg.WL.SortedWalletList() {
		if !wal.HasDiscoveredAccounts && wal.IsLocked() {
			pg.unlockWalletForSyncing(wal)
			return
		}
	}

	err := pg.WL.SelectedWallet.Wallet.SpvSync()
	if err != nil {
		// show error dialog
		log.Info("Error starting sync:", err)
	}
}

func (pg *WalletDexServerSelector) unlockWalletForSyncing(wal *libwallet.Wallet) {
	spendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrResumeAccountDiscoveryTitle)).
		PasswordHint(values.String(values.StrSpendingPassword)).
		SetPositiveButtonText(values.String(values.StrUnlock)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := wal.UnlockWallet(wal.ID, []byte(password))
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			pm.Dismiss()
			pg.startSyncing()
			return true
		})
	pg.ParentWindow().ShowModal(spendingPasswordModal)
}
