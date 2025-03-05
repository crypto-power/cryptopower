package components

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	pageutils "github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletSyncInfoID = "WalletSyncInfo"

type WalletSyncInfo struct {
	*load.Load
	wallet           sharedW.Asset
	walletStatusIcon *cryptomaterial.Icon
	syncSwitch       *cryptomaterial.Switch
	ForwardButton    cryptomaterial.IconButton

	backup   func(sharedW.Asset)
	toBackup widget.Clickable

	// Sync data fields that needs mutex protection.
	isStatusConnected bool
	reload            func()
	isSlider          bool
	statusMu          sync.RWMutex

	switchEnabled atomic.Bool
}

// SyncInfo is made independent of the WalletSyncInfo struct so that once
// set with a value, it persists till unset or the app is killed. This
// will help address the progress bar issue where, changing UI pages alters the
// progress on the sync status progress percentage.
var syncProgressInfo *pageutils.SyncInfo

func NewWalletSyncInfo(l *load.Load, wallet sharedW.Asset, reload func(), backup func(sharedW.Asset)) *WalletSyncInfo {
	wsi := &WalletSyncInfo{
		Load:             l,
		wallet:           wallet,
		reload:           reload,
		walletStatusIcon: cryptomaterial.NewIcon(l.Theme.Icons.DotIcon),
		syncSwitch:       l.Theme.Switch(),
		backup:           backup,
	}

	wsi.ForwardButton, _ = SubpageHeaderButtons(l)
	wsi.ForwardButton.Icon = wsi.Theme.Icons.NavigationArrowForward
	wsi.ForwardButton.Size = values.MarginPadding20

	// Initialize sync progress info if an active instance did not exist.
	if syncProgressInfo == nil {
		syncProgressInfo = pageutils.NewSyncProgressInfo()
	}
	return wsi
}

func (wsi *WalletSyncInfo) Init() {
	autoSync := wsi.wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false)
	wsi.syncSwitch.SetChecked(autoSync)
	go wsi.CheckConnectivity()
}

// safeIsStatusConnected adds read mutex protection to isStatusConnected check.
func (wsi *WalletSyncInfo) safeIsStatusConnected() bool {
	defer wsi.statusMu.RUnlock()
	wsi.statusMu.RLock()
	return wsi.isStatusConnected
}

// IsSliderOn adds read mutex protection to isSlider check. If true a progress
// bar is displayed on the UI
func (wsi *WalletSyncInfo) IsSliderOn() bool {
	defer wsi.statusMu.RUnlock()
	wsi.statusMu.RLock()
	return wsi.isSlider
}

// SetSliderOn safely sets the progress bar to be displayed.
func (wsi *WalletSyncInfo) SetSliderOn() {
	wsi.statusMu.Lock()
	wsi.isSlider = true
	wsi.statusMu.Unlock()
}

func (wsi *WalletSyncInfo) GetWallet() sharedW.Asset {
	return wsi.wallet
}

func (wsi *WalletSyncInfo) WalletInfoLayout(gtx C) D {
	return wsi.pageContentWrapper(gtx, "", nil, func(gtx C) D {
		items := []layout.FlexChild{
			layout.Rigid(wsi.walletNameAndBackupInfo),
			layout.Rigid(wsi.syncStatusSection),
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
	if wsi.IsSliderOn() {
		items = append(items, layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding10,
			}.Layout(gtx, func(gtx C) D {
				icon := wsi.Theme.AssetIcon(wsi.wallet.GetAssetType())
				if wsi.wallet.IsWatchingOnlyWallet() {
					icon = wsi.Theme.WatchOnlyAssetIcon(wsi.wallet.GetAssetType())
				}
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

	if !wsi.wallet.IsWalletBackedUp() {
		items = append(items, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(wsi.Theme.Icons.RedAlert.Layout20dp),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left:  values.MarginPadding9,
						Right: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return material.Clickable(gtx, &wsi.toBackup, func(gtx C) D {
							return cryptomaterial.LinearLayout{
								Width:       cryptomaterial.WrapContent,
								Height:      cryptomaterial.WrapContent,
								Orientation: layout.Vertical,
								Padding:     layout.UniformInset(values.MarginPadding5),
								Background:  wsi.Theme.Color.Danger,
								Border: cryptomaterial.Border{
									Radius: cryptomaterial.CornerRadius{
										TopLeft:     int(values.MarginPadding8),
										TopRight:    int(values.MarginPadding8),
										BottomRight: int(values.MarginPadding8),
										BottomLeft:  int(values.MarginPadding8),
									},
								},
							}.Layout2(gtx, func(gtx C) D {
								return layout.Center.Layout(gtx, func(gtx C) D {
									lbl := material.Body2(wsi.Theme.Base, values.String(values.StrBackupWarning))
									lbl.Color = wsi.Theme.Color.White
									return lbl.Layout(gtx)
								})
							})
						})
					})
				}),
			)

		}))
	}

	if wsi.IsSliderOn() {
		items = append(items, layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, wsi.ForwardButton.Layout)
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, items...)
}

