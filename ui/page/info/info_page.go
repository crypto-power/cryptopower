package info

import (
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/privacy"
	"github.com/crypto-power/cryptopower/ui/page/seedbackup"
	"github.com/crypto-power/cryptopower/ui/page/staking"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/decred/dcrd/dcrutil/v4"
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
	wallet sharedW.Asset

	rescanUpdate *sharedW.HeadersRescanProgressReport

	container *widget.List

	transactions       []*sharedW.Transaction
	recentTransactions layout.List

	stakes       []*sharedW.Transaction
	recentStakes layout.List

	walletStatusIcon *cryptomaterial.Icon
	syncSwitch       *cryptomaterial.Switch
	toBackup         cryptomaterial.Button

	mixerInfoButton,
	mixerRedirectButton cryptomaterial.IconButton
	unmixedBalance sharedW.AssetAmount

	viewAllTxButton,
	viewAllStakeButton cryptomaterial.Button

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

func NewInfoPage(l *load.Load, wallet sharedW.Asset) *WalletInfo {
	pg := &WalletInfo{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(InfoID),
		wallet:           wallet,
		container: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		recentTransactions: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		recentStakes: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}
	pg.toBackup = pg.Theme.Button(values.String(values.StrBackupNow))
	pg.toBackup.Font.Weight = font.Medium
	pg.toBackup.TextSize = pg.ConvertTextSize(values.TextSize14)

	pg.viewAllTxButton = pg.Theme.OutlineButton(values.String(values.StrViewAll))
	pg.viewAllTxButton.Font.Weight = font.Medium
	pg.viewAllTxButton.TextSize = values.TextSize16
	pg.viewAllTxButton.Inset = layout.UniformInset(0)
	pg.viewAllTxButton.HighlightColor = color.NRGBA{}

	pg.viewAllStakeButton = pg.Theme.OutlineButton(values.String(values.StrViewAll))
	pg.viewAllStakeButton.Font.Weight = font.Medium
	pg.viewAllStakeButton.TextSize = values.TextSize16
	pg.viewAllStakeButton.Inset = layout.UniformInset(0)
	pg.viewAllTxButton.HighlightColor = color.NRGBA{}

	pg.mixerRedirectButton, pg.mixerInfoButton = components.SubpageHeaderButtons(l)
	pg.mixerRedirectButton.Icon = pg.Theme.Icons.NavigationArrowForward
	pg.mixerRedirectButton.Size = values.MarginPadding20

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
	autoSync := pg.wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false)
	pg.syncSwitch.SetChecked(autoSync)

	pg.listenForNotifications() // ntfn listeners are stopped in OnNavigatedFrom().

	pg.loadTransactions()

	if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
		pg.loadStakes()

		if pg.wallet.(*dcr.Asset).IsAccountMixerActive() {
			pg.listenForMixerNotifications()
			pg.reloadMixerBalances()
		}
	}
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
// Layout lays out the widgets for the main wallets pg.
func (pg *WalletInfo) Layout(gtx C) D {
	return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
		items := []layout.FlexChild{layout.Rigid(pg.walletInfoLayout)}

		if pg.wallet.GetAssetType() == libutils.DCRWalletAsset && pg.wallet.(*dcr.Asset).IsAccountMixerActive() {
			items = append(items, layout.Rigid(pg.mixerLayout))
		}

		if len(pg.transactions) > 0 {
			items = append(items, layout.Rigid(pg.recentTransactionLayout))
		}

		if len(pg.stakes) > 0 {
			items = append(items, layout.Rigid(pg.recentStakeLayout))
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
	})
}

func (pg *WalletInfo) walletInfoLayout(gtx C) D {
	return pg.pageContentWrapper(gtx, "", nil, func(gtx C) D {
		items := []layout.FlexChild{
			layout.Rigid(pg.walletNameAndBackupInfo),
			layout.Rigid(pg.syncStatusSection),
		}

		if len(pg.wallet.GetEncryptedSeed()) > 0 {
			items = append(items, layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, pg.toBackup.Layout)
					}),
				)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
	})
}

