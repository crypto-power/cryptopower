package root

import (
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/wallet"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletSelectorPageID = "wallet_selector"

type (
	C = layout.Context
	D = layout.Dimensions
)

type badWalletListItem struct {
	*sharedW.Wallet
	deleteBtn cryptomaterial.Button
}

type walletIndexTuple struct {
	AssetType libutils.AssetType
	Index     int
}

type showNavigationFunc func(showNavigation bool)

type walletWithBalance struct {
	wallet       sharedW.Asset
	totalBalance sharedW.AssetAmount
}

type WalletSelectorPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer        *widget.List
	assetDropdownContainer *widget.List
	shadowBox              *cryptomaterial.Shadow
	addWalClickable        map[libutils.AssetType]*cryptomaterial.Clickable

	// wallet selector options
	listLock       sync.RWMutex
	walletsList    map[libutils.AssetType][]*walletWithBalance
	indexMapping   map[int]walletIndexTuple
	badWalletsList map[libutils.AssetType][]*badWalletListItem

	walletComponents      *cryptomaterial.ClickableList
	assetCollapsibles     map[libutils.AssetType]*cryptomaterial.Collapsible
	assetsBalance         map[libutils.AssetType]sharedW.AssetAmount
	assetsTotalUSDBalance map[libutils.AssetType]float64
	assetRate             map[libutils.AssetType]float64

	showNavigationFunc showNavigationFunc
}

