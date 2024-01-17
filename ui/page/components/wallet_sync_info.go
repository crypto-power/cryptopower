package components

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletSyncInfoID = "WalletSyncInfo"

type WalletSyncInfo struct {
	*load.Load
	wallet           sharedW.Asset
	rescanUpdate     *sharedW.HeadersRescanProgressReport
	walletStatusIcon *cryptomaterial.Icon
	syncSwitch       *cryptomaterial.Switch
	toBackup         cryptomaterial.Button

	isStatusConnected bool
	reload            Reload
	backup            func(sharedW.Asset)
	ForwardButton     cryptomaterial.IconButton

	IsSlider bool
}

type ProgressInfo struct {
	remainingSyncTime    string
	HeadersToFetchOrScan int32
	stepFetchProgress    int32
	syncProgress         int
}

type Reload func()

// SyncProgressInfo is made independent of the walletInfo struct so that once
// set with a value, it always persists till unset. This will help address the
// progress bar issue where, changing UI pages alters the progress on the sync
// status progress percentage.
var syncProgressInfo = map[sharedW.Asset]ProgressInfo{}

func NewWalletSyncInfo(l *load.Load, wallet sharedW.Asset, reload Reload, backup func(sharedW.Asset)) *WalletSyncInfo {
	wsi := &WalletSyncInfo{
		Load:             l,
		wallet:           wallet,
		reload:           reload,
		walletStatusIcon: cryptomaterial.NewIcon(l.Theme.Icons.DotIcon),
		syncSwitch:       l.Theme.Switch(),
		backup:           backup,
	}

	wsi.toBackup = l.Theme.Button(values.String(values.StrBackupNow))
	wsi.toBackup.Font.Weight = font.Medium
	wsi.toBackup.TextSize = l.ConvertTextSize(values.TextSize14)

	wsi.ForwardButton, _ = SubpageHeaderButtons(l)
	wsi.ForwardButton.Icon = wsi.Theme.Icons.NavigationArrowForward
	wsi.ForwardButton.Size = values.MarginPadding20
	return wsi
}

func (wsi *WalletSyncInfo) Init() {
	autoSync := wsi.wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false)
	wsi.syncSwitch.SetChecked(autoSync)
	go func() {
		wsi.isStatusConnected = libutils.IsOnline()
	}()
}

func (wsi *WalletSyncInfo) GetWallet() sharedW.Asset {
	return wsi.wallet
}

func (wsi *WalletSyncInfo) WalletInfoLayout(gtx C) D {
	wsi.handle()

	return wsi.pageContentWrapper(gtx, "", nil, func(gtx C) D {
		items := []layout.FlexChild{
			layout.Rigid(wsi.walletNameAndBackupInfo),
			layout.Rigid(wsi.syncStatusSection),
		}

		if len(wsi.wallet.GetEncryptedSeed()) > 0 {
			items = append(items, layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, wsi.toBackup.Layout)
			}))
			if wsi.IsSlider {
				items = append(items, layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout))
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
	})
}

func (wsi *WalletSyncInfo) pageContentWrapper(gtx C, sectionTitle string, redirectBtn, body layout.Widget) D {
	return wsi.Theme.Card().Layout(gtx, func(gtx C) D {
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
								txt := wsi.Theme.Body1(sectionTitle)
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
}

func (wsi *WalletSyncInfo) walletNameAndBackupInfo(gtx C) D {
	items := make([]layout.FlexChild, 0)
	if wsi.IsSlider {
		items = append(items, layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding10,
			}.Layout(gtx, func(gtx C) D {
				icon := wsi.Theme.AssetIcon(wsi.wallet.GetAssetType())
				return icon.Layout16dp(gtx)
			})
		}))
	}
	items = append(items, layout.Rigid(func(gtx C) D {
		return layout.Inset{
			Right: values.MarginPadding10,
		}.Layout(gtx, func(gtx C) D {
			txt := wsi.Theme.Body1(strings.ToUpper(wsi.wallet.GetWalletName()))
			txt.Font.Weight = font.SemiBold
			return txt.Layout(gtx)
		})
	}))

	if len(wsi.wallet.GetEncryptedSeed()) > 0 {
		items = append(items, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(wsi.Theme.Icons.RedAlert.Layout20dp),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left:  values.MarginPadding9,
						Right: values.MarginPadding16,
					}.Layout(gtx, wsi.Theme.Body2(values.String(values.StrBackupWarning)).Layout)
				}),
			)
		}))
	}

	if wsi.IsSlider {
		items = append(items, layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, wsi.ForwardButton.Layout)
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, items...)
}

