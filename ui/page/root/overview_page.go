package root

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/decred/dcrd/dcrutil/v3"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/privacy"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	OverviewPageID = "Overview"
)

type multiWalletTx struct {
	*sharedW.Transaction
	walletID int
}

type OverviewPage struct {
	*app.GenericPageModal
	*load.Load

	pageContainer            layout.List
	marketOverviewList       layout.List
	mobileMarketOverviewList layout.List
	recentProposalList       layout.List
	recentTradeList          layout.List
	recentTransactions       layout.List
	recentStakes             layout.List

	scrollContainer               *widget.List
	mobileMarketOverviewContainer *widget.List

	infoButton, forwardButton cryptomaterial.IconButton // TOD0: use *cryptomaterial.Clickable
	assetBalanceSlider        *cryptomaterial.Slider
	mixerSlider               *cryptomaterial.Slider
	proposalItems             []*components.ProposalItem
	orders                    []*instantswap.Order
	transactions              []*multiWalletTx
	stakes                    []*multiWalletTx
	sliderRedirectBtn         *cryptomaterial.Clickable
	mktValues                 []assetMarketData

	card cryptomaterial.Card

	dcr *assetBalanceSliderItem
	btc *assetBalanceSliderItem
	ltc *assetBalanceSliderItem

	assetsTotalBalance map[libutils.AssetType]sharedW.AssetAmount

	materialLoader    material.LoaderStyle
	forceRefreshRates *cryptomaterial.Clickable

	mixerSliderData      map[int]*mixerData
	sortedMixerSlideKeys []int

	showNavigationFunc showNavigationFunc
}

type assetBalanceSliderItem struct {
	assetType       string
	totalBalance    sharedW.AssetAmount
	totalBalanceUSD string

	image           *cryptomaterial.Image
	backgroundImage *cryptomaterial.Image
}

type assetMarketData struct {
	assetType libutils.AssetType
	market    string
	image     *cryptomaterial.Image
}

type mixerData struct {
	*dcr.Asset
	unmixedBalance sharedW.AssetAmount
}

func NewOverviewPage(l *load.Load, showNavigationFunc showNavigationFunc) *OverviewPage {
	pg := &OverviewPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OverviewPageID),
		pageContainer: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		marketOverviewList: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		mobileMarketOverviewList: layout.List{
			Axis:      layout.Horizontal,
			Alignment: layout.Start,
		},
		mobileMarketOverviewContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Horizontal,
				Alignment: layout.Start,
			},
		},
		recentTradeList: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		recentProposalList: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		mktValues: []assetMarketData{
			{
				assetType: libutils.DCRWalletAsset,
				market:    values.DCRUSDTMarket,
				image:     l.Theme.Icons.DCR,
			},
			{
				assetType: libutils.BTCWalletAsset,
				market:    values.BTCUSDTMarket,
				image:     l.Theme.Icons.BTC,
			},
			{
				assetType: libutils.LTCWalletAsset,
				market:    values.LTCUSDTMarket,
				image:     l.Theme.Icons.LTC,
			},
		},
		recentTransactions: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		recentStakes: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},

		assetBalanceSlider: l.Theme.Slider(),
		card:               l.Theme.Card(),
		sliderRedirectBtn:  l.Theme.NewClickable(false),
		forceRefreshRates:  l.Theme.NewClickable(false),
		showNavigationFunc: showNavigationFunc,
	}

	pg.materialLoader = material.Loader(l.Theme.Base)
	pg.mixerSlider = l.Theme.Slider()
	pg.mixerSlider.ButtonBackgroundColor = values.TransparentColor(values.TransparentDeepBlue, 0.02)
	pg.mixerSlider.IndicatorBackgroundColor = values.TransparentColor(values.TransparentDeepBlue, 0.02)
	pg.mixerSlider.SelectedIndicatorColor = pg.Theme.Color.DeepBlue

	pg.forwardButton, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.forwardButton.Icon = pg.Theme.Icons.NavigationArrowForward
	pg.forwardButton.Size = values.MarginPadding20

	pg.assetsTotalBalance = make(map[libutils.AssetType]sharedW.AssetAmount)

	pg.stakes = make([]*multiWalletTx, 0)
	pg.transactions = make([]*multiWalletTx, 0)

	return pg
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (pg *OverviewPage) ID() string {
	return OverviewPageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *OverviewPage) OnNavigatedTo() {
	pg.updateAssetsSliders()
	go pg.updateAssetsUSDBalance()
	go pg.loadTransactions()

	pg.proposalItems = components.LoadProposals(pg.Load, libwallet.ProposalCategoryAll, 0, 3, true)
	pg.orders = components.LoadOrders(pg.Load, 0, 3, true)

	if components.IsFetchExchangeRateAPIAllowed(pg.WL) {
		go pg.WL.AssetsManager.RateSource.Refresh(false)
	}

	pg.listenForMixerNotifications() // listeners are stopped in OnNavigatedFrom().

}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *OverviewPage) HandleUserInteractions() {
	for pg.sliderRedirectBtn.Clicked() {
		walPage := NewWalletSelectorPage(pg.Load)
		walPage.showNavigationFunc = pg.showNavigationFunc
		pg.ParentNavigator().Display(walPage)
	}

	if pg.forceRefreshRates.Clicked() {
		go pg.WL.AssetsManager.RateSource.Refresh(true)
	}

	// Navigate to mixer page when wallet mixer slider forward button is clicked.
	if pg.forwardButton.Button.Clicked() {
		curSliderIndex := pg.mixerSlider.GetSelectedIndex()
		mixerData := pg.mixerSliderData[pg.sortedMixerSlideKeys[curSliderIndex]]
		pg.WL.SelectedWallet = &load.WalletItem{
			Wallet: mixerData.Asset,
		}

		pg.showNavigationFunc(true)
		walletCallbackFunc := func() {
			pg.showNavigationFunc(false)
		}
		mp := NewMainPage(pg.Load, walletCallbackFunc)
		pg.ParentNavigator().Display(mp)
		mp.Display(privacy.NewAccountMixerPage(pg.Load)) // Display mixer page on the main page.
		mp.pageNavigationTab.SetSelectedSegment(values.String(values.StrStakeShuffle))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *OverviewPage) OnNavigatedFrom() {
	pg.stopNtfnListeners()
}

func (pg *OverviewPage) OnCurrencyChanged() {
	go pg.updateAssetsUSDBalance()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *OverviewPage) Layout(gtx C) D {
	pg.Load.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *OverviewPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	pageContent := []func(gtx C) D{
		pg.sliderLayout,
		pg.marketOverview,
		pg.txStakingSection,
		pg.recentTrades,
		pg.recentProposal,
	}

	return components.UniformPadding(gtx, func(gtx C) D {
		return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
						return pageContent[i](gtx)
					})
				})
			})
		})
	})
}