func NewWalletSelectorPage(l *load.Load) *WalletSelectorPage {
	pg := &WalletSelectorPage{
		GenericPageModal: app.NewGenericPageModal(WalletSelectorPageID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		assetDropdownContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		Load:      l,
		shadowBox: l.Theme.Shadow(),
	}

	pg.assetCollapsibles = make(map[libutils.AssetType]*cryptomaterial.Collapsible)
	pg.assetsBalance = make(map[libutils.AssetType]sharedW.AssetAmount)
	pg.assetsTotalUSDBalance = make(map[libutils.AssetType]float64)
	pg.assetRate = make(map[libutils.AssetType]float64)
	pg.walletsList = make(map[libutils.AssetType][]*walletWithBalance)
	pg.indexMapping = make(map[int]walletIndexTuple)
	pg.addWalClickable = make(map[libutils.AssetType]*cryptomaterial.Clickable)

	pg.initWalletSelectorOptions()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *WalletSelectorPage) OnNavigatedTo() {
	pg.showNavigationFunc(false)

	for _, asset := range pg.AssetsManager.AllAssetTypes() {
		pg.assetCollapsibles[asset] = pg.Load.Theme.Collapsible()
		pg.addWalClickable[asset] = pg.Load.Theme.NewClickable(false)
		pg.addWalClickable[asset].Radius = cryptomaterial.Radius(14)
	}

	go func() {
		// calculate total assets balance
		assetsBalance, err := pg.AssetsManager.CalculateTotalAssetsBalance(true)
		if err != nil {
			log.Error(err)
		}
		pg.assetsBalance = assetsBalance

		// calculate total assets balance in USD
		assetsTotalUSDBalance, err := pg.AssetsManager.CalculateAssetsUSDBalance(assetsBalance)
		if err != nil {
			log.Error(err)
		}
		pg.assetsTotalUSDBalance = assetsTotalUSDBalance

		// calculate assets USD rate
		for assetType := range assetsBalance {
			marketValue, exist := values.AssetExchangeMarketValue[assetType]
			if !exist {
				log.Errorf("Unsupported asset type: %s", assetType)
				break
			}

			rate := pg.AssetsManager.RateSource.GetTicker(marketValue, true)
			if err != nil {
				log.Error(err)
				break
			}
			pg.assetRate[assetType] = rate.LastTradePrice
		}

		pg.ParentWindow().Reload()
	}()

	pg.listenForSyncProgressNotifications() // sync progress listener is stopped in OnNavigatedFrom()
	pg.loadWallets()
	pg.loadBadWallets()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletSelectorPage) HandleUserInteractions() {
	pg.listLock.Lock()
	defer pg.listLock.Unlock()

	if ok, clickedItem := pg.walletComponents.ItemClicked(); ok {
		tuple, exists := pg.indexMapping[clickedItem]
		if !exists {
			// Handle error - this should never happen
			return
		}

		wallets, wExists := pg.walletsList[tuple.AssetType]
		if !wExists || len(wallets) <= tuple.Index {
			// Handle error
			return
		}

		selectedWallet := wallets[tuple.Index].wallet
		pg.showNavigationFunc(true)

		callback := func() {
			pg.ParentNavigator().CloseCurrentPage()
		}
		pg.ParentNavigator().Display(wallet.NewSingleWalletMasterPage(pg.Load, selectedWallet, callback))
	}

	for _, walletsOfType := range pg.badWalletsList {
		for _, badWallet := range walletsOfType {
			if badWallet.deleteBtn.Clicked() {
				pg.deleteBadWallet(badWallet.Wallet.ID)
				pg.ParentWindow().Reload()
			}
		}
	}

	for asset, clickable := range pg.addWalClickable {
		// Create a local copy of asset for each iteration
		asset := asset
		if clickable.Clicked() {
			pg.ParentNavigator().Display(components.NewCreateWallet(pg.Load, func() {
				pg.ParentNavigator().ClosePagesAfter(WalletSelectorPageID)
				// enable sync for the newly created wallet
				wallets, wExists := pg.walletsList[asset]
				var mostRecentWallet *walletWithBalance
				if wExists && len(wallets) > 0 {
					// Getting the most recent wallet in the list
					mostRecentWallet = wallets[len(wallets)-1]
					pg.ToggleSync(mostRecentWallet.wallet, func(b bool) {
						mostRecentWallet.wallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
					})
				}
			}, asset))
		}
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletSelectorPage) OnNavigatedFrom() {
	pg.stopSyncProgressListeners()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *WalletSelectorPage) Layout(gtx C) D {
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *WalletSelectorPage) layoutDesktop(gtx C) D {
	return pg.pageContentLayout(gtx)
}

func (pg *WalletSelectorPage) layoutMobile(gtx C) D {
	return components.HorizontalInset(values.MarginPadding10).Layout(gtx, pg.pageContentLayout)
}

func (pg *WalletSelectorPage) pageContentLayout(gtx C) D {
	assetDropdown := func(gtx C) D {
		supportedAssets := pg.AssetsManager.AllAssetTypes()
		return pg.Theme.List(pg.assetDropdownContainer).Layout(gtx, len(supportedAssets), func(gtx C, i int) D {
			top := values.MarginPadding15
			bottom := values.MarginPadding0
			return layout.Inset{Top: top, Bottom: bottom}.Layout(gtx, pg.assetDropdown(supportedAssets[i]))
		})
	}

	pageContent := []func(gtx C) D{
		assetDropdown,
	}

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		width := values.MarginPadding550
		if pg.IsMobileView() {
			width = pg.CurrentAppWidth()
		}
		return cryptomaterial.LinearLayout{
			Width:   gtx.Dp(width),
			Height:  cryptomaterial.MatchParent,
			Padding: components.HorizontalInset(values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding16)),
		}.Layout2(gtx, func(gtx C) D {
			return pg.Theme.List(pg.scrollContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
				return pageContent[i](gtx)
			})
		})
	})
}

func (pg *WalletSelectorPage) assetDropdown(asset libutils.AssetType) layout.Widget {
	return func(gtx C) D {
		return pg.assetCollapsibles[asset].Layout(gtx,
			func(gtx C) D {
				return pg.dropdownTitleLayout(gtx, asset)
			},
			func(gtx C) D {
				return pg.dropdownContentLayout(gtx, asset)
			},
		)
	}
}

