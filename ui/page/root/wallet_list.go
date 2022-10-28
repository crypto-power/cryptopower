package root

import (
	"gioui.org/layout"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/listeners"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"code.cryptopower.dev/group/cryptopower/wallet"
)

func (pg *WalletDexServerSelector) initWalletSelectorOptions() {
	pg.dcrComponents = pg.Theme.NewClickableList(layout.Vertical)
	pg.btcComponents = pg.Theme.NewClickableList(layout.Vertical)
	pg.dcrWatchOnlyComponents = pg.Theme.NewClickableList(layout.Vertical)
	pg.btcWatchOnlyComponents = pg.Theme.NewClickableList(layout.Vertical)
}

func (pg *WalletDexServerSelector) loadDCRWallets() {
	wallets := pg.WL.SortedWalletList(libutils.DCRWalletAsset)
	mainWalletList := make([]*load.WalletItem, 0)
	watchOnlyWalletList := make([]*load.WalletItem, 0)

	for _, wal := range wallets {
		accountsResult, err := wal.GetAccountsRaw()
		if err != nil {
			continue
		}

		var totalBalance int64
		for _, acc := range accountsResult.Accounts {
			totalBalance += acc.Balance.Total.ToInt()
		}

		// sort wallets into normal wallet and watchonly wallets
		if wal.IsWatchingOnlyWallet() {
			listItem := &load.WalletItem{
				Wallet:       wal,
				TotalBalance: wal.ToAmount(totalBalance).String(),
			}

			watchOnlyWalletList = append(watchOnlyWalletList, listItem)
		} else {
			listItem := &load.WalletItem{
				Wallet:       wal,
				TotalBalance: wal.ToAmount(totalBalance).String(),
			}

			mainWalletList = append(mainWalletList, listItem)
		}
	}

	pg.listLock.Lock()
	pg.dcrWalletList = mainWalletList
	pg.dcrWatchOnlyWalletList = watchOnlyWalletList
	pg.listLock.Unlock()
}

func (pg *WalletDexServerSelector) loadBTCWallets() {
	wallets := pg.WL.SortedWalletList(libutils.BTCWalletAsset)
	mainWalletList := make([]*load.WalletItem, 0)
	watchOnlyWalletList := make([]*load.WalletItem, 0)

	for _, wal := range wallets {
		accountsResult, err := wal.GetAccountsRaw()
		if err != nil {
			continue
		}

		var totalBalance int64
		for _, acc := range accountsResult.Accounts {
			totalBalance += acc.Balance.Total.ToInt()
		}

		// sort wallets into normal wallet and watchonly wallets
		if wal.IsWatchingOnlyWallet() {
			listItem := &load.WalletItem{
				Wallet:       wal,
				TotalBalance: wal.ToAmount(totalBalance).String(),
			}

			watchOnlyWalletList = append(watchOnlyWalletList, listItem)
		} else {
			listItem := &load.WalletItem{
				Wallet:       wal,
				TotalBalance: wal.ToAmount(totalBalance).String(),
			}

			mainWalletList = append(mainWalletList, listItem)
		}
	}

	pg.listLock.Lock()
	pg.btcWalletList = mainWalletList
	pg.btcWatchOnlyWalletList = watchOnlyWalletList
	pg.listLock.Unlock()
}

func (pg *WalletDexServerSelector) loadBadWallets() {
	dcrBadWallets := pg.WL.MultiWallet.DCRBadWallets()
	btcBadWallets := pg.WL.MultiWallet.BTCBadWallets()
	pg.dcrBadWalletsList = make([]*badWalletListItem, 0, len(dcrBadWallets))
	pg.btcBadWalletsList = make([]*badWalletListItem, 0, len(btcBadWallets))

	// dcr bad wallets
	for _, badWallet := range dcrBadWallets {
		listItem := &badWalletListItem{
			Wallet:    badWallet,
			deleteBtn: pg.Theme.OutlineButton(values.String(values.StrDeleted)),
		}
		listItem.deleteBtn.Color = pg.Theme.Color.Danger
		listItem.deleteBtn.Inset = layout.Inset{}
		pg.dcrBadWalletsList = append(pg.dcrBadWalletsList, listItem)
	}

	// btc bad wallets
	for _, badWallet := range btcBadWallets {
		listItem := &badWalletListItem{
			Wallet:    badWallet,
			deleteBtn: pg.Theme.OutlineButton(values.String(values.StrDeleted)),
		}
		listItem.deleteBtn.Color = pg.Theme.Color.Danger
		listItem.deleteBtn.Inset = layout.Inset{}
		pg.btcBadWalletsList = append(pg.btcBadWalletsList, listItem)
	}
}