func (pg *OverviewPage) layoutMobile(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.sliderLayout,
		pg.mobileMarketOverview,
		pg.txStakingSection,
		pg.recentTrades,
		pg.recentProposal,
	}

	return components.UniformPadding(gtx, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
				return pageContent[i](gtx)
			})
		})
	})
}

func (pg *OverviewPage) sliderLayout(gtx C) D {
	axis := layout.Horizontal
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		axis = layout.Vertical
	}

	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: axis,
		Direction:   layout.Center,
		Margin:      layout.Inset{Bottom: values.MarginPadding20},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// Only show mixer slider if mixer is running
			if len(pg.mixerSliderData) == 0 {
				return pg.assetBalanceSliderLayout(gtx)
			}

			if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
				return layout.Flex{Axis: axis}.Layout(gtx,
					layout.Rigid(pg.assetBalanceSliderLayout),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, pg.mixerSliderLayout)
					}),
				)
			}

			return layout.Flex{}.Layout(gtx,
				layout.Flexed(.5, pg.assetBalanceSliderLayout),
				layout.Flexed(.5, func(gtx C) D {
					return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, pg.mixerSliderLayout)
				}),
			)
		}),
	)
}

func (pg *OverviewPage) assetBalanceSliderLayout(gtx C) D {
	var sliderWidget []layout.Widget

	if pg.dcr != nil {
		sliderWidget = append(sliderWidget, pg.assetBalanceItemLayout(pg.dcr))
	}
	if pg.btc != nil {
		sliderWidget = append(sliderWidget, pg.assetBalanceItemLayout(pg.btc))
	}
	if pg.ltc != nil {
		sliderWidget = append(sliderWidget, pg.assetBalanceItemLayout(pg.ltc))
	}

	return pg.assetBalanceSlider.Layout(gtx, sliderWidget)
}

func (pg *OverviewPage) assetBalanceItemLayout(item *assetBalanceSliderItem) layout.Widget {
	return func(gtx C) D {
		return pg.sliderRedirectBtn.Layout(gtx, func(gtx C) D {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					width := gtx.Constraints.Max.X
					height := width / item.backgroundImage.AspectRatio() // maintain aspect ratio
					return item.backgroundImage.LayoutSizeWithRadius(gtx, gtx.Metric.PxToDp(width), gtx.Metric.PxToDp(height), 8)
				}),
				layout.Expanded(func(gtx C) D {
					col := pg.Theme.Color.InvText
					return layout.Flex{
						Axis:      layout.Vertical,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							lbl := pg.Theme.Body1(item.assetType)
							lbl.Color = col
							return pg.centerLayout(gtx, values.MarginPadding15, values.MarginPadding10, lbl.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.centerLayout(gtx, values.MarginPadding0, values.MarginPadding10, func(gtx C) D {
								return item.image.LayoutSize(gtx, values.MarginPadding65)
							})
						}),
						layout.Rigid(func(gtx C) D {
							return pg.centerLayout(gtx, values.MarginPadding0, values.MarginPadding10, func(gtx C) D {
								return components.LayoutBalanceColorWithState(gtx, pg.Load, item.totalBalance.String(), col)
							})
						}),
						layout.Rigid(func(gtx C) D {
							card := pg.Theme.Card()
							card.Radius = cryptomaterial.Radius(12)
							card.Color = values.TransparentColor(values.TransparentBlack, 0.2)
							return pg.centerLayout(gtx, values.MarginPadding0, values.MarginPadding0, func(gtx C) D {
								return card.Layout(gtx, func(gtx C) D {
									return layout.Inset{
										Top:    values.MarginPadding4,
										Bottom: values.MarginPadding4,
										Right:  values.MarginPadding8,
										Left:   values.MarginPadding8,
									}.Layout(gtx, func(gtx C) D {
										return components.LayoutBalanceColorWithStateUSD(gtx, pg.Load, item.totalBalanceUSD, col)
									})
								})
							})
						}),
					)
				}),
			)
		})
	}
}