// syncStatusSection lays out content for displaying sync status.
func (wsi *WalletSyncInfo) syncStatusSection(gtx C) D {
	syncing := wsi.wallet.IsSyncing()

	// btcwallet and ltcWallet do not export implementation to track address discovery.
	// During btc & ltc address discovery, show the normal synced info page with an
	// extra label showing the address discovery is in progress.
	rescanning := wsi.wallet.IsRescanning() && !wsi.isBtcOrLtcAsset() && !syncing

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
	rescanUpdate := wsi.FetchRescanUpdate()
	if rescanUpdate == nil {
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
							blocksScannedLabel := wsi.labelTexSize16Layout(fmt.Sprint(rescanUpdate.CurrentRescanHeight), 0, false)
							return EndToEndRow(gtx, headersFetchedTitleLabel, blocksScannedLabel)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							progressTitleLabel := wsi.labelTexSize16Layout(values.String(values.StrSyncingProgress), 0, true)
							rescanProgress := values.StringF(values.StrBlocksLeft, rescanUpdate.TotalHeadersToScan-rescanUpdate.CurrentRescanHeight)
							blocksScannedLabel := wsi.labelTexSize16Layout(rescanProgress, 0, false)
							return EndToEndRow(gtx, progressTitleLabel, blocksScannedLabel)
						})
					}),
				)
			})
		})
	})
}

// isBtcOrLtcAsset returns true if the current wallet is of asset type BTC or LTC.
func (wsi *WalletSyncInfo) isBtcOrLtcAsset() bool {
	isBtcAsset := wsi.wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := wsi.wallet.GetAssetType() == libutils.LTCWalletAsset
	return isBtcAsset || isLtcAsset
}