// syncStatusSection lays out content for displaying sync status.
func (wsi *WalletSyncInfo) syncStatusSection(gtx C) D {
	isBtcAsset := wsi.wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := wsi.wallet.GetAssetType() == libutils.LTCWalletAsset
	syncing := wsi.wallet.IsSyncing()

	// btcwallet does not export implementation to track address discovery.
	// During btc address discovery, show the normal synced info page with an
	// extra label showing the address discovery is in progress.
	rescanning := wsi.wallet.IsRescanning() && !isLtcAsset && !isBtcAsset && !syncing

	uniform := layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding5}
	return wsi.Theme.Card().Layout(gtx, func(gtx C) D {
		return Container{Padding: layout.Inset{
			Top:    values.MarginPadding15,
			Bottom: values.MarginPadding16,
		}}.Layout(gtx, func(gtx C) D {
			items := []layout.FlexChild{layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, wsi.syncBoxTitleRow)
			})}

			if syncing || rescanning {
				items = append(items, layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(.93, wsi.progressBarRow),
							layout.Rigid(wsi.syncStatusIcon),
						)
					})
				}))
			}

			if rescanning {
				items = append(items, layout.Rigid(func(gtx C) D {
					return wsi.rescanDetailsLayout(gtx, uniform)
				}))
			} else {
				items = append(items, layout.Rigid(func(gtx C) D {
					return wsi.syncContent(gtx, uniform)
				}))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
	})
}

func (wsi *WalletSyncInfo) rescanDetailsLayout(gtx C, inset layout.Inset) D {
	if !wsi.wallet.IsRescanning() || wsi.rescanUpdate == nil {
		return D{}
	}
	return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		card := wsi.Theme.Card()
		card.Color = wsi.Theme.Color.Gray4
		return card.Layout(gtx, func(gtx C) D {
			return Container{Padding: layout.UniformInset(values.MarginPadding16)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							return wsi.Theme.Body1(wsi.wallet.GetWalletName()).Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							headersFetchedTitleLabel := wsi.labelTexSize16Layout(values.String(values.StrBlocksScanned), 0, true)
							blocksScannedLabel := wsi.labelTexSize16Layout(fmt.Sprint(wsi.rescanUpdate.CurrentRescanHeight), 0, false)
							return EndToEndRow(gtx, headersFetchedTitleLabel, blocksScannedLabel)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							progressTitleLabel := wsi.labelTexSize16Layout(values.String(values.StrSyncingProgress), 0, true)
							rescanProgress := values.StringF(values.StrBlocksLeft, wsi.rescanUpdate.TotalHeadersToScan-wsi.rescanUpdate.CurrentRescanHeight)
							blocksScannedLabel := wsi.labelTexSize16Layout(rescanProgress, 0, false)
							return EndToEndRow(gtx, progressTitleLabel, blocksScannedLabel)
						})
					}),
				)
			})
		})
	})
}

