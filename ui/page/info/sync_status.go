package info

import (
	"fmt"
	"time"

	"gioui.org/layout"

	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *WalletInfo) initWalletStatusWidgets() {
	pg.walletStatusIcon = cryptomaterial.NewIcon(pg.Theme.Icons.ImageBrightness1)
	pg.syncSwitch = pg.Theme.Switch()
}

// syncStatusSection lays out content for displaying sync status.
func (pg *WalletInfo) syncStatusSection(gtx C) D {
	isBtcAsset := pg.WL.SelectedWallet.Wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := pg.WL.SelectedWallet.Wallet.GetAssetType() == libutils.LTCWalletAsset
	syncing := pg.WL.SelectedWallet.Wallet.IsSyncing()

	// btcwallet does not export implementation to track address discovery.
	// During btc address discovery, show the normal synced info page with an
	// extra label showing the address discovery is in progress.
	rescanning := pg.WL.SelectedWallet.Wallet.IsRescanning() && !isLtcAsset && !isBtcAsset && !syncing

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
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(.9, pg.progressBarRow),
							layout.Flexed(.1, pg.syncStatusIcon),
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
	statusLabel := pg.Theme.Label(values.TextSize14, values.String(values.StrOffline))
	pg.walletStatusIcon.Color = pg.Theme.Color.Danger
	if pg.WL.SelectedWallet.Wallet.IsConnectedToNetwork() {
		statusLabel.Text = values.String(values.StrOnline)
		pg.walletStatusIcon.Color = pg.Theme.Color.Success
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(pg.Theme.Label(values.TextSize14, values.String(values.StrWalletStatus)).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if pg.WL.SelectedWallet.Wallet.IsSyncShuttingDown() {
				return layout.Inset{
					Left: values.MarginPadding4,
				}.Layout(gtx, pg.Theme.Label(values.TextSize14, values.String(values.StrCanceling)).Layout)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Right: values.MarginPadding4,
						Left:  values.MarginPadding4,
					}.Layout(gtx, func(gtx C) D {
						return pg.walletStatusIcon.Layout(gtx, values.MarginPadding15)
					})
				}),
				layout.Rigid(statusLabel.Layout),
				layout.Rigid(func(gtx C) D {
					if pg.WL.SelectedWallet.Wallet.IsConnectedToNetwork() {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								connectedPeers := fmt.Sprintf("%d", pg.WL.SelectedWallet.Wallet.ConnectedPeers())
								return pg.Theme.Label(values.TextSize14, values.StringF(values.StrConnectedTo, connectedPeers)).Layout(gtx)
							}),
						)
					}

					if !pg.isStatusConnected {
						return pg.Theme.Label(values.TextSize14, values.String(values.StrNoInternet)).Layout(gtx)
					}
					return pg.Theme.Label(values.TextSize14, values.String(values.StrNoConnectedPeer)).Layout(gtx)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, pg.layoutAutoSyncSection)
		}),
	)
}

func (pg *WalletInfo) syncStatusIcon(gtx C) D {
	icon := pg.Theme.Icons.SyncingIcon
	if pg.WL.SelectedWallet.Wallet.IsSynced() {
		icon = pg.Theme.Icons.SuccessIcon
	} else if pg.WL.SelectedWallet.Wallet.IsSyncing() {
		icon = pg.Theme.Icons.SyncingIcon
	}

	i := layout.Inset{Left: values.MarginPadding16}
	return i.Layout(gtx, func(gtx C) D {
		return icon.LayoutSize(gtx, values.MarginPadding20)
	})
}