func (pg *OverviewPage) mixerSliderLayout(gtx C) D {
	sliderWidget := make([]layout.Widget, 0)
	for _, key := range pg.sortedMixerSlideKeys {
		// Append the mixer slide widgets in an anonymous function. This stops
		// the the function literal from capturing only the final key {key}
		// value.
		addMixerSlideWidget := func(k int) {
			if slideData, ok := pg.mixerSliderData[k]; ok {
				sliderWidget = append(sliderWidget, func(gtx C) D {
					return pg.mixerLayout(gtx, slideData)
				})
			}
		}
		addMixerSlideWidget(key)
	}

	return pg.mixerSlider.Layout(gtx, sliderWidget)
}

func (pg *OverviewPage) mixerLayout(gtx C, data *mixerData) D {
	r := 8
	return cryptomaterial.LinearLayout{
		Width:       gtx.Constraints.Max.X,
		Height:      gtx.Dp(values.MarginPadding221),
		Orientation: layout.Vertical,
		Padding:     layout.UniformInset(values.MarginPadding15),
		Background:  pg.Theme.Color.Surface,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.CornerRadius{
				TopLeft:     r,
				TopRight:    r,
				BottomRight: r,
				BottomLeft:  r,
			},
		},
	}.Layout(gtx,
		layout.Rigid(pg.topMixerLayout),
		layout.Rigid(pg.middleMixerLayout),
		layout.Rigid(
			func(gtx C) D {
				return pg.bottomMixerLayout(gtx, data)
			},
		),
	)
}

func (pg *OverviewPage) topMixerLayout(gtx C) D {
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(pg.Theme.Icons.Mixer.Layout24dp),
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.Body1(values.String(values.StrMixerRunning))
			lbl.Font.Weight = font.SemiBold
			return layout.Inset{
				Left:  values.MarginPadding8,
				Right: values.MarginPadding8,
			}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(pg.infoButton.Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, pg.forwardButton.Layout)
		}),
	)
}

func (pg *OverviewPage) middleMixerLayout(gtx C) D {
	r := gtx.Dp(7)
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Padding: layout.Inset{
			Left:   values.MarginPadding10,
			Right:  values.MarginPadding10,
			Top:    values.MarginPadding4,
			Bottom: values.MarginPadding4,
		},
		Margin: layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		},
		Background: pg.Theme.Color.LightBlue7,
		Alignment:  layout.Middle,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.CornerRadius{
				TopLeft:     r,
				TopRight:    r,
				BottomRight: r,
				BottomLeft:  r,
			},
		},
	}.Layout(gtx,
		layout.Rigid(pg.Theme.Icons.Alert.Layout20dp),
		layout.Rigid(func(gtc C) D {
			lbl := pg.Theme.Body2(values.String(values.StrKeepAppOpen))
			return layout.Inset{Left: values.MarginPadding6}.Layout(gtx, lbl.Layout)
		}),
	)
}

func (pg *OverviewPage) bottomMixerLayout(gtx C, data *mixerData) D {
	r := 8
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Padding:     layout.UniformInset(values.MarginPadding15),
		Background:  pg.Theme.Color.Gray4,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.CornerRadius{
				TopLeft:     r,
				TopRight:    r,
				BottomRight: r,
				BottomLeft:  r,
			},
		},
	}.Layout(gtx,
		layout.Rigid(func(gtc C) D {
			lbl := pg.Theme.Body2(data.GetWalletName())
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(pg.Theme.Body1(values.String(values.StrUnmixedBalance)).Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return components.LayoutBalance(gtx, pg.Load, data.unmixedBalance.String())
					})
				}),
			)
		}),
	)
}