// syncContent lays out sync status content when the wallet is syncing, synced, not connected
func (wsi *WalletSyncInfo) syncContent(gtx C, uniform layout.Inset) D {
	isBtcAsset := wsi.wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := wsi.wallet.GetAssetType() == libutils.LTCWalletAsset
	isSyncing := wsi.wallet.IsSyncing()
	isBtcORLtcAsset := isBtcAsset || isLtcAsset
	// Rescanning should happen on a synced chain.
	isRescanning := wsi.wallet.IsRescanning() && !isSyncing
	isInProgress := isSyncing || isRescanning
	bestBlock := wsi.wallet.GetBestBlock()
	dp8 := values.MarginPadding8
	return uniform.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(wsi.labelTexSize16Layout(values.String(values.StrLatestBlock), dp8, true)),
					layout.Rigid(func(gtx C) D {
						if !isInProgress {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrBlockHeaderFetched), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if isRescanning && (isBtcORLtcAsset) {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrSyncingProgress), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !isInProgress || (isRescanning && (isBtcORLtcAsset)) {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrSyncCompTime), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !(isRescanning && (isBtcORLtcAsset)) {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrAddressDiscoveryInProgress), dp8, true)(gtx)
					}),
				)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							latestBlockTitle := fmt.Sprintf("%d (%s)", bestBlock.Height, TimeAgo(bestBlock.Timestamp))
							return wsi.labelTexSize16Layout(latestBlockTitle, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInProgress || (isRescanning && (isBtcORLtcAsset)) {
								return D{}
							}
							blockHeightFetched := values.StringF(values.StrBlockHeaderFetchedCount, bestBlock.Height, wsi.FetchSyncProgress().HeadersToFetchOrScan)
							return wsi.labelTexSize16Layout(blockHeightFetched, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							currentSeconds := time.Now().Unix()
							w := wsi.wallet
							daysBehind := TimeFormat(int(currentSeconds-w.GetBestBlockTimeStamp()), true)

							syncProgress := values.String(values.StrWalletNotSynced)
							if wsi.wallet.IsSyncing() {
								syncProgress = values.StringF(values.StrSyncingProgressStat, daysBehind)
							} else if wsi.wallet.IsRescanning() {
								syncProgress = values.String(values.StrRescanningBlocks)
							} else if wsi.wallet.IsSynced() {
								syncProgress = values.String(values.StrComplete)
							}

							return wsi.labelTexSize16Layout(syncProgress, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInProgress || (isRescanning && (isBtcORLtcAsset)) {
								return D{}
							}
							_, timeLeft := wsi.progressStatusDetails()
							return wsi.labelTexSize16Layout(timeLeft, dp8, false)(gtx)
						}),
					)
				})
			}),
		)
	})
}

// syncBoxTitleRow lays out widgets in the title row inside the sync status box.
func (wsi *WalletSyncInfo) syncBoxTitleRow(gtx C) D {
	textSize14 := values.TextSize14
	statusLabel := wsi.Theme.Label(wsi.ConvertTextSize(textSize14), values.String(values.StrOffline))
	wsi.walletStatusIcon.Color = wsi.Theme.Color.Danger
	if wsi.wallet.IsConnectedToNetwork() {
		statusLabel.Text = values.String(values.StrOnline)
		wsi.walletStatusIcon.Color = wsi.Theme.Color.Success
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if wsi.IsMobileView() {
				return D{} // not enough space
			}

			return wsi.labelSize(textSize14, values.String(values.StrWalletStatus)).Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if wsi.wallet.IsSyncShuttingDown() {
				return layout.Inset{
					Left: values.MarginPadding4,
				}.Layout(gtx, wsi.labelSize(textSize14, values.String(values.StrCanceling)).Layout)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					inset := layout.Inset{Right: values.MarginPadding4}
					if !wsi.IsMobileView() {
						inset.Left = values.MarginPadding4
					}
					return inset.Layout(gtx, func(gtx C) D {
						return wsi.walletStatusIcon.Layout(gtx, values.MarginPadding15)
					})
				}),
				layout.Rigid(statusLabel.Layout),
				layout.Rigid(func(gtx C) D {
					if wsi.wallet.IsConnectedToNetwork() {
						connectedPeers := fmt.Sprintf("%d", wsi.wallet.ConnectedPeers())
						return wsi.labelSize(textSize14, values.StringF(values.StrConnectedTo, connectedPeers)).Layout(gtx)
					}

					if !wsi.isStatusConnected {
						return wsi.labelSize(textSize14, values.String(values.StrNoInternet)).Layout(gtx)
					}
					return wsi.labelSize(textSize14, values.String(values.StrNoConnectedPeer)).Layout(gtx)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, wsi.layoutAutoSyncSection)
		}),
	)
}

func (wsi *WalletSyncInfo) labelSize(size unit.Sp, txt string) cryptomaterial.Label {
	return wsi.Theme.Label(wsi.ConvertTextSize(size), txt)
}

