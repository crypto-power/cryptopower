package info

import (
	"fmt"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *WalletInfo) initWalletStatusWidgets() {
	pg.walletStatusIcon = cryptomaterial.NewIcon(pg.Theme.Icons.DotIcon)
	pg.syncSwitch = pg.Theme.Switch()
}

// syncStatusSection lays out content for displaying sync status.
func (pg *WalletInfo) syncStatusSection(gtx C) D {
	isBtcAsset := pg.wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := pg.wallet.GetAssetType() == libutils.LTCWalletAsset
	syncing := pg.wallet.IsSyncing()

	// btcwallet does not export implementation to track address discovery.
	// During btc address discovery, show the normal synced info page with an
	// extra label showing the address discovery is in progress.
	rescanning := pg.wallet.IsRescanning() && !isLtcAsset && !isBtcAsset && !syncing

	uniform := layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding5}
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		return components.Container{Padding: layout.Inset{
			Top:    values.MarginPadding15,
			Bottom: values.MarginPadding16,
		}}.Layout(gtx, func(gtx C) D {
			items := []layout.FlexChild{layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, pg.syncBoxTitleRow)
			})}

			if syncing || rescanning {
				items = append(items, layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(.93, pg.progressBarRow),
							layout.Rigid(pg.syncStatusIcon),
						)
					})
				}))
			}

			if rescanning {
				items = append(items, layout.Rigid(func(gtx C) D {
					return pg.rescanDetailsLayout(gtx, uniform)
				}))
			} else {
				items = append(items, layout.Rigid(func(gtx C) D {
					return pg.syncContent(gtx, uniform)
				}))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
	})
}

// syncBoxTitleRow lays out widgets in the title row inside the sync status box.
func (pg *WalletInfo) syncBoxTitleRow(gtx C) D {
	statusLabel := pg.textSize14Label(values.String(values.StrOffline))
	pg.walletStatusIcon.Color = pg.Theme.Color.Danger
	if pg.wallet.IsConnectedToNetwork() {
		statusLabel.Text = values.String(values.StrOnline)
		pg.walletStatusIcon.Color = pg.Theme.Color.Success
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if pg.IsMobileView() {
				return D{} // not enough space
			}
			return pg.textSize14Label(values.String(values.StrWalletStatus)).Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if pg.wallet.IsSyncShuttingDown() {
				return layout.Inset{
					Left: values.MarginPadding4,
				}.Layout(gtx, pg.textSize14Label(values.String(values.StrCanceling)).Layout)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					inset := layout.Inset{Right: values.MarginPadding4}
					if !pg.IsMobileView() {
						inset.Left = values.MarginPadding4
					}
					return inset.Layout(gtx, func(gtx C) D {
						return pg.walletStatusIcon.Layout(gtx, values.MarginPadding15)
					})
				}),
				layout.Rigid(statusLabel.Layout),
				layout.Rigid(func(gtx C) D {
					if pg.wallet.IsConnectedToNetwork() {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								connectedPeers := fmt.Sprintf("%d", pg.wallet.ConnectedPeers())
								return pg.textSize14Label(values.StringF(values.StrConnectedTo, connectedPeers)).Layout(gtx)
							}),
						)
					}

					if !pg.isStatusConnected {
						return pg.textSize14Label(values.String(values.StrNoInternet)).Layout(gtx)
					}
					return pg.textSize14Label(values.String(values.StrNoConnectedPeer)).Layout(gtx)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, pg.layoutAutoSyncSection)
		}),
	)
}

func (pg *WalletInfo) textSize14Label(txt string) cryptomaterial.Label {
	return pg.Theme.Label(pg.ConvertTextSize(values.TextSize14), txt)
}