func (pg *WalletDexServerSelector) deleteBadWallet(badWalletID int) {
	warningModal := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrRemoveWallet)).
		Body(values.String(values.StrWalletRestoreMsg)).
		SetNegativeButtonText(values.String(values.StrCancel)).
		PositiveButtonStyle(pg.Load.Theme.Color.Surface, pg.Load.Theme.Color.Danger).
		SetPositiveButtonText(values.String(values.StrRemove)).
		SetPositiveButtonCallback(func(_ bool, im *modal.InfoModal) bool {
			err := pg.WL.MultiWallet.DeleteBadWallet(badWalletID)
			if err != nil {
				errorModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(errorModal)
				return false
			}
			infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrWalletRemoved), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(infoModal)
			pg.loadBadWallets() // refresh bad wallets list
			pg.ParentWindow().Reload()
			return true
		})
	pg.ParentWindow().ShowModal(warningModal)
}

func (pg *WalletDexServerSelector) syncStatusIcon(gtx C, wallet sharedW.Asset) D {
	var (
		syncStatusIcon *cryptomaterial.Image
		syncStatus     string
	)

	switch {
	case wallet.IsSynced():
		syncStatusIcon = pg.Theme.Icons.SuccessIcon
		syncStatus = values.String(values.StrSynced)
	case wallet.IsSyncing():
		syncStatusIcon = pg.Theme.Icons.SyncingIcon
		syncStatus = values.String(values.StrSyncingState)
	default:
		syncStatusIcon = pg.Theme.Icons.NotSynced
		syncStatus = values.String(values.StrWalletNotSynced)
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(syncStatusIcon.Layout16dp),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Left: values.MarginPadding5,
			}.Layout(gtx, pg.Theme.Label(values.TextSize16, syncStatus).Layout)
		}),
	)
}

func (pg *WalletDexServerSelector) DCRwalletListLayout(gtx C) D {
	walletSections := []func(gtx C) D{}
	if len(pg.dcrWalletList) > 0 {
		walletSections = append(walletSections, pg.DCRWalletSection)
	}

	if len(pg.dcrWatchOnlyWalletList) > 0 {
		walletSections = append(walletSections, pg.DCRwatchOnlyWalletSection)
	}

	if len(pg.dcrBadWalletsList) > 0 {
		walletSections = append(walletSections, pg.DCRbadWalletSection)
	}

	list := &layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(walletSections), func(gtx C, i int) D {
		return walletSections[i](gtx)
	})
}

func (pg *WalletDexServerSelector) BTCwalletListLayout(gtx C) D {
	walletSections := []func(gtx C) D{}
	if len(pg.btcWalletList) > 0 {
		walletSections = append(walletSections, pg.BTCWalletSection)
	}

	if len(pg.btcWatchOnlyWalletList) > 0 {
		walletSections = append(walletSections, pg.BTCwatchOnlyWalletSection)
	}

	if len(pg.btcBadWalletsList) > 0 {
		walletSections = append(walletSections, pg.BTCbadWalletSection)
	}

	list := &layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(walletSections), func(gtx C, i int) D {
		return walletSections[i](gtx)
	})
}

func (pg *WalletDexServerSelector) DCRWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()
	mainWalletList := pg.dcrWalletList

	return pg.dcrComponents.Layout(gtx, len(mainWalletList), func(gtx C, i int) D {
		return pg.DCRwalletWrapper(gtx, mainWalletList[i], false)
	})
}

func (pg *WalletDexServerSelector) BTCWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()
	mainWalletList := pg.btcWalletList

	return pg.btcComponents.Layout(gtx, len(mainWalletList), func(gtx C, i int) D {
		return pg.BTCwalletWrapper(gtx, mainWalletList[i], false)
	})
}

func (pg *WalletDexServerSelector) DCRwatchOnlyWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()
	watchOnlyWalletList := pg.dcrWatchOnlyWalletList

	return pg.dcrWatchOnlyComponents.Layout(gtx, len(watchOnlyWalletList), func(gtx C, i int) D {
		return pg.DCRwalletWrapper(gtx, watchOnlyWalletList[i], true)
	})
}

func (pg *WalletDexServerSelector) BTCwatchOnlyWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()
	watchOnlyWalletList := pg.btcWatchOnlyWalletList

	return pg.btcWatchOnlyComponents.Layout(gtx, len(watchOnlyWalletList), func(gtx C, i int) D {
		return pg.BTCwalletWrapper(gtx, watchOnlyWalletList[i], true)
	})
}

func (pg *WalletDexServerSelector) DCRbadWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()

	return pg.badWalletsWrapper(gtx, pg.dcrBadWalletsList)
}

func (pg *WalletDexServerSelector) BTCbadWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()

	return pg.badWalletsWrapper(gtx, pg.btcBadWalletsList)
}