func (wsi *WalletSyncInfo) labelTexSize16Layout(txt string, bottomInset unit.Dp, colorGrey bool) func(gtx C) D {
	return func(gtx C) D {
		lbl := wsi.Theme.Body1(txt)
		if colorGrey {
			lbl.Color = wsi.Theme.Color.GrayText2
		}
		lbl.TextSize = wsi.ConvertTextSize(values.TextSize16)
		return layout.Inset{Bottom: bottomInset}.Layout(gtx, lbl.Layout)
	}
}

// progressBarRow lays out the progress bar.
func (wsi *WalletSyncInfo) progressBarRow(gtx C) D {
	return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
		progress, _ := wsi.progressStatusDetails()

		p := wsi.Theme.ProgressBar(progress)
		p.Height = values.MarginPadding16
		p.Radius = cryptomaterial.Radius(4)
		p.Color = wsi.Theme.Color.Success
		p.TrackColor = wsi.Theme.Color.Gray2

		progressTitleLabel := wsi.labelSize(values.TextSize14, fmt.Sprintf("%v%%", progress))
		progressTitleLabel.Color = wsi.Theme.Color.Text
		return p.TextLayout(gtx, progressTitleLabel.Layout)
	})
}

// progressStatusRow lays out the progress status when the wallet is syncing.
func (wsi *WalletSyncInfo) progressStatusDetails() (int, string) {
	timeLeftLabel := ""
	pgrss := wsi.FetchSyncProgress()
	timeLeft := pgrss.remainingSyncTime
	progress := pgrss.syncProgress

	walletIsRescanning := wsi.wallet.IsRescanning()
	if walletIsRescanning && wsi.rescanUpdate != nil {
		progress = int(wsi.rescanUpdate.RescanProgress)
		timeLeft = TimeFormat(int(wsi.rescanUpdate.RescanTimeRemaining), true)
	}

	if wsi.wallet.IsSyncing() || walletIsRescanning {
		timeLeftLabel = values.StringF(values.StrTimeLeftFmt, timeLeft)
		if progress == 0 {
			timeLeftLabel = values.String(values.StrLoading)
		}
	}

	return progress, timeLeftLabel
}

func (wsi *WalletSyncInfo) layoutAutoSyncSection(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, wsi.syncSwitch.Layout)
		}),
		layout.Rigid(wsi.Theme.Body2(values.String(values.StrSync)).Layout),
	)
}

func (wsi *WalletSyncInfo) FetchSyncProgress() ProgressInfo {
	pgrss, ok := syncProgressInfo[wsi.wallet]
	if !ok {
		pgrss = ProgressInfo{}
	}
	// remove the unnecessary sync progress data if already synced.
	wsi.deleteSyncProgress()
	return pgrss
}

// deleteSyncProgress removes the map entry after the data persisted is no longer necessary.
func (wsi *WalletSyncInfo) deleteSyncProgress() {
	wal := wsi.wallet
	if wal.IsSynced() {
		delete(syncProgressInfo, wal)
	}
}

func (wsi *WalletSyncInfo) syncStatusIcon(gtx C) D {
	icon := wsi.Theme.Icons.SyncingIcon
	if wsi.wallet.IsSynced() {
		icon = wsi.Theme.Icons.SuccessIcon
	} else if wsi.wallet.IsSyncing() {
		icon = wsi.Theme.Icons.SyncingIcon
	}

	i := layout.Inset{Left: values.MarginPadding16}
	return i.Layout(gtx, func(gtx C) D {
		return icon.LayoutSize(gtx, wsi.ConvertIconSize(values.MarginPadding20))
	})
}