func (pg *WalletInfo) walletNameAndBackupInfo(gtx C) D {
	items := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return layout.Inset{
			Right: values.MarginPadding10,
		}.Layout(gtx, func(gtx C) D {
			txt := pg.Theme.Body1(strings.ToUpper(pg.wallet.GetWalletName()))
			txt.Font.Weight = font.SemiBold
			return txt.Layout(gtx)
		})
	})}

	if len(pg.wallet.GetEncryptedSeed()) > 0 {
		items = append(items, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(pg.Theme.Icons.RedAlert.Layout20dp),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left:  values.MarginPadding9,
						Right: values.MarginPadding16,
					}.Layout(gtx, pg.Theme.Body2(values.String(values.StrBackupWarning)).Layout)
				}),
			)
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, items...)
}

func (pg *WalletInfo) mixerLayout(gtx C) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return components.MixerComponent{
			Load:           pg.Load,
			WalletName:     pg.wallet.GetWalletName(),
			UnmixedBalance: pg.unmixedBalance.String(),
			ForwardButton:  pg.mixerRedirectButton,
			InfoButton:     pg.mixerInfoButton,
			Width:          cryptomaterial.MatchParent,
			Height:         cryptomaterial.WrapContent,
		}.MixerLayout(gtx)
	})
}

func (pg *WalletInfo) recentTransactionLayout(gtx C) D {
	return pg.pageContentWrapper(gtx, values.String(values.StrRecentTransactions), pg.viewAllTxButton.Layout, func(gtx C) D {
		return pg.recentTransactions.Layout(gtx, len(pg.transactions), func(gtx C, index int) D {
			tx := pg.transactions[index]
			isHiddenSeparator := index == len(pg.transactions)-1
			return pg.walletTxWrapper(gtx, tx, isHiddenSeparator)
		})
	})
}

func (pg *WalletInfo) recentStakeLayout(gtx C) D {
	return pg.pageContentWrapper(gtx, values.String(values.StrStakingActivity), pg.viewAllStakeButton.Layout, func(gtx C) D {
		return pg.recentStakes.Layout(gtx, len(pg.stakes), func(gtx C, index int) D {
			tx := pg.stakes[index]
			isHiddenSeparator := index == len(pg.stakes)-1
			return pg.walletTxWrapper(gtx, tx, isHiddenSeparator)
		})
	})
}

func (pg *WalletInfo) pageContentWrapper(gtx C, sectionTitle string, redirectBtn, body layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Bottom: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if sectionTitle == "" {
										return D{}
									}

									txt := pg.Theme.Body1(sectionTitle)
									txt.Font.Weight = font.SemiBold
									return txt.Layout(gtx)
								}),
								layout.Flexed(1, func(gtx C) D {
									if redirectBtn != nil {
										return layout.E.Layout(gtx, redirectBtn)
									}
									return D{}
								}),
							)
						})
					}),
					layout.Rigid(body),
				)
			})
		})
	})
}

func (pg *WalletInfo) walletTxWrapper(gtx C, tx *sharedW.Transaction, isHiddenSeparator bool) D {
	if !isHiddenSeparator {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		separator := pg.Theme.Separator()
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return components.LayoutTransactionRow(gtx, pg.Load, pg.wallet, tx, true)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					// Show bottom divider for all rows except last
					return layout.Inset{Left: values.MarginPadding32}.Layout(gtx, separator.Layout)
				})
			}),
		)
	}

	return components.LayoutTransactionRow(gtx, pg.Load, pg.wallet, tx, true)
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

	isSyncShutting := pg.wallet.IsSyncShuttingDown()
	pg.syncSwitch.SetEnabled(!isSyncShutting)
	if pg.syncSwitch.Changed() {
		if pg.wallet.IsRescanning() {
			pg.wallet.CancelRescan()
		}

		go func() {
			pg.ToggleSync(pg.wallet, func(b bool) {
				pg.syncSwitch.SetChecked(b)
				pg.wallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
			})
		}()
	}

	if pg.toBackup.Button.Clicked() {
		currentPage := pg.ParentWindow().CurrentPageID()
		pg.ParentWindow().Display(seedbackup.NewBackupInstructionsPage(pg.Load, pg.wallet, func(load *load.Load, navigator app.WindowNavigator) {
			navigator.ClosePagesAfter(currentPage)
		}))
	}

	// Navigate to mixer page when wallet mixer slider forward button is clicked.
	if pg.mixerRedirectButton.Button.Clicked() {
		pg.ParentNavigator().Display(privacy.NewAccountMixerPage(pg.Load, pg.wallet.(*dcr.Asset)))
	}

	if pg.viewAllTxButton.Button.Clicked() {
		pg.ParentNavigator().Display(transaction.NewTransactionsPage(pg.Load, pg.wallet))
	}

	if pg.viewAllStakeButton.Button.Clicked() {
		pg.ParentNavigator().Display(staking.NewStakingPage(pg.Load, pg.wallet.(*dcr.Asset)))
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
			// set the new progress against the associated asset.
			syncProgressInfo[pg.wallet] = progress

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

	err := pg.wallet.AddSyncProgressListener(syncProgressListener, InfoID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(walletID int, transaction *sharedW.Transaction) {
			pg.ParentWindow().Reload()
		},
		OnBlockAttached: func(walletID int, blockHeight int32) {
			pg.ParentWindow().Reload()
		},
	}
	err = pg.wallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, InfoID)
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
	pg.wallet.SetBlocksRescanProgressListener(blocksRescanProgressListener)
}

