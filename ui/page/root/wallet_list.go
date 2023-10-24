package root

import (
	"gioui.org/font"
	"gioui.org/layout"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/crypto-power/cryptopower/wallet"
)

func (pg *WalletSelectorPage) initWalletSelectorOptions() {
	pg.walletComponents = pg.Theme.NewClickableList(layout.Vertical)
}

func (pg *WalletSelectorPage) loadWallets() {

	wallets := pg.WL.AllSortedWalletList()
	walletsList := make(map[libutils.AssetType][]*load.WalletItem)

	for _, wal := range wallets {
		balance, err := wal.GetWalletBalance()
		if err != nil {
			log.Errorf("wallet (%v) balance was ignored : %v", wal.GetWalletName(), err)
		}

		listItem := &load.WalletItem{
			Wallet:       wal,
			TotalBalance: balance.Total,
		}

		walletsList[wal.GetAssetType()] = append(walletsList[wal.GetAssetType()], listItem)
	}

	pg.listLock.Lock()
	pg.walletsList[libutils.DCRWalletAsset] = walletsList[libutils.DCRWalletAsset]
	pg.walletsList[libutils.BTCWalletAsset] = walletsList[libutils.BTCWalletAsset]
	pg.walletsList[libutils.LTCWalletAsset] = walletsList[libutils.LTCWalletAsset]
	pg.listLock.Unlock()
}

func (pg *WalletSelectorPage) loadBadWallets() {
	pg.badWalletsList = make(map[libutils.AssetType][]*badWalletListItem)

	dcrBadWallets := pg.WL.AssetsManager.DCRBadWallets()
	btcBadWallets := pg.WL.AssetsManager.BTCBadWallets()
	ltcBadWallets := pg.WL.AssetsManager.LTCBadWallets()

	populateBadWallets := func(assetType libutils.AssetType, badWallets map[int]*sharedW.Wallet) {
		for _, badWallet := range badWallets {
			listItem := &badWalletListItem{
				Wallet:    badWallet,
				deleteBtn: pg.Theme.OutlineButton(values.String(values.StrDeleted)),
			}
			listItem.deleteBtn.Color = pg.Theme.Color.Danger
			listItem.deleteBtn.Inset = layout.Inset{}
			pg.badWalletsList[assetType] = append(pg.badWalletsList[assetType], listItem)
		}
	}

	populateBadWallets(libutils.DCRWalletAsset, dcrBadWallets) // dcr bad wallets
	populateBadWallets(libutils.BTCWalletAsset, btcBadWallets) // btc bad wallets
	populateBadWallets(libutils.LTCWalletAsset, ltcBadWallets) // ltc bad wallets
}

func (pg *WalletSelectorPage) deleteBadWallet(badWalletID int) {
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

func (pg *WalletSelectorPage) syncStatusIcon(gtx C, wallet sharedW.Asset) D {
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

func (pg *WalletSelectorPage) walletListLayout(gtx C, assetType libutils.AssetType) D {
	walletSections := []func(gtx C) D{}
	if len(pg.walletsList[assetType]) > 0 {
		walletSection := func(gtx C) D {
			return pg.walletSection(gtx, pg.walletsList[assetType])
		}
		walletSections = append(walletSections, walletSection)
	}

	if len(pg.badWalletsList[assetType]) > 0 {
		badWalletSection := func(gtx C) D {
			return pg.badWalletSection(gtx, pg.badWalletsList[assetType])
		}
		walletSections = append(walletSections, badWalletSection)
	}

	list := &layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(walletSections), func(gtx C, i int) D {
		return walletSections[i](gtx)
	})
}

func (pg *WalletSelectorPage) walletSection(gtx C, mainWalletList []*load.WalletItem) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()

	var itemIDs []walletIndexTuple
	for i, wallet := range mainWalletList {
		globalIndex := len(itemIDs)
		itemIDs = append(itemIDs, walletIndexTuple{
			AssetType: wallet.Wallet.GetAssetType(),
			Index:     globalIndex,
		})

		// Populate the mapping here
		pg.indexMapping[globalIndex] = walletIndexTuple{
			AssetType: wallet.Wallet.GetAssetType(),
			Index:     i,
		}
	}

	return pg.walletComponents.Layout(gtx, len(itemIDs), func(gtx C, i int) D {
		SelectedWalletItem := itemIDs[i]
		wallet := mainWalletList[SelectedWalletItem.Index]
		return pg.walletWrapper(gtx, wallet)
	})
}

func (pg *WalletSelectorPage) badWalletSection(gtx C, badWalletsList []*badWalletListItem) D {
	pg.listLock.RLock()
	defer pg.listLock.RUnlock()

	return pg.badWalletsWrapper(gtx, badWalletsList)
}