func (pg *WalletInfo) syncStatusIcon(gtx C) D {
	icon := pg.Theme.Icons.SyncingIcon
	if pg.wallet.IsSynced() {
		icon = pg.Theme.Icons.SuccessIcon
	} else if pg.wallet.IsSyncing() {
		icon = pg.Theme.Icons.SyncingIcon
	}

	i := layout.Inset{Left: values.MarginPadding16}
	return i.Layout(gtx, func(gtx C) D {
		return icon.LayoutSize(gtx, pg.ConvertIconSize(values.MarginPadding20))
	})
}

// syncContent lays out sync status content when the wallet is syncing, synced, not connected
func (pg *WalletInfo) syncContent(gtx C, uniform layout.Inset) D {
	isBtcAsset := pg.wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := pg.wallet.GetAssetType() == libutils.LTCWalletAsset
	isSyncing := pg.wallet.IsSyncing()
	isBtcORLtcAsset := isBtcAsset || isLtcAsset
	// Rescanning should happen on a synced chain.
	isRescanning := pg.wallet.IsRescanning() && !isSyncing
	isInProgress := isSyncing || isRescanning
	bestBlock := pg.wallet.GetBestBlock()
	dp8 := values.MarginPadding8
	return uniform.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(pg.labelTexSize16Layout(values.String(values.StrLatestBlock), dp8, true)),
					layout.Rigid(func(gtx C) D {
						if !isInProgress {
							return D{}
						}
						return pg.labelTexSize16Layout(values.String(values.StrBlockHeaderFetched), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if isRescanning && (isBtcORLtcAsset) {
							return D{}
						}
						return pg.labelTexSize16Layout(values.String(values.StrSyncingProgress), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !isInProgress || (isRescanning && (isBtcORLtcAsset)) {
							return D{}
						}
						return pg.labelTexSize16Layout(values.String(values.StrSyncCompTime), dp8, true)(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !(isRescanning && (isBtcORLtcAsset)) {
							return D{}
						}
						return pg.labelTexSize16Layout(values.String(values.StrAddressDiscoveryInProgress), dp8, true)(gtx)
					}),
				)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							latestBlockTitle := fmt.Sprintf("%d (%s)", bestBlock.Height, components.TimeAgo(bestBlock.Timestamp))
							return pg.labelTexSize16Layout(latestBlockTitle, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInProgress || (isRescanning && (isBtcORLtcAsset)) {
								return D{}
							}
							blockHeightFetched := values.StringF(values.StrBlockHeaderFetchedCount, bestBlock.Height, pg.FetchSyncProgress().HeadersToFetchOrScan)
							return pg.labelTexSize16Layout(blockHeightFetched, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							currentSeconds := time.Now().Unix()
							w := pg.wallet
							daysBehind := components.TimeFormat(int(currentSeconds-w.GetBestBlockTimeStamp()), true)

							syncProgress := values.String(values.StrWalletNotSynced)
							if pg.wallet.IsSyncing() {
								syncProgress = values.StringF(values.StrSyncingProgressStat, daysBehind)
							} else if pg.wallet.IsRescanning() {
								syncProgress = values.String(values.StrRescanningBlocks)
							} else if pg.wallet.IsSynced() {
								syncProgress = values.String(values.StrComplete)
							}

							return pg.labelTexSize16Layout(syncProgress, dp8, false)(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInProgress || (isRescanning && (isBtcORLtcAsset)) {
								return D{}
							}
							_, timeLeft := pg.progressStatusDetails()
							return pg.labelTexSize16Layout(timeLeft, dp8, false)(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (pg *WalletInfo) labelTexSize16Layout(txt string, bottomInset unit.Dp, colorGrey bool) func(gtx C) D {
	return func(gtx C) D {
		lbl := pg.Theme.Body1(txt)
		if colorGrey {
			lbl.Color = pg.Theme.Color.GrayText2
		}
		lbl.TextSize = pg.ConvertTextSize(values.TextSize16)
		return layout.Inset{Bottom: bottomInset}.Layout(gtx, lbl.Layout)
	}
}

func (pg *WalletInfo) layoutAutoSyncSection(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.syncSwitch.Layout)
		}),
		layout.Rigid(pg.Theme.Body2(values.String(values.StrSync)).Layout),
	)
}

// progressBarRow lays out the progress bar.
func (pg *WalletInfo) progressBarRow(gtx C) D {
	return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
		progress, _ := pg.progressStatusDetails()

		p := pg.Theme.ProgressBar(progress)
		p.Height = values.MarginPadding16
		p.Radius = cryptomaterial.Radius(4)
		p.Color = pg.Theme.Color.Success
		p.TrackColor = pg.Theme.Color.Gray2

		progressTitleLabel := pg.textSize14Label(fmt.Sprintf("%v%%", progress))
		progressTitleLabel.Color = pg.Theme.Color.Text
		return p.TextLayout(gtx, progressTitleLabel.Layout)
	})
}

// progressStatusRow lays out the progress status when the wallet is syncing.
func (pg *WalletInfo) progressStatusDetails() (int, string) {
	timeLeftLabel := ""
	pgrss := pg.FetchSyncProgress()
	timeLeft := pgrss.remainingSyncTime
	progress := pgrss.syncProgress

	walletIsRescanning := pg.wallet.IsRescanning()
	if walletIsRescanning && pg.rescanUpdate != nil {
		progress = int(pg.rescanUpdate.RescanProgress)
		timeLeft = components.TimeFormat(int(pg.rescanUpdate.RescanTimeRemaining), true)
	}

	if pg.wallet.IsSyncing() || walletIsRescanning {
		timeLeftLabel = values.StringF(values.StrTimeLeftFmt, timeLeft)
		if progress == 0 {
			timeLeftLabel = values.String(values.StrLoading)
		}
	}

	return progress, timeLeftLabel
}

func (pg *WalletInfo) rescanDetailsLayout(gtx C, inset layout.Inset) D {
	if !pg.wallet.IsRescanning() || pg.rescanUpdate == nil {
		return D{}
	}
	return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		card := pg.Theme.Card()
		card.Color = pg.Theme.Color.Gray4
		return card.Layout(gtx, func(gtx C) D {
			return components.Container{Padding: layout.UniformInset(values.MarginPadding16)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							return pg.Theme.Body1(pg.wallet.GetWalletName()).Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							headersFetchedTitleLabel := pg.labelTexSize16Layout(values.String(values.StrBlocksScanned), 0, true)
							blocksScannedLabel := pg.labelTexSize16Layout(fmt.Sprint(pg.rescanUpdate.CurrentRescanHeight), 0, false)
							return components.EndToEndRow(gtx, headersFetchedTitleLabel, blocksScannedLabel)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return inset.Layout(gtx, func(gtx C) D {
							progressTitleLabel := pg.labelTexSize16Layout(values.String(values.StrSyncingProgress), 0, true)
							rescanProgress := values.StringF(values.StrBlocksLeft, pg.rescanUpdate.TotalHeadersToScan-pg.rescanUpdate.CurrentRescanHeight)
							blocksScannedLabel := pg.labelTexSize16Layout(rescanProgress, 0, false)
							return components.EndToEndRow(gtx, progressTitleLabel, blocksScannedLabel)
						})
					}),
				)
			})
		})
	})
}

func (pg *WalletInfo) FetchSyncProgress() progressInfo {
	pgrss, ok := syncProgressInfo[pg.wallet]
	if !ok {
		pgrss = progressInfo{}
	}
	// remove the unnecessary sync progress data if already synced.
	pg.deleteSyncProgress()
	return pgrss
}

// deleteSyncProgress removes the map entry after the data persisted is no longer necessary.
func (pg *WalletInfo) deleteSyncProgress() {
	wal := pg.wallet
	if wal.IsSynced() {
		delete(syncProgressInfo, wal)
	}
}