// syncContent lays out sync status content when the wallet is syncing, synced, not connected
func (wsi *WalletSyncInfo) syncContent(gtx C, uniform layout.Inset) D {
	isSyncing := wsi.wallet.IsSyncing()

	// Rescanning should happen on a synced chain.
	isRescanning := wsi.wallet.IsRescanning() && !isSyncing
	isInProgress := isSyncing || isRescanning
	bestBlock := wsi.wallet.GetBestBlock()
	isAddDiscovering := false
	syncIsScanning := false
	if !wsi.isBtcOrLtcAsset() {
		isAddDiscovering = wsi.wallet.(*dcr.Asset).IsAddressDiscovering()
		syncIsScanning = wsi.wallet.(*dcr.Asset).IsSycnRescanning()
	}
	dp8 := values.MarginPadding8

	currentSeconds := time.Now().Unix()
	w := wsi.wallet
	daysBehind := pageutils.TimeFormat(int(currentSeconds-w.GetBestBlockTimeStamp()), true)

	totalStep := 2
	if !wsi.isBtcOrLtcAsset() {
		totalStep = 3
	}
	syncStep := 1
	syncProgressState := values.String(values.StrFetchingBlockHeaders)
	syncProgress := values.String(values.StrWalletNotSynced)
	if wsi.wallet.IsSyncing() {
		if !wsi.isBtcOrLtcAsset() {
			if isAddDiscovering {
				syncStep = 2
				syncProgressState = values.String(values.StrAddressDiscovering)
			} else if syncIsScanning {
				syncStep = 3
				syncProgressState = values.String(values.StrRescanningBlocks)
			}
		}
		syncProgress = values.StringF(values.StrSyncingProgressStat, daysBehind)
	} else if wsi.wallet.IsRescanning() {
		syncStep = 2
		syncProgress = values.String(values.StrRescanningBlocks)
		syncProgressState = values.String(values.StrRescanningBlocks)
	} else if wsi.wallet.IsSynced() {
		syncProgress = values.String(values.StrComplete)
		syncProgressState = values.String(values.StrComplete)
	}

	return uniform.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if !isInProgress {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.StringF(values.StrSyncSteps, syncStep, totalStep), dp8, false)(gtx)
					}),
					layout.Rigid(wsi.labelTexSize16Layout(values.String(values.StrLatestBlock), dp8, true)),
					layout.Rigid(func(gtx C) D {
						if !isInProgress {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrBlockHeaderFetched), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if isRescanning && wsi.isBtcOrLtcAsset() {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrSyncingProgress), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !isInProgress || (isRescanning && wsi.isBtcOrLtcAsset()) {
							return D{}
						}
						return wsi.labelTexSize16Layout(values.String(values.StrSyncCompTime), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !(isRescanning && wsi.isBtcOrLtcAsset()) {
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
							if !isInProgress {
								return D{}
							}
							return wsi.labelTexSize16Layout(syncProgressState, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							latestBlockTitle := fmt.Sprintf("%d (%s)", bestBlock.Height, pageutils.TimeAgo(bestBlock.Timestamp))
							return wsi.labelTexSize16Layout(latestBlockTitle, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInProgress || (isRescanning && wsi.isBtcOrLtcAsset()) {
								return D{}
							}
							header := wsi.FetchSyncProgress().HeadersToFetchOrScan()
							// When progress's state is rescan header is a header of rescan and not fetch
							// this is a workaround display block for user
							if header < bestBlock.Height {
								header = bestBlock.Height
							}
							blockHeightFetched := values.StringF(values.StrBlockHeaderFetchedCount, bestBlock.Height, header)
							return wsi.labelTexSize16Layout(blockHeightFetched, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return wsi.labelTexSize16Layout(syncProgress, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInProgress || (isRescanning && wsi.isBtcOrLtcAsset()) {
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

					if !wsi.safeIsStatusConnected() {
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
func (wsi *WalletSyncInfo) progressStatusDetails() (progress int, timeLeft string) {
	if wsi.wallet.IsRescanning() {
		sp := wsi.FetchRescanUpdate()
		if sp == nil {
			return
		}
		progress = int(sp.RescanProgress)
		timeLeft = sp.RescanTimeRemaining.String()
	} else {
		sp := wsi.FetchSyncProgress()
		progress = sp.SyncProgress()
		timeLeft = sp.RemainingSyncTime()
	}
	if wsi.wallet.IsSyncing() || wsi.wallet.IsRescanning() {
		timeLeft = values.StringF(values.StrTimeLeftFmt, timeLeft)
		if progress == 0 {
			timeLeft = values.String(values.StrLoading)
		}
	}
	return
}

func (wsi *WalletSyncInfo) layoutAutoSyncSection(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			wsi.syncSwitch.SetChecked(wsi.wallet.IsSyncing() || wsi.wallet.IsSynced())
			return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, wsi.syncSwitch.Layout)
		}),
		layout.Rigid(wsi.Theme.Body2(values.String(values.StrSync)).Layout),
	)
}

// FetchSyncProgress the sync progress of associated with the current wallet type.
// Once synced, progress is no longer persisted.
func (wsi *WalletSyncInfo) FetchSyncProgress() pageutils.ProgressInfo {
	pgrss := syncProgressInfo.GetSyncProgress(wsi.wallet)

	// remove the unnecessary sync progress data if already synced.
	if wsi.wallet.IsSynced() {
		syncProgressInfo.DeleteSyncProgress(wsi.wallet)
	}
	return pgrss
}

// FetchRescanUpdate returns the rescan update if the wallet is rescanning and
// an update exists. If rescanning isn't running, clear the rescan data for the
// current asset type
func (wsi *WalletSyncInfo) FetchRescanUpdate() *sharedW.HeadersRescanProgressReport {
	walletIsRescanning := wsi.wallet.IsRescanning()
	isRescanUpdateAvailable := syncProgressInfo.IsRescanProgressSet(wsi.wallet)

	if walletIsRescanning && isRescanUpdateAvailable {
		return syncProgressInfo.GetRescanProgress(wsi.wallet)
	}

	if !walletIsRescanning {
		syncProgressInfo.DeleteRescanProgress(wsi.wallet)
	}
	return nil
}

func (wsi *WalletSyncInfo) syncStatusIcon(gtx C) D {
	icon := wsi.Theme.Icons.SyncingIcon
	if wsi.wallet.IsRescanning() || wsi.wallet.IsSyncing() {
		icon = wsi.Theme.Icons.SyncingIcon
	} else if wsi.wallet.IsSynced() {
		icon = wsi.Theme.Icons.SuccessIcon
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
	updateSyncProgress := func(timeRemaining time.Duration, headersFetched int32,
		stepFetchProgress int32, totalSyncProgress int32) {
		// Update sync progress fields which will be displayed
		// when the next UI invalidation occurs.

		previousProgress := wsi.FetchSyncProgress()

		// headers to fetch cannot be less than the previously fetched.
		// Page refresh only needed if there is new data to update the UI.
		if headersFetched >= previousProgress.HeadersToFetchOrScan() {
			// set the new progress against the associated asset.
			syncProgressInfo.SetSyncProgress(wsi.wallet, timeRemaining, headersFetched,
				stepFetchProgress, totalSyncProgress)

			// After new sync state changes, refresh the display.
			wsi.reload()
		}
	}

	syncProgressListener := &sharedW.SyncProgressListener{
		OnHeadersFetchProgress: func(t *sharedW.HeadersFetchProgressReport) {
			updateSyncProgress(t.TotalTimeRemaining, t.TotalHeadersToFetch,
				t.HeadersFetchProgress, t.TotalSyncProgress)
		},
		OnAddressDiscoveryProgress: func(t *sharedW.AddressDiscoveryProgressReport) {
			updateSyncProgress(t.TotalTimeRemaining, t.AddressDiscoveryProgress,
				t.AddressDiscoveryProgress, t.TotalSyncProgress)
		},
		OnHeadersRescanProgress: func(t *sharedW.HeadersRescanProgressReport) {
			updateSyncProgress(t.TotalTimeRemaining, t.TotalHeadersToScan,
				t.RescanProgress, t.TotalSyncProgress)
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
		OnTransaction: func(_ int, _ *sharedW.Transaction) {
			wsi.reload()
		},
		OnBlockAttached: func(_ int, _ int32) {
			wsi.reload()
		},
	}
	err = wsi.wallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, WalletSyncInfoID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	blocksRescanProgressListener := &sharedW.BlocksRescanProgressListener{
		OnBlocksRescanStarted: func(_ int) {},
		OnBlocksRescanProgress: func(progress *sharedW.HeadersRescanProgressReport) {
			syncProgressInfo.SetRescanProgress(wsi.wallet, progress)
			wsi.reload()
		},
		OnBlocksRescanEnded: func(_ int, _ error) {
			syncProgressInfo.DeleteRescanProgress(wsi.wallet)
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

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
func (wsi *WalletSyncInfo) HandleUserInteractions(gtx C) {
	// As long as the internet connection hasn't been established keep checking.
	if !wsi.safeIsStatusConnected() {
		go wsi.CheckConnectivity()
	}

	isSyncShutting := wsi.wallet.IsSyncShuttingDown()
	wsi.syncSwitch.SetEnabled(!isSyncShutting)
	if wsi.syncSwitch.Changed(gtx) {
		if wsi.wallet.IsRescanning() {
			wsi.wallet.CancelRescan()
		}

		// Toggling switch states is handled in the layout() method.
		go func() {
			wsi.ToggleSync(wsi.wallet, func(b bool) {
				wsi.wallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
				wsi.reload()
			})
		}()
	}

	// Manage the sync toggle switch during the sync shutdown process.
	isSyncShuttingDown := wsi.wallet.IsSyncShuttingDown()
	if isSyncShuttingDown {
		wsi.switchEnabled.Store(true)
		wsi.syncSwitch.SetEnabled(false)
		wsi.reload()
	} else if !isSyncShuttingDown && wsi.switchEnabled.CompareAndSwap(true, false) {
		wsi.syncSwitch.SetEnabled(true)
		wsi.reload()
	}

	if wsi.toBackup.Clicked(gtx) {
		wsi.backup(wsi.wallet)
	}
}

// CheckConnectivity checks for internet connectivity.
func (wsi *WalletSyncInfo) CheckConnectivity() {
	status := libutils.IsOnline()
	if status {
		wsi.statusMu.Lock()
		wsi.isStatusConnected = status
		wsi.statusMu.Unlock()
	}
}