func (pg *OverviewPage) marketOverview(gtx C) D {
	rates := pg.marketRates()
	if len(rates) == 0 {
		return D{}
	}

	titleLayout := func(gtx layout.Context) layout.Dimensions {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Horizontal,
			Spacing:     layout.SpaceBetween,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return pg.Theme.Body2(values.String(values.StrMarketOverview)).Layout(gtx)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, pg.ratesRefreshComponent())
			}),
		)
	}

	return pg.pageContentWrapper(gtx, values.String(values.StrMarketOverview), titleLayout, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(pg.marketTableHeader),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
					return pg.marketOverviewList.Layout(gtx, len(pg.mktValues), func(gtx C, i int) D {
						asset := pg.mktValues[i]
						rate, ok := rates[asset.market]
						if !ok {
							return D{}
						}

						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return pg.marketTableRows(gtx, asset, rate)
							}),
							layout.Rigid(func(gtx C) D {
								// No divider for last row
								if i == len(pg.mktValues)-1 {
									return layout.Dimensions{}
								}

								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								separator := pg.Theme.Separator()
								return layout.E.Layout(gtx, func(gtx C) D {
									// Show bottom divider for all rows except last
									return layout.Inset{
										Left:   values.MarginPadding33,
										Top:    values.MarginPadding10,
										Bottom: values.MarginPadding15,
									}.Layout(gtx, separator.Layout)
								})
							}),
						)
					})
				})
			}),
		)
	})
}

func (pg *OverviewPage) mobileMarketOverview(gtx C) D {
	rates := pg.marketRates()
	if len(rates) == 0 {
		return D{}
	}

	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Background:  pg.Theme.Color.DefaultThemeColors().SurfaceHighlight,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.MatchParent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Spacing:     layout.SpaceBetween,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							txt := pg.Theme.Label(values.TextSize18, values.String(values.StrMarketOverview))
							txt.Color = pg.Theme.Color.Text
							return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, txt.Layout)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, pg.ratesRefreshComponent())
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.mobileMarketOverviewList.Layout(gtx, len(pg.mktValues), func(gtx C, i int) D {
						asset := pg.mktValues[i]
						rate, ok := rates[asset.market]
						if !ok {
							return D{}
						}
						changeStr := "----"
						var isPositiveChange *bool
						if rate.PriceChangePercent != nil {
							change := *rate.PriceChangePercent
							if change < 0 {
								no := false
								isPositiveChange = &no
							}
							if change > 0 {
								yes := false
								isPositiveChange = &yes
							}
							changeStr = fmt.Sprintf("%.2f", change) + "%"
						}

						card := pg.Theme.Card()
						radius := cryptomaterial.CornerRadius{TopLeft: 20, BottomLeft: 20, TopRight: 20, BottomRight: 20}
						card.Radius = cryptomaterial.Radius(8)
						card.Color = pg.Theme.Color.DefaultThemeColors().White
						return layout.Inset{Right: values.MarginPadding12}.Layout(gtx, func(gtx C) D {
							return card.Layout(gtx, func(gtx C) D {
								return cryptomaterial.LinearLayout{
									Width:       gtx.Dp(values.MarginPadding150),
									Height:      cryptomaterial.WrapContent,
									Orientation: layout.Vertical,
									Alignment:   layout.Middle,

									Border: cryptomaterial.Border{
										Radius: radius,
									},
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										// DCR has a different icon on mobile.
										if asset.assetType == libutils.DCRWalletAsset {
											return layout.Inset{Top: values.MarginPadding12}.Layout(gtx, pg.Theme.Icons.DCRBlue.Layout48dp)
										}
										return layout.Inset{Top: values.MarginPadding12}.Layout(gtx, asset.image.Layout48dp)
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													txt := pg.Theme.Label(values.TextSize16, asset.assetType.ToFull())
													txt.Color = pg.Theme.Color.Text
													return txt.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													txt := pg.Theme.Label(values.TextSize12, asset.assetType.String())
													txt.Color = pg.Theme.Color.GrayText3
													return layout.Inset{Left: values.MarginPadding4, Top: values.MarginPadding4}.Layout(gtx, txt.Layout)
												}),
											)
										})

									}),
									layout.Rigid(func(gtx C) D {
										gtx.Constraints.Min.X = gtx.Dp(50)
										separator := pg.Theme.Separator()
										return layout.E.Layout(gtx, func(gtx C) D {
											// Show bottom divider for all rows except last
											return layout.Inset{
												Left:   values.MarginPadding50,
												Right:  values.MarginPadding50,
												Top:    values.MarginPadding12,
												Bottom: values.MarginPadding8,
											}.Layout(gtx, separator.Layout)
										})
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
											txt := pg.Theme.Label(values.TextSize16, utils.FormatAsUSDString(pg.Printer, rate.LastTradePrice))
											txt.Color = pg.Theme.Color.Text
											return txt.Layout(gtx)
										})
									}),
									layout.Rigid(func(gtx C) D {
										card := pg.Theme.Card()
										card.Radius = cryptomaterial.Radius(12)
										card.Color = pg.Theme.Color.DefaultThemeColors().Gray3
										if isPositiveChange != nil {
											if *isPositiveChange {
												card.Color = pg.Theme.Color.DefaultThemeColors().Green50
											} else {
												card.Color = pg.Theme.Color.DefaultThemeColors().Orange3
											}
										}
										return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
											return pg.centerLayout(gtx, values.MarginPadding0, values.MarginPadding0, func(gtx C) D {
												return card.Layout(gtx, func(gtx C) D {
													return layout.Inset{
														Top:    values.MarginPadding4,
														Bottom: values.MarginPadding4,
														Right:  values.MarginPadding12,
														Left:   values.MarginPadding12,
													}.Layout(gtx, func(gtx C) D {
														lbl := pg.Theme.Body2(changeStr)
														lbl.Color = pg.Theme.Color.DefaultThemeColors().Gray1
														if isPositiveChange != nil {
															if *isPositiveChange {
																lbl.Color = pg.Theme.Color.DefaultThemeColors().Green500
															} else {
																lbl.Color = pg.Theme.Color.DefaultThemeColors().OrangeRipple
															}
														}
														return lbl.Layout(gtx)
													})
												})
											})
										})
									}),
								)
							})
						})
					})
				}),
			)
		}),
	)
}

