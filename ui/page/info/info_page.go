package info

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/seedbackup"
	"github.com/crypto-power/cryptopower/ui/values"
)

const InfoID = "Info"

type (
	C = layout.Context
	D = layout.Dimensions
)

type WalletInfo struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	assetsManager *libwallet.AssetsManager

	rescanUpdate *sharedW.HeadersRescanProgressReport

	container *widget.List

	walletStatusIcon *cryptomaterial.Icon
	syncSwitch       *cryptomaterial.Switch
	toBackup         cryptomaterial.Button
	checkBox         cryptomaterial.CheckBoxStyle

	isStatusConnected bool
}

type progressInfo struct {
	remainingSyncTime    string
	headersToFetchOrScan int32
	stepFetchProgress    int32
	syncProgress         int
}

// SyncProgressInfo is made independent of the walletInfo struct so that once
// set with a value, it always persists till unset. This will help address the
// progress bar issue where, changing UI pages alters the progress on the sync
// status progress percentage.
var syncProgressInfo = map[sharedW.Asset]progressInfo{}

func NewInfoPage(l *load.Load) *WalletInfo {
	pg := &WalletInfo{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(InfoID),
		assetsManager:    l.WL.AssetsManager,
		container: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		checkBox: l.Theme.CheckBox(new(widget.Bool), values.String(values.StrAwareOfRisk)),
	}
	pg.toBackup = pg.Theme.Button(values.String(values.StrBackupNow))
	pg.toBackup.Font.Weight = font.Medium
	pg.toBackup.TextSize = values.TextSize14

	go func() {
		pg.isStatusConnected = libutils.IsOnline()
	}()

	pg.initWalletStatusWidgets()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedTo() {
	autoSync := pg.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false)
	pg.syncSwitch.SetChecked(autoSync)

	pg.listenForNotifications() // ntfn listeners are stopped in OnNavigatedFrom().
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
// Layout lays out the widgets for the main wallets pg.
func (pg *WalletInfo) Layout(gtx layout.Context) layout.Dimensions {
	return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
		return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
			return pg.Theme.Card().Layout(gtx, func(gtx C) D {
				return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Right: values.MarginPadding10,
								Left:  values.MarginPadding10,
							}.Layout(gtx, func(gtx C) D {
								txt := pg.Theme.Body1(pg.WL.SelectedWallet.Wallet.GetWalletName())
								txt.Font.Weight = font.SemiBold
								return txt.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx C) D {
							if len(pg.WL.SelectedWallet.Wallet.GetEncryptedSeed()) > 0 {
								return layout.Inset{
									Top: values.MarginPadding16,
								}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(pg.Theme.Icons.RedAlert.Layout24dp),
										layout.Rigid(func(gtx C) D {
											return layout.Inset{
												Left:  values.MarginPadding9,
												Right: values.MarginPadding16,
											}.Layout(gtx, pg.Theme.Body2(values.String(values.StrBackupWarning)).Layout)
										}),
										layout.Rigid(pg.toBackup.Layout),
									)
								})
							}
							return D{}
						}),
						layout.Rigid(pg.syncStatusSection),
					)
				})
			})
		})
	})

}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletInfo) HandleUserInteractions() {
	// As long as the internet connection hasn't been established keep checking.
	if !pg.isStatusConnected {
		go func() {
			pg.isStatusConnected = libutils.IsOnline()
		}()
	}

	isSyncShutting := pg.WL.SelectedWallet.Wallet.IsSyncShuttingDown()
	pg.syncSwitch.SetEnabled(!isSyncShutting)
	if pg.syncSwitch.Changed() {
		if pg.WL.SelectedWallet.Wallet.IsRescanning() {
			pg.WL.SelectedWallet.Wallet.CancelRescan()
		}
		go func() {
			pg.ToggleSync(func(b bool) {
				pg.syncSwitch.SetChecked(b)
				pg.WL.SelectedWallet.Wallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
			})
		}()
	}

	if pg.toBackup.Button.Clicked() {
		currentPage := pg.ParentWindow().CurrentPageID()
		pg.ParentWindow().Display(seedbackup.NewBackupInstructionsPage(pg.Load, pg.WL.SelectedWallet.Wallet, func(load *load.Load, navigator app.WindowNavigator) {
			navigator.ClosePagesAfter(currentPage)
		}))
	}
}