func (pg *WalletInfo) listenForMixerNotifications() {
	accountMixerNotificationListener := &dcr.AccountMixerNotificationListener{
		OnAccountMixerStarted: func(walletID int) {
			pg.reloadMixerBalances()
			pg.ParentWindow().Reload()
		},
		OnAccountMixerEnded: func(walletID int) {
			pg.reloadMixerBalances()
			pg.ParentWindow().Reload()
		},
	}
	err := pg.wallet.(*dcr.Asset).AddAccountMixerNotificationListener(accountMixerNotificationListener, InfoID)
	if err != nil {
		log.Errorf("Error adding account mixer notification listener: %+v", err)
		return
	}

	// this is needed to refresh the UI on every block
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnBlockAttached: func(walletID int, blockHeight int32) {
			pg.reloadMixerBalances()
			pg.ParentWindow().Reload()
		},
	}
	err = pg.wallet.(*dcr.Asset).AddTxAndBlockNotificationListener(txAndBlockNotificationListener, InfoID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}
}

func (pg *WalletInfo) reloadMixerBalances() {
	accounts, _ := pg.wallet.GetAccountsRaw()
	for _, acct := range accounts.Accounts {
		if acct.Number == pg.wallet.(*dcr.Asset).UnmixedAccountNumber() {
			bal := acct.Balance.Total
			// to prevent NPE set default amount 0 if asset amount is nil
			if bal == nil {
				bal = dcr.Amount(dcrutil.Amount(0))
			}
			pg.unmixedBalance = bal
		}
	}
}

func (pg *WalletInfo) loadTransactions() {
	txs, err := pg.wallet.GetTransactionsRaw(0, 3, libutils.TxFilterAllTx, true, "")
	if err != nil {
		log.Errorf("error loading transactions: %v", err)
		return
	}
	pg.transactions = txs
}

func (pg *WalletInfo) loadStakes() {
	pg.stakes = make([]*sharedW.Transaction, 0)

	txs, err := pg.wallet.GetTransactionsRaw(0, 10, libutils.TxFilterStaking, true, "")
	if err != nil {
		log.Errorf("error loading staking activities: %v", err)
		return
	}
	for _, stakeTx := range txs {
		if (stakeTx.Type == dcr.TxTypeTicketPurchase) || (stakeTx.Type == dcr.TxTypeRevocation) {
			pg.stakes = append(pg.stakes, stakeTx)
		}
	}
	if len(pg.stakes) > 3 {
		pg.stakes = pg.stakes[:3]
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedFrom() {
	pg.wallet.RemoveSyncProgressListener(InfoID)
	pg.wallet.RemoveTxAndBlockNotificationListener(InfoID)
	pg.wallet.SetBlocksRescanProgressListener(nil)
	if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
		pg.wallet.(*dcr.Asset).RemoveAccountMixerNotificationListener(InfoID)
	}
}