// ListenForNotifications starts a goroutine to watch for sync updates and
// update the UI accordingly. To prevent UI lags, this method does not refresh
// the window display every time a sync update is received. During active blocks
// sync, rescan or proposals sync, the Layout method auto refreshes the display
// every set interval. Other sync updates that affect the UI but occur outside
// of an active sync requires a display refresh. The caller of this method must
// ensure that the StopListeningForNotifications() method is called whenever the
// the page or modal using these notifications is closed.
func (wsi *WalletSyncInfo) ListenForNotifications() {
	updateSyncProgress := func(progress ProgressInfo) {
		// Update sync progress fields which will be displayed
		// when the next UI invalidation occurs.

		previousProgress := wsi.FetchSyncProgress()
		// headers to fetch cannot be less than the previously fetched.
		// Page refresh only needed if there is new data to update the UI.
		if progress.HeadersToFetchOrScan >= previousProgress.HeadersToFetchOrScan {
			// set the new progress against the associated asset.
			syncProgressInfo[wsi.wallet] = progress

			// We only care about sync state changes here, to
			// refresh the window display.
			wsi.reload()
		}
	}

	syncProgressListener := &sharedW.SyncProgressListener{
		OnHeadersFetchProgress: func(t *sharedW.HeadersFetchProgressReport) {
			progress := ProgressInfo{}
			progress.stepFetchProgress = t.HeadersFetchProgress
			progress.HeadersToFetchOrScan = t.TotalHeadersToFetch
			progress.syncProgress = int(t.TotalSyncProgress)
			progress.remainingSyncTime = TimeFormat(int(t.TotalTimeRemainingSeconds), true)
			updateSyncProgress(progress)
		},
		OnAddressDiscoveryProgress: func(t *sharedW.AddressDiscoveryProgressReport) {
			progress := ProgressInfo{}
			progress.syncProgress = int(t.TotalSyncProgress)
			progress.remainingSyncTime = TimeFormat(int(t.TotalTimeRemainingSeconds), true)
			progress.stepFetchProgress = t.AddressDiscoveryProgress
			updateSyncProgress(progress)
		},
		OnHeadersRescanProgress: func(t *sharedW.HeadersRescanProgressReport) {
			progress := ProgressInfo{}
			progress.HeadersToFetchOrScan = t.TotalHeadersToScan
			progress.syncProgress = int(t.TotalSyncProgress)
			progress.remainingSyncTime = TimeFormat(int(t.TotalTimeRemainingSeconds), true)
			progress.stepFetchProgress = t.RescanProgress
			updateSyncProgress(progress)
		},
		OnSyncCompleted: func() {
			wsi.reload()
		},
	}

	err := wsi.wallet.AddSyncProgressListener(syncProgressListener, WalletSyncInfoID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(walletID int, transaction *sharedW.Transaction) {
			wsi.reload()
		},
		OnBlockAttached: func(walletID int, blockHeight int32) {
			wsi.reload()
		},
	}
	err = wsi.wallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, WalletSyncInfoID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	blocksRescanProgressListener := &sharedW.BlocksRescanProgressListener{
		OnBlocksRescanStarted: func(walletID int) {
			wsi.rescanUpdate = nil
		},
		OnBlocksRescanProgress: func(progress *sharedW.HeadersRescanProgressReport) {
			wsi.rescanUpdate = progress
		},
		OnBlocksRescanEnded: func(walletID int, err error) {
			wsi.rescanUpdate = nil
			wsi.reload()
		},
	}
	wsi.wallet.SetBlocksRescanProgressListener(blocksRescanProgressListener)
}

// StopListeningForNotifications stops listening for sync progress, tx and block
// notifications.
func (wsi *WalletSyncInfo) StopListeningForNotifications() {
	wsi.wallet.RemoveSyncProgressListener(WalletSyncInfoID)
	wsi.wallet.RemoveTxAndBlockNotificationListener(WalletSyncInfoID)
	wsi.wallet.SetBlocksRescanProgressListener(nil)
}

func (wsi *WalletSyncInfo) handle() {
	// As long as the internet connection hasn't been established keep checking.
	if !wsi.isStatusConnected {
		go func() {
			wsi.isStatusConnected = libutils.IsOnline()
		}()
	}

	isSyncShutting := wsi.wallet.IsSyncShuttingDown()
	wsi.syncSwitch.SetEnabled(!isSyncShutting)
	if wsi.syncSwitch.Changed() {
		if wsi.wallet.IsRescanning() {
			wsi.wallet.CancelRescan()
		}

		go func() {
			wsi.ToggleSync(wsi.wallet, func(b bool) {
				wsi.syncSwitch.SetChecked(b)
				wsi.wallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
			})
		}()
	}

	if wsi.toBackup.Button.Clicked() {
		wsi.backup(wsi.wallet)
	}
}