func (pg *OverviewPage) marketRates() map[string]*ext.Ticker {
	marketRates := make(map[string]*ext.Ticker)

	if !components.IsFetchExchangeRateAPIAllowed(pg.WL) {
		return marketRates
	}

	for i := range pg.mktValues {
		asset := pg.mktValues[i]
		rate := pg.WL.AssetsManager.RateSource.GetTicker(asset.market)
		if rate == nil || rate.LastTradePrice <= 0 {
			continue
		}
		marketRates[asset.market] = rate
	}

	return marketRates
}

func (pg *OverviewPage) marketTableHeader(gtx C) D {
	col := pg.Theme.Color.GrayText3

	leftWidget := func(gtx C) D {
		return layout.Inset{
			Left: values.MarginPadding33,
		}.Layout(gtx, pg.assetTableLabel(values.String(values.StrName), col))
	}

	rightWidget := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Flexed(.8, func(gtx C) D {
				return layout.E.Layout(gtx, pg.assetTableLabel(values.String(values.StrPrice), col))
			}),
			layout.Flexed(.2, func(gtx C) D {
				return layout.E.Layout(gtx, pg.assetTableLabel(values.String(values.Str24HChange), col))
			}),
		)
	}
	return components.EndToEndRow(gtx, leftWidget, rightWidget)
}

func (pg *OverviewPage) marketTableRows(gtx C, asset assetMarketData, rate *ext.Ticker) D {
	leftWidget := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(asset.image.Layout24dp),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Left:  values.MarginPadding8,
					Right: values.MarginPadding4,
				}.Layout(gtx, pg.assetTableLabel(asset.assetType.ToFull(), pg.Theme.Color.Text))
			}),
			layout.Rigid(pg.assetTableLabel(asset.assetType.String(), pg.Theme.Color.GrayText3)),
		)
	}

	rightWidget := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Flexed(.785, func(gtx C) D {
				return layout.E.Layout(gtx, pg.assetTableLabel(utils.FormatAsUSDString(pg.Printer, rate.LastTradePrice), pg.Theme.Color.Text))
			}),
			layout.Flexed(.215, func(gtx C) D {
				hasRateChange := rate.PriceChangePercent != nil
				changeStr := "----"
				col := pg.Theme.Color.GrayText4
				if hasRateChange {
					change := *rate.PriceChangePercent
					if change < 0 {
						col = pg.Theme.Color.Danger
					}
					if change > 0 {
						col = pg.Theme.Color.Success
					}
					changeStr = fmt.Sprintf("%.2f", change) + "%"
				}

				return layout.E.Layout(gtx, pg.assetTableLabel(changeStr, col))
			}),
		)
	}
	return components.EndToEndRow(gtx, leftWidget, rightWidget)
}

func (pg *OverviewPage) assetTableLabel(title string, col color.NRGBA) layout.Widget {
	lbl := pg.Theme.Body1(title)
	lbl.Color = col
	return lbl.Layout
}