func (pg *WalletSelectorPage) dropdownTitleLayout(gtx C, asset libutils.AssetType) D {
	margin := layout.Inset{}
	if pg.assetCollapsibles[asset].IsExpanded() {
		margin = layout.Inset{Bottom: values.MarginPadding5}
		for key := range pg.assetCollapsibles {
			if key != asset {
				pg.assetCollapsibles[key].SetExpanded(false)
				pg.ParentWindow().Reload()
			}
		}
	}
	pg.shadowBox.SetShadowRadius(20)
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding18),
		Background: pg.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Shadow:     pg.shadowBox,
		Margin:     margin,
		Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding8,
				Left:  values.MarginPadding8,
			}.Layout(gtx, func(gtx C) D {
				image := components.CoinImageBySymbol(pg.Load, asset, false)
				if image != nil {
					return image.LayoutSize(gtx, values.MarginPadding30)
				}
				return D{}
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					txt := pg.Theme.Label(values.TextSize16, asset.String())
					txt.Color = pg.Theme.Color.Text
					txt.Font.Weight = font.SemiBold
					return txt.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					txt := pg.Theme.Label(values.TextSize16, asset.ToFull())
					txt.Color = pg.Theme.Color.Text
					return txt.Layout(gtx)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{
							Axis:      layout.Vertical,
							Alignment: layout.End,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								// check if asset balance is nil
								if pg.assetsBalance[asset] == nil || pg.assetsBalance[asset].String() == "0 BTC" {
									txt := pg.Theme.Label(values.TextSize16, "0.00 "+asset.String())
									txt.Color = pg.Theme.Color.Text
									txt.Font.Weight = font.SemiBold
									return txt.Layout(gtx)
								}
								return components.LayoutBalanceWithStateSemiBold(gtx, pg.Load, pg.assetsBalance[asset].String())
							}),
							layout.Rigid(func(gtx C) D {
								usdBalance := ""
								if pg.AssetsManager.ExchangeRateFetchingEnabled() {
									usdBalance = utils.FormatAsUSDString(pg.Printer, pg.assetsTotalUSDBalance[asset])
								}
								return components.LayoutBalanceWithStateUSD(gtx, pg.Load, usdBalance)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
							if pg.assetCollapsibles[asset].IsExpanded() {
								return pg.Theme.NewIcon(pg.Theme.Icons.ChevronUp).Layout16dp(gtx)
							}

							return pg.Theme.NewIcon(pg.Theme.Icons.ChevronDown).Layout16dp(gtx)
						})
					}),
				)
			})
		}),
	)
}

func (pg *WalletSelectorPage) dropdownContentLayout(gtx C, asset libutils.AssetType) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     cryptomaterial.WrapContent,
		Background: pg.Theme.Color.LightGray,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.CornerRadius{
				BottomLeft:  int(values.MarginPadding14),
				BottomRight: int(values.MarginPadding14),
			},
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if len(pg.walletsList[asset]) > 0 {
							return pg.walletListLayout(gtx, asset)
						}
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Center.Layout(gtx, func(gtx C) D {
							txt := pg.Theme.Label(values.TextSize16, "No wallets created yet")
							txt.Color = pg.Theme.Color.GrayText3
							return txt.Layout(gtx)
						})
					}),
					layout.Rigid(pg.layoutAddMoreRowSection(pg.addWalClickable[asset], values.String(values.StrAddWallet), pg.Theme.Icons.NewWalletIcon.Layout20dp)),
				)
			})
		}),
	)
}

func (pg *WalletSelectorPage) layoutAddMoreRowSection(clk *cryptomaterial.Clickable, buttonText string, ic func(gtx C) D) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{
			Left:   values.MarginPadding24,
			Top:    values.MarginPadding16,
			Bottom: values.MarginPadding24,
		}.Layout(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:      cryptomaterial.WrapContent,
				Height:     cryptomaterial.WrapContent,
				Background: pg.Theme.Color.DefaultThemeColors().SurfaceHighlight,
				Clickable:  clk,
				Border:     cryptomaterial.Border{Radius: clk.Radius},
				Alignment:  layout.Middle,
			}.Layout(gtx,
				layout.Rigid(ic),
				layout.Rigid(func(gtx C) D {
					txt := pg.Theme.Label(values.TextSize16, buttonText)
					txt.Color = pg.Theme.Color.DefaultThemeColors().Primary
					txt.Font.Weight = font.SemiBold
					return layout.Inset{
						Left: values.MarginPadding8,
					}.Layout(gtx, txt.Layout)
				}),
			)
		})
	}
}
