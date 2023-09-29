package root

import (
	"context"
	"image/color"
	"sort"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/wallet"
	"github.com/decred/dcrd/dcrutil/v3"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
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

type OverviewPage struct {
	*app.GenericPageModal
	*load.Load

	ctx       context.Context
	ctxCancel context.CancelFunc

	pageContainer      layout.List
	marketOverviewList layout.List
	recentProposalList layout.List
	recentTradeList    layout.List

	scrollContainer *widget.List

	infoButton, forwardButton cryptomaterial.IconButton // TOD0: use *cryptomaterial.Clickable
	assetBalanceSlider        *cryptomaterial.Slider
	mixerSlider               *cryptomaterial.Slider
	proposalItems             []*components.ProposalItem
	orders                    []*instantswap.Order
	sliderRedirectBtn         *cryptomaterial.Clickable

	card cryptomaterial.Card

	dcr *assetBalanceSliderItem
	btc *assetBalanceSliderItem
	ltc *assetBalanceSliderItem

	assetsTotalBalance map[libutils.AssetType]sharedW.AssetAmount

	*listeners.AccountMixerNotificationListener
	*listeners.TxAndBlockNotificationListener
	mixerSliderData      map[int]*mixerData
	sortedMixerSlideKeys []int
}

type assetBalanceSliderItem struct {
	assetType       string
	totalBalance    sharedW.AssetAmount
	totalBalanceUSD string

	image           *cryptomaterial.Image
	backgroundImage *cryptomaterial.Image
}

type assetMarketData struct {
	title            string
	subText          string
	price            string
	idChange         string
	isChangePositive bool
	image            *cryptomaterial.Image
}

type mixerData struct {
	*dcr.Asset
	unmixedBalance sharedW.AssetAmount
}

func NewOverviewPage(l *load.Load) *OverviewPage {
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
		assetBalanceSlider: l.Theme.Slider(),
		card:               l.Theme.Card(),
		sliderRedirectBtn:  l.Theme.NewClickable(false),
	}

	pg.mixerSlider = l.Theme.Slider()
	pg.mixerSlider.ButtonBackgroundColor = values.TransparentColor(values.TransparentDeepBlue, 0.02)
	pg.mixerSlider.IndicatorBackgroundColor = values.TransparentColor(values.TransparentDeepBlue, 0.02)
	pg.mixerSlider.SelectedIndicatorColor = pg.Theme.Color.DeepBlue

	pg.forwardButton, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.forwardButton.Icon = pg.Theme.Icons.NavigationArrowForward
	pg.forwardButton.Size = values.MarginPadding20

	pg.assetsTotalBalance = make(map[libutils.AssetType]sharedW.AssetAmount)

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
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.updateSliders()
	go pg.fetchExchangeRate()

	pg.proposalItems = components.LoadProposals(pg.Load, libwallet.ProposalCategoryAll, 0, 3, true)
	pg.orders = components.LoadOrders(pg.Load, 0, 3, true)

	pg.listenForMixerNotifications()

}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *OverviewPage) HandleUserInteractions() {
	for pg.sliderRedirectBtn.Clicked() {
		pg.ParentNavigator().Display(NewWalletSelectorPage(pg.Load))
	}

	// Navigate to mixer page when wallet mixer slider forward button is clicked.
	if pg.forwardButton.Button.Clicked() {
		curSliderIndex := pg.mixerSlider.GetSelectedIndex()
		mixerData := pg.mixerSliderData[pg.sortedMixerSlideKeys[curSliderIndex]]
		pg.WL.SelectedWallet = &load.WalletItem{
			Wallet: mixerData.Asset,
		}

		mp := NewMainPage(pg.Load)
		pg.ParentNavigator().Display(mp)
		mp.Display(privacy.NewAccountMixerPage(pg.Load)) // Display mixer page on the main page.
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
	pg.ctxCancel()
}

func (pg *OverviewPage) OnCurrencyChanged() {
	go pg.fetchExchangeRate()
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

func (pg *OverviewPage) layoutMobile(_ C) D {
	return D{}
}

func (pg *OverviewPage) sliderLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
		Margin:      layout.Inset{Bottom: values.MarginPadding20},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// Only show mixer slider if mixer is running
			if len(pg.mixerSliderData) == 0 {
				return pg.assetBalanceSliderLayout(gtx)
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
		sliderWidget = append(sliderWidget, pg.assetBalanceItemLayout(*pg.dcr))
	}
	if pg.btc != nil {
		sliderWidget = append(sliderWidget, pg.assetBalanceItemLayout(*pg.btc))
	}
	if pg.ltc != nil {
		sliderWidget = append(sliderWidget, pg.assetBalanceItemLayout(*pg.ltc))
	}

	return pg.assetBalanceSlider.Layout(gtx, sliderWidget)
}

func (pg *OverviewPage) assetBalanceItemLayout(item assetBalanceSliderItem) layout.Widget {
	return func(gtx C) D {
		return pg.sliderRedirectBtn.Layout(gtx, func(gtx C) D {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					return item.backgroundImage.LayoutSize2(gtx, unit.Dp(gtx.Constraints.Max.X), values.MarginPadding221)
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
								return components.LayoutBalanceColor(gtx, pg.Load, item.totalBalance.String(), col)
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
										lbl := pg.Theme.Body2(item.totalBalanceUSD)
										lbl.Color = col
										return lbl.Layout(gtx)
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
		// Append the mixer slide widgets in an anonymouse function. This stops the
		// the fuction literal from capturing only the final key {key} value.
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
	return pg.pageContentWrapper(gtx, "Market Overview", func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(pg.marketTableHeader),
			layout.Rigid(func(gtx C) D {
				// TODO use real asset data
				mktValues := []assetMarketData{
					{
						title:            "Decred",
						subText:          "DCR",
						price:            "$1000",
						idChange:         "-0.56%",
						isChangePositive: false,
						image:            pg.Theme.Icons.DCR,
					},
					{
						title:            "Litecoin",
						subText:          "LTC",
						price:            "$100",
						idChange:         "0.56%",
						isChangePositive: true,
						image:            pg.Theme.Icons.LTC,
					},
					{
						title:            "Bitcoin",
						subText:          "BTC",
						price:            "$21000",
						idChange:         "-0.56%",
						isChangePositive: false,
						image:            pg.Theme.Icons.BTC,
					}}

				return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
					return pg.marketOverviewList.Layout(gtx, len(mktValues), func(gtx C, i int) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return pg.marketTableRows(gtx, mktValues[i])
							}),
							layout.Rigid(func(gtx C) D {
								// No divider for last row
								if i == len(mktValues)-1 {
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

func (pg *OverviewPage) marketTableHeader(gtx C) D {
	col := pg.Theme.Color.GrayText3

	leftWidget := func(gtx C) D {
		return layout.Inset{
			Left: values.MarginPadding33,
		}.Layout(gtx, pg.assetTableLabel("Name", col))
	}

	rightWidget := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Flexed(.8, func(gtx C) D {
				return layout.E.Layout(gtx, pg.assetTableLabel("Price", col))
			}),
			layout.Flexed(.2, func(gtx C) D {
				return layout.E.Layout(gtx, pg.assetTableLabel("ID Change", col))
			}),
		)
	}
	return components.EndToEndRow(gtx, leftWidget, rightWidget)
}

func (pg *OverviewPage) marketTableRows(gtx C, asset assetMarketData) D {
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
				}.Layout(gtx, pg.assetTableLabel(asset.title, pg.Theme.Color.Text))
			}),
			layout.Rigid(pg.assetTableLabel(asset.subText, pg.Theme.Color.GrayText3)),
		)
	}

	rightWidget := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Flexed(.785, func(gtx C) D {
				return layout.E.Layout(gtx, pg.assetTableLabel(asset.price, pg.Theme.Color.Text))
			}),
			layout.Flexed(.215, func(gtx C) D {
				col := pg.Theme.Color.Success
				if !asset.isChangePositive {
					col = pg.Theme.Color.Danger
				}
				return layout.E.Layout(gtx, pg.assetTableLabel(asset.idChange, col))
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
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Flexed(.5, func(gtx C) D {
			return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return pg.pageContentWrapper(gtx, "Recent Transactions", func(gtx C) D {
					return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return pg.Theme.Body1("No recent transaction").Layout(gtx)
					})
				})
			})
		}),
		layout.Flexed(.5, func(gtx C) D {
			return pg.pageContentWrapper(gtx, "Staking Activity", func(gtx C) D {
				return pg.centerLayout(gtx, values.MarginPadding10, values.MarginPadding10, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X

					return pg.Theme.Body1("No recent Staking Activity").Layout(gtx)
				})
			})
		}),
	)
}

