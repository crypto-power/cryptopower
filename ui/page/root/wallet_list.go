package root

import (
	"gioui.org/layout"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/crypto-power/cryptopower/wallet"
)

func (pg *WalletDexServerSelector) initWalletSelectorOptions() {
	pg.walletComponents = pg.Theme.NewClickableList(layout.Vertical)
}

func (pg *WalletDexServerSelector) loadWallets() {
	wallets := pg.WL.AllSortedWalletList()
	walletsList := make([]*load.WalletItem, 0, len(wallets))

	for _, wal := range wallets {
		var totalBalance int64
		accountsResult, err := wal.GetAccountsRaw()
		if err != nil {
			log.Errorf("wallet (%v) balance was ignored : %v", wal.GetWalletName(), err)
		} else {
			for _, acc := range accountsResult.Accounts {
				totalBalance += acc.Balance.Total.ToInt()
			}
		}

		listItem := &load.WalletItem{
			Wallet:       wal,
			TotalBalance: wal.ToAmount(totalBalance).String(),
		}

		walletsList = append(walletsList, listItem)
	}

	pg.listLock.Lock()
	pg.walletsList = walletsList
	pg.listLock.Unlock()
}

func (pg *WalletDexServerSelector) loadBadWallets() {
	dcrBadWallets := pg.WL.AssetsManager.DCRBadWallets()
	btcBadWallets := pg.WL.AssetsManager.BTCBadWallets()
	ltcBadWallets := pg.WL.AssetsManager.LTCBadWallets()
	pg.badWalletsList = make([]*badWalletListItem, 0, len(dcrBadWallets))

	populatebadWallets := func(badWallets map[int]*sharedW.Wallet) {
		for _, badWallet := range badWallets {
			listItem := &badWalletListItem{
				Wallet:    badWallet,
				deleteBtn: pg.Theme.OutlineButton(values.String(values.StrDeleted)),
			}
			listItem.deleteBtn.Color = pg.Theme.Color.Danger
			listItem.deleteBtn.Inset = layout.Inset{}
			pg.badWalletsList = append(pg.badWalletsList, listItem)
		}
	}

	populatebadWallets(dcrBadWallets) // dcr bad wallets
	populatebadWallets(btcBadWallets) // btc bad wallets
	populatebadWallets(ltcBadWallets) // ltc bad wallets
}

func (pg *WalletDexServerSelector) deleteBadWallet(badWalletID int) {
	warningModal := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrRemoveWallet)).
		Body(values.String(values.StrWalletRestoreMsg)).
		SetNegativeButtonText(values.String(values.StrCancel)).
		PositiveButtonStyle(pg.Load.Theme.Color.Surface, pg.Load.Theme.Color.Danger).
		SetPositiveButtonText(values.String(values.StrRemove)).
		SetPositiveButtonCallback(func(_ bool, im *modal.InfoModal) bool {
			err := pg.WL.AssetsManager.DeleteBadWallet(badWalletID)
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

func (pg *WalletDexServerSelector) walletListLayout(gtx C) D {
	walletSections := []func(gtx C) D{}
	if len(pg.walletsList) > 0 {
		walletSections = append(walletSections, pg.walletSection)
	}

	if len(pg.badWalletsList) > 0 {
		walletSections = append(walletSections, pg.badWalletSection)
	}

	list := &layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(walletSections), func(gtx C, i int) D {
		return walletSections[i](gtx)
	})
}

func (pg *WalletDexServerSelector) walletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()
	mainWalletList := pg.walletsList

	return pg.walletComponents.Layout(gtx, len(mainWalletList), func(gtx C, i int) D {
		wallet := mainWalletList[i]
		return pg.walletWrapper(gtx, wallet.Wallet.GetAssetType(), wallet)
	})
}

func (pg *WalletDexServerSelector) badWalletSection(gtx C) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()

	return pg.badWalletsWrapper(gtx, pg.badWalletsList)
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

func (pg *WalletDexServerSelector) walletWrapper(gtx C, wType libutils.AssetType, item *load.WalletItem) D {
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
				isWatchingOnlyWallet := item.Wallet.IsWatchingOnlyWallet()
				image := components.CoinImageBySymbol(pg.Load, wType, isWatchingOnlyWallet)
				if image != nil {
					return image.LayoutSize(gtx, values.MarginPadding30)
				}
				return D{}
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

// start sync listener
func (pg *WalletDexServerSelector) listenForNotifications() {
	if pg.isListenerAdded {
		return
	}

	pg.isListenerAdded = true

	allWallets := make([]sharedW.Asset, 0)
	allWallets = append(allWallets, pg.WL.SortedWalletList(libutils.DCRWalletAsset)...)
	allWallets = append(allWallets, pg.WL.SortedWalletList(libutils.BTCWalletAsset)...)
	allWallets = append(allWallets, pg.WL.SortedWalletList(libutils.LTCWalletAsset)...)

	for k, w := range allWallets {
		syncListener := listeners.NewSyncProgress()
		err := w.AddSyncProgressListener(syncListener, WalletDexServerSelectorID)
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
					wal.RemoveSyncProgressListener(WalletDexServerSelectorID)
					close(syncListener.SyncStatusChan)
					syncListener = nil
					pg.isListenerAdded = false
					return
				}
			}
		}(w, k)
	}
}