func (pg *WalletSelectorPage) badWalletsWrapper(gtx C, badWalletsList []*badWalletListItem) D {
	m16 := values.MarginPadding16
	m10 := values.MarginPadding10

	layoutBadWallet := func(gtx C, badWallet *badWalletListItem, lastItem bool) D {
		return layout.Inset{Bottom: m10}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(pg.Theme.Body2(badWallet.Name).Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, badWallet.deleteBtn.Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if lastItem {
						return D{}
					}
					return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Separator().Layout(gtx)
					})
				}),
			)
		})
	}

	return cryptomaterial.LinearLayout{
		Width:  cryptomaterial.WrapContent,
		Height: cryptomaterial.WrapContent,
		Padding: layout.Inset{
			Top:    values.MarginPadding16,
			Bottom: values.MarginPadding16},
		Background: pg.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Shadow:     pg.shadowBox,
		Margin: layout.Inset{
			Top:    values.MarginPadding8,
			Bottom: values.MarginPadding2,
			Left:   values.MarginPadding16,
			Right:  values.MarginPadding16},
		Border: cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: m16, Right: m16}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Label(values.TextSize16, "Bad Wallets")
						txt.Color = pg.Theme.Color.Text
						txt.Font.Weight = font.SemiBold
						return txt.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: m10, Bottom: m10}.Layout(gtx, pg.Theme.Separator().Layout)
					}),
					layout.Rigid(func(gtx C) D {
						return pg.Theme.NewClickableList(layout.Vertical).Layout(gtx, len(badWalletsList), func(gtx C, i int) D {
							return layoutBadWallet(gtx, badWalletsList[i], i == len(badWalletsList)-1)
						})
					}),
				)
			})
		}),
	)
}

func (pg *WalletSelectorPage) walletWrapper(gtx C, item *load.WalletItem) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding16),
		Background: pg.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Shadow:     pg.shadowBox,
		Margin: layout.Inset{
			Top:    values.MarginPadding8,
			Bottom: values.MarginPadding4,
			Left:   values.MarginPadding16,
			Right:  values.MarginPadding16},
		Border: cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Vertical,
				Alignment: layout.Start,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							txt := pg.Theme.Label(values.TextSize16, item.Wallet.GetWalletName())
							txt.Color = pg.Theme.Color.Text
							txt.Font.Weight = font.SemiBold
							return txt.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if item.Wallet.IsWatchingOnlyWallet() {
								return layout.Inset{
									Left: values.MarginPadding8,
								}.Layout(gtx, func(gtx C) D {
									return walletHightlighLabel(pg.Theme, gtx, values.TextSize12, values.String(values.StrWatchOnly))
								})
							}
							return D{}
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.syncStatusIcon(gtx, item.Wallet)
						}),
						layout.Rigid(func(gtx C) D {
							if len(item.Wallet.GetEncryptedSeed()) > 0 {
								return layout.Flex{
									Axis:      layout.Horizontal,
									Alignment: layout.Middle,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Left:  values.MarginPadding8,
											Right: values.MarginPadding8,
										}.Layout(gtx, pg.Theme.Icons.Dot.Layout8dp)
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Right: values.MarginPadding4,
										}.Layout(gtx, pg.Theme.Icons.RedAlert.Layout16dp)
									}),
									layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrNotBackedUp)).Layout),
								)
							}
							return D{}
						}),
					)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Vertical,
					Alignment: layout.End,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Label(values.TextSize16, item.TotalBalance.String())
						txt.Color = pg.Theme.Color.Text
						txt.Font.Weight = font.SemiBold
						return txt.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if components.IsFetchExchangeRateAPIAllowed(pg.WL) {
							usdBalance := utils.FormatAsUSDString(pg.Printer, item.TotalBalance.MulF64(pg.assetRate[item.Wallet.GetAssetType()]).ToCoin())
							txt := pg.Theme.Label(values.TextSize16, usdBalance)
							txt.Color = pg.Theme.Color.Text
							return txt.Layout(gtx)
						}

						txt := pg.Theme.Label(values.TextSize16, "$--")
						txt.Color = pg.Theme.Color.Text
						return txt.Layout(gtx)
					}),
				)
			})
		}),
	)
}

// start sync listener
func (pg *WalletSelectorPage) listenForNotifications() {
	if pg.isListenerAdded {
		return
	}

	pg.isListenerAdded = true

	allWallets := pg.WL.AllSortedWalletList()
	for _, w := range allWallets {
		syncListener := listeners.NewSyncProgress()
		err := w.AddSyncProgressListener(syncListener, WalletSelectorPageID)
		if err != nil {
			log.Errorf("Error adding sync progress listener: %v", err)
			return
		}

		go func(wal sharedW.Asset) {
			for {
				select {
				case n := <-syncListener.SyncStatusChan:
					if n.Stage == wallet.SyncCompleted {
						pg.ParentWindow().Reload()
					}
				case <-pg.ctx.Done():
					wal.RemoveSyncProgressListener(WalletSelectorPageID)
					close(syncListener.SyncStatusChan)
					syncListener = nil
					pg.isListenerAdded = false
					return
				}
			}
		}(w)
	}
}