func (pg *OverviewPage) recentTrades(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Trade", func(gtx C) D {
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
	return pg.pageContentWrapper(gtx, "Recent Proposals", func(gtx C) D {
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

func (pg *OverviewPage) calculateTotalAssetBalance() (map[libutils.AssetType]int64, error) {
	wallets := pg.WL.AssetsManager.AllWallets()
	assetsTotalBalance := make(map[libutils.AssetType]int64)

	for _, wal := range wallets {
		if wal.IsWatchingOnlyWallet() {
			continue
		}

		accountsResult, err := wal.GetAccountsRaw()
		if err != nil {
			return nil, err
		}

		for _, account := range accountsResult.Accounts {
			assetsTotalBalance[wal.GetAssetType()] += account.Balance.Total.ToInt()
		}
	}

	return assetsTotalBalance, nil
}

func (pg *OverviewPage) fetchExchangeRate() {
	if components.IsFetchExchangeRateAPIAllowed(pg.WL) {
		preferredExchange := pg.WL.AssetsManager.GetCurrencyConversionExchange()

		usdBalance := func(bal sharedW.AssetAmount, market string) string {
			rate, err := pg.WL.AssetsManager.ExternalService.GetTicker(preferredExchange, market)
			if err != nil {
				log.Error(err)
				return "$--"
			}

			balanceInUSD := bal.MulF64(rate.LastTradePrice).ToCoin()
			return utils.FormatUSDBalance(pg.Printer, balanceInUSD)
		}

		for assetType, balance := range pg.assetsTotalBalance {
			switch assetType {
			case libutils.DCRWalletAsset:
				pg.dcr.totalBalanceUSD = usdBalance(balance, values.DCRUSDTMarket)
			case libutils.BTCWalletAsset:
				pg.btc.totalBalanceUSD = usdBalance(balance, values.BTCUSDTMarket)
			case libutils.LTCWalletAsset:
				pg.ltc.totalBalanceUSD = usdBalance(balance, values.LTCUSDTMarket)
			default:
				log.Errorf("Unsupported asset type: %s", assetType)
				return
			}
		}

		pg.assetBalanceSlider.RefreshItems()
		pg.ParentWindow().Reload()
	}
}

func (pg *OverviewPage) updateSliders() {
	assetItems, err := pg.calculateTotalAssetBalance()
	if err != nil {
		log.Error(err)
		return
	}

	sliderItem := func(totalBalance sharedW.AssetAmount, assetFullName string, icon, bkgImage *cryptomaterial.Image) *assetBalanceSliderItem {
		return &assetBalanceSliderItem{
			assetType:       assetFullName,
			totalBalance:    totalBalance,
			totalBalanceUSD: "$--",
			image:           icon,
			backgroundImage: bkgImage,
		}
	}

	for assetType, totalBalance := range assetItems {
		assetFullName := strings.ToUpper(assetType.ToFull())

		switch assetType {
		case libutils.BTCWalletAsset:
			balance := pg.WL.AssetsManager.AllBTCWallets()[0].ToAmount(totalBalance)
			pg.btc = sliderItem(balance, assetFullName, pg.Theme.Icons.BTCGroupIcon, pg.Theme.Icons.BTCBackground)
			pg.assetsTotalBalance[assetType] = balance
		case libutils.DCRWalletAsset:
			balance := pg.WL.AssetsManager.AllDCRWallets()[0].ToAmount(totalBalance)
			pg.dcr = sliderItem(balance, assetFullName, pg.Theme.Icons.DCRGroupIcon, pg.Theme.Icons.DCRBackground)
			pg.assetsTotalBalance[assetType] = balance
		case libutils.LTCWalletAsset:
			balance := pg.WL.AssetsManager.AllLTCWallets()[0].ToAmount(totalBalance)
			pg.ltc = sliderItem(balance, assetFullName, pg.Theme.Icons.LTCGroupIcon, pg.Theme.Icons.LTCBackground)
			pg.assetsTotalBalance[assetType] = balance
		default:
			log.Errorf("Unsupported asset type: %s", assetType)
			return
		}
	}
}

func (pg *OverviewPage) pageContentWrapper(gtx C, sectionTitle string, body layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(pg.Theme.Body2(sectionTitle).Layout),
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
	// Get all DCR wallets, and subscribe to the individual wallet's mixer channel.
	// We are only interested in DCR wallets since mixing support
	// is limited to DCR at this point.
	dcrWallets := pg.WL.AssetsManager.AllDCRWallets()

	if pg.AccountMixerNotificationListener == nil {
		pg.AccountMixerNotificationListener = listeners.NewAccountMixerNotificationListener()
		pg.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
		for _, wal := range dcrWallets {
			w := wal.(*dcr.Asset)
			if w == nil {
				log.Warn(values.ErrDCRSupportedOnly)
				continue
			}
			err := w.AddAccountMixerNotificationListener(pg, OverviewPageID)
			if err != nil {
				log.Errorf("Error adding account mixer notification listener: %+v", err)
				continue
			}

			err = w.AddTxAndBlockNotificationListener(pg, true, OverviewPageID)
			if err != nil {
				log.Errorf("Error adding tx and block notification listener: %v", err)
				continue
			}

		}
	}

	pg.sortedMixerSlideKeys = make([]int, 0)
	pg.mixerSliderData = make(map[int]*mixerData)
	for _, wal := range dcrWallets {
		w := wal.(*dcr.Asset)

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

	go func() {
		for {
			select {
			case n := <-pg.MixerChan:
				if n.RunStatus == wallet.MixerStarted {
					pg.setUnMixedBalance(n.WalletID)
					pg.ParentWindow().Reload()
				}

				if n.RunStatus == wallet.MixerEnded {
					delete(pg.mixerSliderData, n.WalletID)
					// Reload mixer slider items
					pg.mixerSlider.RefreshItems()
					pg.ParentWindow().Reload()
				}
			case n := <-pg.TxAndBlockNotifChan():
				// Reload wallets unmixed balance and reload UI on
				// new blocks.
				if n.Type == listeners.BlockAttached {
					go func() {
						pg.reloadBalances()
						pg.ParentWindow().Reload()
					}()
				}
			case <-pg.ctx.Done():
				for _, wal := range dcrWallets {
					w := wal.(*dcr.Asset)
					w.RemoveAccountMixerNotificationListener(OverviewPageID)
					w.RemoveTxAndBlockNotificationListener(OverviewPageID)
				}
				close(pg.MixerChan)
				pg.CloseTxAndBlockChan()
				pg.AccountMixerNotificationListener = nil
				pg.TxAndBlockNotificationListener = nil

				return
			}
		}
	}()
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