func (pg *WalletDexServerSelector) badWalletsWrapper(gtx C, badWalletsList []*badWalletListItem) D {
	m20 := values.MarginPadding20
	m10 := values.MarginPadding10

	layoutBadWallet := func(gtx C, badWallet *badWalletListItem, lastItem bool) D {
		return layout.Inset{Top: m10, Bottom: m10}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(pg.Theme.Body2(badWallet.Name).Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, badWallet.deleteBtn.Layout)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if lastItem {
						return D{}
					}
					return layout.Inset{Top: values.MarginPadding10, Left: values.MarginPadding38, Right: values.MarginPaddingMinus10}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Separator().Layout(gtx)
					})
				}),
			)
		})
	}

	card := pg.Theme.Card()
	card.Color = pg.Theme.Color.Surface
	card.Radius = cryptomaterial.Radius(10)

	sectionTitleLabel := pg.Theme.Body1("Bad Wallets") // TODO: localize string
	sectionTitleLabel.Color = pg.Theme.Color.GrayText2

	return card.Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: m20, Left: m20}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(sectionTitleLabel.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: m10, Bottom: m10}.Layout(gtx, pg.Theme.Separator().Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
						return pg.Theme.NewClickableList(layout.Vertical).Layout(gtx, len(badWalletsList), func(gtx C, i int) D {
							return layoutBadWallet(gtx, badWalletsList[i], i == len(badWalletsList)-1)
						})
					})
				}),
			)
		})
	})
}

func (pg *WalletDexServerSelector) DCRwalletWrapper(gtx C, item *load.WalletItem, isWatchingOnlyWallet bool) D {
	pg.shadowBox.SetShadowRadius(14)
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding9),
		Background: pg.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Shadow:     pg.shadowBox,
		Margin:     layout.UniformInset(values.MarginPadding5),
		Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding10,
				Left:  values.MarginPadding10,
			}.Layout(gtx, func(gtx C) D {
				if isWatchingOnlyWallet {
					return pg.Theme.Icons.DcrWatchOnly.Layout36dp(gtx)
				}
				return pg.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding30)
			})
		}),
		layout.Rigid(pg.Theme.Label(values.TextSize16, item.Wallet.GetWalletName()).Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if len(item.Wallet.GetEncryptedSeed()) > 0 {
							return layout.Flex{
								Axis:      layout.Horizontal,
								Alignment: layout.Middle,
							}.Layout(gtx,
								layout.Rigid(pg.Theme.Icons.RedAlert.Layout16dp),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Right: values.MarginPadding10,
									}.Layout(gtx, pg.Theme.Label(values.TextSize16, values.String(values.StrNotBackedUp)).Layout)
								}),
							)
						}
						return D{}
					}),
					layout.Rigid(func(gtx C) D {
						return pg.syncStatusIcon(gtx, item.Wallet)
					}),
				)
			})
		}),
	)
}

func (pg *WalletDexServerSelector) BTCwalletWrapper(gtx C, item *load.WalletItem, isWatchingOnlyWallet bool) D {
	pg.shadowBox.SetShadowRadius(14)
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding9),
		Background: pg.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Shadow:     pg.shadowBox,
		Margin:     layout.UniformInset(values.MarginPadding5),
		Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding10,
				Left:  values.MarginPadding10,
			}.Layout(gtx, func(gtx C) D {
				if isWatchingOnlyWallet {
					return pg.Theme.Icons.DcrWatchOnly.Layout36dp(gtx)
				}
				return pg.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding30)
			})
		}),
		layout.Rigid(pg.Theme.Label(values.TextSize16, item.Wallet.GetWalletName()).Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx)
			})
		}),
	)
}

// start sync listener
func (pg *WalletDexServerSelector) listenForNotifications() {
	if pg.isListenerAdded {
		return
	}

	pg.isListenerAdded = true

	for k, w := range pg.WL.SortedWalletList(libutils.DCRWalletAsset) {
		syncListener := listeners.NewSyncProgress()
		dcrUniqueImpl := w.(*dcr.DCRAsset)
		err := dcrUniqueImpl.AddSyncProgressListener(syncListener, WalletDexServerSelectorID)
		if err != nil {
			log.Errorf("Error adding sync progress listener: %v", err)
			return
		}

		go func(wal sharedW.Asset, k int) {
			for {
				select {
				case n := <-syncListener.SyncStatusChan:
					if n.Stage == wallet.SyncCompleted {
						pg.ParentWindow().Reload()
					}
				case <-pg.ctx.Done():
					dcrUniqueImpl.RemoveSyncProgressListener(WalletDexServerSelectorID)
					close(syncListener.SyncStatusChan)
					syncListener = nil
					pg.isListenerAdded = false
					return
				}
			}
		}(w, k)
	}
}