func (pg *OverviewPage) txStakingSection(gtx C) D {
	axis := layout.Horizontal
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		axis = layout.Vertical
	}

	txAndWallet := func(mtx *multiWalletTx) (*sharedW.Transaction, sharedW.Asset) {
		return mtx.Transaction, pg.WL.AssetsManager.WalletWithID(mtx.walletID)
	}

	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: axis,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {

			if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
				return layout.Flex{Axis: axis}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							return pg.pageContentWrapper(gtx, "Recent Transactions", nil, func(gtx C) D {
								return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									return pg.Theme.Body1("No recent transaction").Layout(gtx)
								})
							})
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							return pg.pageContentWrapper(gtx, "Staking Activity", nil, func(gtx C) D {
								return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X

									return pg.Theme.Body1("No recent Staking Activity").Layout(gtx)
								})
							})
						})
					}),
				)
			}

			return layout.Flex{}.Layout(gtx,
				layout.Flexed(.5, func(gtx C) D {
					return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
						return pg.pageContentWrapper(gtx, values.String(values.StrRecentTransactions), nil, func(gtx C) D {
							if len(pg.transactions) == 0 {
								return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									return pg.Theme.Body1(values.String(values.StrNoTransactions)).Layout(gtx)
								})
							}

							return pg.recentTransactions.Layout(gtx, len(pg.transactions), func(gtx C, index int) D {
								tx, wal := txAndWallet(pg.transactions[index])
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, false)
									}),
									layout.Rigid(func(gtx C) D {
										// No divider for last row
										if index == len(pg.transactions)-1 {
											return D{}
										}

										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										separator := pg.Theme.Separator()
										return layout.E.Layout(gtx, func(gtx C) D {
											// Show bottom divider for all rows except last
											return layout.Inset{Left: values.MarginPadding8}.Layout(gtx, separator.Layout)
										})
									}),
								)
							})
						})
					})
				}),
				layout.Flexed(.5, func(gtx C) D {
					return pg.pageContentWrapper(gtx, values.String(values.StrStakingActivity), nil, func(gtx C) D {
						if len(pg.stakes) == 0 {
							return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return pg.Theme.Body1(values.String(values.StrNoStaking)).Layout(gtx)
							})
						}

						return pg.recentStakes.Layout(gtx, len(pg.stakes), func(gtx C, index int) D {
							tx, wal := txAndWallet(pg.transactions[index])
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, false)
								}),
								layout.Rigid(func(gtx C) D {
									// No divider for last row
									if index == len(pg.stakes)-1 {
										return D{}
									}

									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									separator := pg.Theme.Separator()
									return layout.E.Layout(gtx, func(gtx C) D {
										// Show bottom divider for all rows except last
										return layout.Inset{Left: values.MarginPadding8}.Layout(gtx, separator.Layout)
									})
								}),
							)
						})
					})
				}),
			)
		}),
	)
}

func (pg *OverviewPage) recentTrades(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Trade", nil, func(gtx C) D {
		if len(pg.orders) == 0 {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
				return pg.Theme.Body1("No recent trades").Layout(gtx)
			})
		}

		return pg.recentTradeList.Layout(gtx, len(pg.orders), func(gtx C, i int) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return components.OrderItemWidget(gtx, pg.Load, pg.orders[i])
				}),
				layout.Rigid(func(gtx C) D {
					// Show bottom divider for all rows except the last row.
					if i == len(pg.orders)-1 {
						return layout.Dimensions{}
					}

					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Inset{Left: values.MarginPadding50}.Layout(gtx, pg.Theme.Separator().Layout)
					})
				}),
			)
		})
	})
}

func (pg *OverviewPage) recentProposal(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Proposals", nil, func(gtx C) D {
		if len(pg.proposalItems) == 0 {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
				return pg.Theme.Body1("No proposals").Layout(gtx)
			})
		}

		return pg.recentProposalList.Layout(gtx, len(pg.proposalItems), func(gtx C, i int) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return components.ProposalsList(pg.ParentWindow(), gtx, pg.Load, pg.proposalItems[i])
				}),
				layout.Rigid(func(gtx C) D {
					// No divider for last row
					if i == len(pg.proposalItems)-1 {
						return layout.Dimensions{}
					}
					return pg.Theme.Separator().Layout(gtx)
				}),
			)
		})
	})
}

func (pg *OverviewPage) updateAssetsUSDBalance() {
	if components.IsFetchExchangeRateAPIAllowed(pg.WL) {
		assetsTotalUSDBalance, err := components.CalculateAssetsUSDBalance(pg.Load, pg.assetsTotalBalance)
		if err != nil {
			log.Error(err)
			return
		}

		toUSDString := func(balance float64) string {
			return utils.FormatAsUSDString(pg.Printer, balance)
		}

		for assetType, balance := range assetsTotalUSDBalance {
			switch assetType {
			case libutils.DCRWalletAsset:
				pg.dcr.totalBalanceUSD = toUSDString(balance)
			case libutils.BTCWalletAsset:
				pg.btc.totalBalanceUSD = toUSDString(balance)
			case libutils.LTCWalletAsset:
				pg.ltc.totalBalanceUSD = toUSDString(balance)
			default:
				log.Errorf("Unsupported asset type: %s", assetType)
				return
			}
		}

		pg.assetBalanceSlider.RefreshItems()
		pg.ParentWindow().Reload()
	}
}