// syncContent lays out sync status content when the wallet is syncing, synced, not connected
func (pg *WalletInfo) syncContent(gtx C, uniform layout.Inset) D {
	isBtcAsset := pg.WL.SelectedWallet.Wallet.GetAssetType() == libutils.BTCWalletAsset
	isLtcAsset := pg.WL.SelectedWallet.Wallet.GetAssetType() == libutils.LTCWalletAsset
	isSyncing := pg.WL.SelectedWallet.Wallet.IsSyncing()
	isBtcORLtcAsset := isBtcAsset || isLtcAsset
	// Rescanning should happen on a synced chain.
	isRescanning := pg.WL.SelectedWallet.Wallet.IsRescanning() && !isSyncing
	isInprogress := isSyncing || isRescanning
	bestBlock := pg.WL.SelectedWallet.Wallet.GetBestBlock()
	return uniform.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						latestBlockTitle := pg.Theme.Body1(values.String(values.StrLatestBlock))
						latestBlockTitle.Color = pg.Theme.Color.GrayText2
						return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, latestBlockTitle.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						if !isInprogress {
							return D{}
						}
						blockHeaderFetched := pg.Theme.Body1(values.String(values.StrBlockHeaderFetched))
						blockHeaderFetched.Color = pg.Theme.Color.GrayText2
						return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, blockHeaderFetched.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						if isRescanning && (isBtcORLtcAsset) {
							return D{}
						}
						syncProgress := pg.Theme.Body1(values.String(values.StrSyncingProgress))
						syncProgress.Color = pg.Theme.Color.GrayText2
						return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, syncProgress.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						if !isInprogress || (isRescanning && (isBtcORLtcAsset)) {
							return D{}
						}
						estTime := pg.Theme.Body1(values.String(values.StrSyncCompTime))
						estTime.Color = pg.Theme.Color.GrayText2
						return estTime.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !(isRescanning && (isBtcORLtcAsset)) {
							return D{}
						}
						addrDiscovery := pg.Theme.Body1(values.String(values.StrAddressDiscoveryInProgress))
						addrDiscovery.Color = pg.Theme.Color.GrayText2
						return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, addrDiscovery.Layout)
					}),
				)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							latestBlockTitle := pg.Theme.Body1(fmt.Sprintf("%d (%s)", bestBlock.Height, components.TimeAgo(bestBlock.Timestamp)))
							return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, latestBlockTitle.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInprogress || (isRescanning && (isBtcORLtcAsset)) {
								return D{}
							}
							pgrss := pg.fetchSyncProgress()
							blockHeightFetchedText := values.StringF(values.StrBlockHeaderFetchedCount, bestBlock.Height,
								pgrss.headersToFetchOrScan)
							blockHeightFetched := pg.Theme.Body1(blockHeightFetchedText)
							return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, blockHeightFetched.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							currentSeconds := time.Now().Unix()
							w := pg.WL.SelectedWallet.Wallet
							daysBehind := components.TimeFormat(int(currentSeconds-w.GetBestBlockTimeStamp()), true)

							syncProgress := values.String(values.StrWalletNotSynced)
							if pg.WL.SelectedWallet.Wallet.IsSyncing() {
								syncProgress = values.StringF(values.StrSyncingProgressStat, daysBehind)
							} else if pg.WL.SelectedWallet.Wallet.IsRescanning() {
								syncProgress = values.String(values.StrRescanningBlocks)
							} else if pg.WL.SelectedWallet.Wallet.IsSynced() {
								syncProgress = values.String(values.StrComplete)
							}

							syncProgressBody := pg.Theme.Body1(syncProgress)
							return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, syncProgressBody.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							if !isInprogress || (isRescanning && (isBtcORLtcAsset)) {
								return D{}
							}
							_, timeLeft := pg.progressStatusDetails()
							estTime := pg.Theme.Body1(timeLeft)
							return estTime.Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (pg *WalletInfo) layoutAutoSyncSection(gtx C) D {
	return layout.Inset{Right: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.syncSwitch.Layout)
			}),
			layout.Rigid(pg.Theme.Body2(values.String(values.StrSync)).Layout),
		)
	})
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

		progressTitleLabel := pg.Theme.Label(values.TextSize14, fmt.Sprintf("%v%%", progress))
		progressTitleLabel.Color = pg.Theme.Color.Text
		return p.TextLayout(gtx, progressTitleLabel.Layout)
	})
}

// progressStatusRow lays out the progress status when the wallet is syncing.
func (pg *WalletInfo) progressStatusDetails() (int, string) {
	timeLeftLabel := ""
	pgrss := pg.fetchSyncProgress()
	timeLeft := pgrss.remainingSyncTime
	progress := pgrss.syncProgress

	wallet := pg.WL.SelectedWallet.Wallet
	walletIsRescanning := wallet.IsRescanning()
	if walletIsRescanning && pg.rescanUpdate != nil {
		progress = int(pg.rescanUpdate.RescanProgress)
		timeLeft = components.TimeFormat(int(pg.rescanUpdate.RescanTimeRemaining), true)
	}

	if wallet.IsSyncing() || walletIsRescanning {
		timeLeftLabel = values.StringF(values.StrTimeLeft, timeLeft)
		if progress == 0 {
			timeLeftLabel = values.String(values.StrLoading)
		}
	}

	return progress, timeLeftLabel
}

func (pg *WalletInfo) rescanDetailsLayout(gtx C, inset layout.Inset) D {
	wal := pg.WL.SelectedWallet.Wallet
	if !wal.IsRescanning() || pg.rescanUpdate == nil {
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
							return pg.Theme.Body1(wal.GetWalletName()).Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						headersFetchedTitleLabel := pg.Theme.Body2(values.String(values.StrBlocksScanned))
						headersFetchedTitleLabel.Color = pg.Theme.Color.GrayText2

						blocksScannedLabel := pg.Theme.Body1(fmt.Sprint(pg.rescanUpdate.CurrentRescanHeight))
						return inset.Layout(gtx, func(gtx C) D {
							return components.EndToEndRow(gtx, headersFetchedTitleLabel.Layout, blocksScannedLabel.Layout)
						})
					}),
					layout.Rigid(func(gtx C) D {
						progressTitleLabel := pg.Theme.Body2(values.String(values.StrSyncingProgress))
						progressTitleLabel.Color = pg.Theme.Color.GrayText2

						rescanProgress := values.StringF(values.StrBlocksLeft, pg.rescanUpdate.TotalHeadersToScan-pg.rescanUpdate.CurrentRescanHeight)
						blocksScannedLabel := pg.Theme.Body1(rescanProgress)
						return inset.Layout(gtx, func(gtx C) D {
							return components.EndToEndRow(gtx, progressTitleLabel.Layout, blocksScannedLabel.Layout)
						})
					}),
				)
			})
		})
	})
}

func (pg *WalletInfo) fetchSyncProgress() progressInfo {
	pgrss, ok := syncProgressInfo[pg.WL.SelectedWallet.Wallet]
	if !ok {
		pgrss = progressInfo{}
	}
	// remove the unnecessary sync progress data if already synced.
	pg.deleteSyncProgress()
	return pgrss
}

// deleteSyncProgress removes the map entry after the data persisted is no longer necessary.
func (pg *WalletInfo) deleteSyncProgress() {
	wal := pg.WL.SelectedWallet.Wallet
	if wal.IsSynced() {
		delete(syncProgressInfo, wal)
	}
}