// listenForNotifications starts a goroutine to watch for sync updates and
// update the UI accordingly. To prevent UI lags, this method does not refresh
// the window display every time a sync update is received. During active blocks
// sync, rescan or proposals sync, the Layout method auto refreshes the display
// every set interval. Other sync updates that affect the UI but occur outside
// of an active sync requires a display refresh.
func (pg *WalletInfo) listenForNotifications() {
	updateSyncProgress := func(progress progressInfo) {
		// Update sync progress fields which will be displayed
		// when the next UI invalidation occurs.

		previousProgress := pg.fetchSyncProgress()
		// headers to fetch cannot be less than the previously fetched.
		// Page refresh only needed if there is new data to update the UI.
		if progress.headersToFetchOrScan >= previousProgress.headersToFetchOrScan {
			currentAsset := pg.WL.SelectedWallet.Wallet
			// set the new progress against the associated asset.
			syncProgressInfo[currentAsset] = progress

			// We only care about sync state changes here, to
			// refresh the window display.
			pg.ParentWindow().Reload()
		}
	}

	syncProgressListener := &sharedW.SyncProgressListener{
		OnHeadersFetchProgress: func(t *sharedW.HeadersFetchProgressReport) {
			progress := progressInfo{}
			progress.stepFetchProgress = t.HeadersFetchProgress
			progress.headersToFetchOrScan = t.TotalHeadersToFetch
			progress.syncProgress = int(t.TotalSyncProgress)
			progress.remainingSyncTime = components.TimeFormat(int(t.TotalTimeRemainingSeconds), true)
			updateSyncProgress(progress)
		},
		OnAddressDiscoveryProgress: func(t *sharedW.AddressDiscoveryProgressReport) {
			progress := progressInfo{}
			progress.syncProgress = int(t.TotalSyncProgress)
			progress.remainingSyncTime = components.TimeFormat(int(t.TotalTimeRemainingSeconds), true)
			progress.stepFetchProgress = t.AddressDiscoveryProgress
			updateSyncProgress(progress)
		},
		OnHeadersRescanProgress: func(t *sharedW.HeadersRescanProgressReport) {
			progress := progressInfo{}
			progress.headersToFetchOrScan = t.TotalHeadersToScan
			progress.syncProgress = int(t.TotalSyncProgress)
			progress.remainingSyncTime = components.TimeFormat(int(t.TotalTimeRemainingSeconds), true)
			progress.stepFetchProgress = t.RescanProgress
			updateSyncProgress(progress)
		},
		OnSyncCompleted: func() {
			pg.ParentWindow().Reload()
		},
	}

	err := pg.WL.SelectedWallet.Wallet.AddSyncProgressListener(syncProgressListener, InfoID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(transaction *sharedW.Transaction) {
			pg.ParentWindow().Reload()
		},
		OnBlockAttached: func(walletID int, blockHeight int32) {
			pg.ParentWindow().Reload()
		},
	}
	err = pg.WL.SelectedWallet.Wallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, InfoID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	blocksRescanProgressListener := &sharedW.BlocksRescanProgressListener{
		OnBlocksRescanStarted: func(walletID int) {
			pg.rescanUpdate = nil
		},
		OnBlocksRescanProgress: func(progress *sharedW.HeadersRescanProgressReport) {
			pg.rescanUpdate = progress
		},
		OnBlocksRescanEnded: func(walletID int, err error) {
			pg.rescanUpdate = nil
			pg.ParentWindow().Reload()
		},
	}
	pg.WL.SelectedWallet.Wallet.SetBlocksRescanProgressListener(blocksRescanProgressListener)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedFrom() {
	pg.WL.SelectedWallet.Wallet.RemoveSyncProgressListener(InfoID)
	pg.WL.SelectedWallet.Wallet.RemoveTxAndBlockNotificationListener(InfoID)
	pg.WL.SelectedWallet.Wallet.SetBlocksRescanProgressListener(nil)
}