func (pg *OverviewPage) updateAssetsSliders() {
	assetsBalance, err := components.CalculateTotalAssetsBalance(pg.Load)
	if err != nil {
		log.Error(err)
		return
	}
	pg.assetsTotalBalance = assetsBalance

	sliderItem := func(totalBalance sharedW.AssetAmount, assetFullName string, icon, bkgImage *cryptomaterial.Image) *assetBalanceSliderItem {
		return &assetBalanceSliderItem{
			assetType:       assetFullName,
			totalBalance:    totalBalance,
			totalBalanceUSD: "$--",
			image:           icon,
			backgroundImage: bkgImage,
		}
	}

	for assetType, balance := range assetsBalance {
		assetFullName := strings.ToUpper(assetType.ToFull())

		switch assetType {
		case libutils.BTCWalletAsset:
			pg.btc = sliderItem(balance, assetFullName, pg.Theme.Icons.BTCGroupIcon, pg.Theme.Icons.BTCBackground)
		case libutils.DCRWalletAsset:
			pg.dcr = sliderItem(balance, assetFullName, pg.Theme.Icons.DCRGroupIcon, pg.Theme.Icons.DCRBackground)
		case libutils.LTCWalletAsset:
			pg.ltc = sliderItem(balance, assetFullName, pg.Theme.Icons.LTCGroupIcon, pg.Theme.Icons.LTCBackground)
		default:
			log.Errorf("Unsupported asset type: %s", assetType)
			return
		}
	}
}

func (pg *OverviewPage) pageContentWrapper(gtx C, sectionTitle string, altTitleLayout func(gtx C) D, body layout.Widget) D {
	titleLayout := pg.Theme.Body2(sectionTitle).Layout
	if altTitleLayout != nil {
		titleLayout = altTitleLayout
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(titleLayout),
		layout.Rigid(func(gtx C) D {
			r := 8
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Vertical,
				Padding:     layout.UniformInset(values.MarginPadding15),
				Margin: layout.Inset{
					Top:    values.MarginPadding8,
					Bottom: values.MarginPadding20,
				},
				Background: pg.Theme.Color.Surface,
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.CornerRadius{
						TopLeft:     r,
						TopRight:    r,
						BottomRight: r,
						BottomLeft:  r,
					},
				},
			}.Layout2(gtx, body)
		}),
	)
}

func (pg *OverviewPage) centerLayout(gtx C, top, bottom unit.Dp, content layout.Widget) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    top,
			Bottom: bottom,
		}.Layout(gtx, content)
	})
}

func (pg *OverviewPage) listenForMixerNotifications() {
	accountMixerNotificationListener := &dcr.AccountMixerNotificationListener{
		OnAccountMixerStarted: func(walletID int) {
			pg.setUnMixedBalance(walletID)
			pg.ParentWindow().Reload()
		},
		OnAccountMixerEnded: func(walletID int) {
			delete(pg.mixerSliderData, walletID)
			// Reload mixer slider items
			pg.mixerSlider.RefreshItems()
			pg.ParentWindow().Reload()
		},
	}

	// Reload wallets unmixed balance and reload UI on new blocks.
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnBlockAttached: func(walletID int, blockHeight int32) {
			pg.reloadBalances()
			pg.ParentWindow().Reload()
		},
	}

	wallets := pg.WL.AssetsManager.AllWallets()
	for _, wal := range wallets {
		if w, ok := wal.(*dcr.Asset); ok {
			// Only dcr wallets have mixing support currently.
			err := w.AddAccountMixerNotificationListener(accountMixerNotificationListener, OverviewPageID)
			if err != nil {
				log.Errorf("Error adding account mixer notification listener: %+v", err)
				continue
			}
		}

		err := wal.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, OverviewPageID)
		if err != nil {
			log.Errorf("Error adding tx and block notification listener: %v", err)
			continue
		}
	}

	// Reload the window whenever there is an exchange rate update.
	rateListener := &ext.RateListener{
		OnRateUpdated: pg.ParentWindow().Reload,
	}
	err := pg.WL.AssetsManager.RateSource.AddRateListener(rateListener, OverviewPageID)
	if err != nil {
		log.Error("RateSource.AddRateListener error: %v", err)
	}

	pg.sortedMixerSlideKeys = make([]int, 0)
	pg.mixerSliderData = make(map[int]*mixerData)
	for _, wal := range wallets {
		w, ok := wal.(*dcr.Asset)
		if !ok {
			continue
		}

		if w.IsAccountMixerActive() {
			if _, ok := pg.mixerSliderData[w.ID]; !ok {
				pg.mixerSliderData[w.ID] = &mixerData{
					Asset: w,
				}
				pg.setUnMixedBalance(w.ID)
				// Store the slide keys in a slice to maintain a consistent slide sequence.
				// since ranging over a map doesn't guarantee an order.
				pg.sortedMixerSlideKeys = append(pg.sortedMixerSlideKeys, w.ID)
			}
		}
	}
	// Sort the mixer slide keys so that the slides are drawn in the order of the wallets
	// on wallet list.
	sort.Ints(pg.sortedMixerSlideKeys)
	// Reload mixer slider items
	pg.mixerSlider.RefreshItems()
}

func (pg *OverviewPage) stopNtfnListeners() {
	wallets := pg.WL.AssetsManager.AllWallets()
	for _, wal := range wallets {
		if w, ok := wal.(*dcr.Asset); ok {
			w.RemoveAccountMixerNotificationListener(OverviewPageID)
		}
		wal.RemoveTxAndBlockNotificationListener(OverviewPageID)
	}
	pg.WL.AssetsManager.RateSource.RemoveRateListener(OverviewPageID)
}

func (pg *OverviewPage) setUnMixedBalance(id int) {
	mixerSliderData := pg.mixerSliderData[id]
	accounts, err := mixerSliderData.GetAccountsRaw()
	if err != nil {
		log.Errorf("error loading mixer account. %s", err)
		return
	}

	for _, acct := range accounts.Accounts {
		if acct.Number == mixerSliderData.UnmixedAccountNumber() {
			bal := acct.Balance.Total
			// to prevent NPE set default amount 0 if asset amount is nil
			if bal == nil {
				bal = dcr.Amount(dcrutil.Amount(0))
			}
			mixerSliderData.unmixedBalance = bal
		}
	}
}

func (pg *OverviewPage) reloadBalances() {
	for _, wal := range pg.mixerSliderData {
		accounts, _ := wal.GetAccountsRaw()
		for _, acct := range accounts.Accounts {
			if acct.Number == wal.UnmixedAccountNumber() {
				bal := acct.Balance.Total
				// to prevent NPE set default amount 0 if asset amount is nil
				if bal == nil {
					bal = dcr.Amount(dcrutil.Amount(0))
				}
				wal.unmixedBalance = bal
			}
		}
	}
}

func (pg *OverviewPage) ratesRefreshComponent() func(gtx C) D {
	return func(gtx layout.Context) layout.Dimensions {
		refreshing := pg.WL.AssetsManager.RateSource.Refreshing()
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var text string
				if refreshing {
					text = values.String(values.StrRefreshState)
				} else {
					lastUpdatedTimestamp := pg.WL.AssetsManager.RateSource.LastUpdate().Unix()
					text = values.String(values.StrUpdated) + " " + components.TimeAgo(lastUpdatedTimestamp)
				}
				lastUpdatedInfo := pg.Theme.Label(values.TextSize12, text)
				lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
				return layout.Inset{Bottom: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return cryptomaterial.LinearLayout{
					Width:     cryptomaterial.WrapContent,
					Height:    cryptomaterial.WrapContent,
					Direction: layout.E,
					Alignment: layout.End,
					Margin:    layout.Inset{Left: values.MarginPadding8},
					Clickable: pg.forceRefreshRates,
				}.Layout2(gtx, func(gtx layout.Context) layout.Dimensions {
					if refreshing {
						gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding20)
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Inset{Left: values.MarginPadding5, Bottom: values.MarginPadding2}.Layout(gtx, pg.materialLoader.Layout)
					}
					return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return pg.Theme.Icons.Restore.LayoutSize(gtx, values.MarginPadding18)
					})
				})
			}),
		)
	}
}

func (pg *OverviewPage) loadTransactions() {
	pg.transactions = make([]*multiWalletTx, 0)
	wal := pg.WL.AllSortedWalletList()
	for _, w := range wal {
		txs, err := w.GetTransactionsRaw(0, 3, libutils.TxFilterAllTx, true)
		if err != nil {
			log.Errorf("error loading transactions: %v", err)
			return
		}

		for _, tx := range txs {
			pg.transactions = append(pg.transactions, &multiWalletTx{tx, w.GetWalletID()})
		}
	}

	sort.Slice(pg.transactions, func(i, j int) bool {
		return pg.transactions[i].Timestamp > pg.transactions[j].Timestamp
	})

	if len(pg.transactions) > 3 {
		pg.transactions = pg.transactions[:3]
	}

	pg.loadStakes()
}

func (pg *OverviewPage) loadStakes() {
	pg.stakes = make([]*multiWalletTx, 0)
	wal := pg.WL.AssetsManager.AllDCRWallets()
	for _, w := range wal {
		txs, err := w.GetTransactionsRaw(0, 6, libutils.TxFilterStaking, true)
		if err != nil {
			log.Errorf("error loading staking activities: %v", err)
			return
		}
		for _, stakeTx := range txs {
			if (stakeTx.Type == dcr.TxTypeTicketPurchase) || (stakeTx.Type == dcr.TxTypeRevocation) {
				pg.stakes = append(pg.stakes, &multiWalletTx{stakeTx, w.GetWalletID()})
			}
		}
	}

	sort.Slice(pg.stakes, func(i, j int) bool {
		return pg.stakes[i].Timestamp > pg.stakes[j].Timestamp
	})

	if len(pg.stakes) > 3 {
		pg.stakes = pg.stakes[:3]
	}
}
