package root

import (
	"context"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
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

	pageContainer   layout.List
	scrollContainer *widget.List

	infoButton, forwardButton cryptomaterial.IconButton // TOD0: use *cryptomaterial.Clickable
	coinSlider                *cryptomaterial.Slider
	mixerSlider               *cryptomaterial.Slider

	card cryptomaterial.Card
}

type supportedCoinSliderItem struct {
	Title    string
	MainText string
	SubText  string

	Image           *cryptomaterial.Image
	BackgroundImage *cryptomaterial.Image
}

func NewOverviewPage(l *load.Load) *OverviewPage {
	pg := &OverviewPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OverviewPageID),
		pageContainer: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		coinSlider: l.Theme.Slider(),
		card:       l.Theme.Card(),
	}

	pg.mixerSlider = l.Theme.Slider()
	pg.mixerSlider.ButtonBackgroundColor = values.TransparentColor(values.TransparentDeepBlue, 0.02)
	pg.mixerSlider.IndicatorBackgroundColor = values.TransparentColor(values.TransparentDeepBlue, 0.02)
	pg.mixerSlider.SelectedIndicatorColor = pg.Theme.Color.DeepBlue

	pg.forwardButton, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.forwardButton.Icon = pg.Theme.Icons.NavigationArrowForward
	pg.forwardButton.Size = values.MarginPadding20

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
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *OverviewPage) HandleUserInteractions() {

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
	m10 := values.MarginPadding10
	m20 := values.MarginPadding20
	return layout.Inset{
		Right:  m20,
		Left:   m20,
		Top:    m10,
		Bottom: m10,
	}.Layout(gtx, func(gtx C) D {
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
	return D{}
}

func (pg *OverviewPage) sliderLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Rigid(pg.supportedCoinSliderLayout),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, pg.mixerSliderLayout)
		}),
	)
}

func (pg *OverviewPage) supportedCoinSliderLayout(gtx C) D {
	// TODO use real data
	dcr := supportedCoinSliderItem{
		Title:           "DECRED",
		MainText:        "20000.199 DCR",
		SubText:         "$1000",
		Image:           pg.Theme.Icons.DCRGroupIcon,
		BackgroundImage: pg.Theme.Icons.DCRBackground,
	}
	ltc := supportedCoinSliderItem{
		Title:           "Litecoin",
		MainText:        "50000.199 LTC",
		SubText:         "$9000",
		Image:           pg.Theme.Icons.LTGroupIcon,
		BackgroundImage: pg.Theme.Icons.LTBackground,
	}
	btc := supportedCoinSliderItem{
		Title:           "Bitcoin",
		MainText:        "100000.199 BTC",
		SubText:         "$89000",
		Image:           pg.Theme.Icons.BTCGroupIcon,
		BackgroundImage: pg.Theme.Icons.BTCBackground,
	}

	sliderWidget := []layout.Widget{
		func(gtx C) D {
			return pg.supportedCoinItemLayout(gtx, dcr)
		},
		func(gtx C) D {
			return pg.supportedCoinItemLayout(gtx, ltc)
		},
		func(gtx C) D {
			return pg.supportedCoinItemLayout(gtx, btc)
		},
	}
	return pg.coinSlider.Layout(gtx, sliderWidget)
}

func (pg *OverviewPage) mixerSliderLayout(gtx C) D {
	sliderWidget := []layout.Widget{
		pg.mixerLayout,
	}
	return pg.mixerSlider.Layout(gtx, sliderWidget)
}

func (pg *OverviewPage) supportedCoinItemLayout(gtx C, item supportedCoinSliderItem) D {
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			return item.BackgroundImage.LayoutSize2(gtx, values.MarginPadding368, values.MarginPadding221)
		}),
		layout.Expanded(func(gtx C) D {
			col := pg.Theme.Color.InvText
			return layout.Flex{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					lbl := pg.Theme.Body1(item.Title)
					lbl.Color = col
					return pg.centerLayout(gtx, values.MarginPadding15, values.MarginPadding10, lbl.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.centerLayout(gtx, values.MarginPadding0, values.MarginPadding10, func(gtx C) D {
						return item.Image.LayoutSize(gtx, values.MarginPadding65)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return pg.centerLayout(gtx, values.MarginPadding0, values.MarginPadding10, func(gtx C) D {
						return components.LayoutBalanceColor(gtx, pg.Load, item.MainText, col)
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
								lbl := pg.Theme.Body2(item.SubText)
								lbl.Color = col
								return lbl.Layout(gtx)
							})
						})
					})
				}),
			)
		}),
	)
}

func (pg *OverviewPage) mixerLayout(gtx C) D {
	r := 8
	return cryptomaterial.LinearLayout{
		Width:       gtx.Dp(values.MarginPadding368),
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
		layout.Rigid(pg.bottomMixerLayout),
	)
}

func (pg *OverviewPage) topMixerLayout(gtx C) D {
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(pg.Theme.Icons.Mixer.Layout24dp),
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.Body1("Mixer is Running...") // TODO
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
			lbl := pg.Theme.Body2("Keep app open")
			return layout.Inset{Left: values.MarginPadding6}.Layout(gtx, lbl.Layout)
		}),
	)
}

func (pg *OverviewPage) bottomMixerLayout(gtx C) D {
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
			lbl := pg.Theme.Body2("myWallet")
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(pg.Theme.Body1("Unmixed balance").Layout), // TODO
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return components.LayoutBalance(gtx, pg.Load, "100.67 DCR")
					})
				}),
			)
		}),
	)
}

func (pg *OverviewPage) marketOverview(gtx C) D {
	return pg.pageContentWrapper(gtx, "Market Overview", func(gtx C) D {
		return pg.supportedCoinSliderLayout(gtx)
	})
}

func (pg *OverviewPage) txStakingSection(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.pageContentWrapper(gtx, "Recent Proposals", func(gtx C) D {
				return pg.supportedCoinSliderLayout(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return pg.pageContentWrapper(gtx, "Staking Activity", func(gtx C) D {
				return pg.supportedCoinSliderLayout(gtx)
			})
		}),
	)
}

func (pg *OverviewPage) recentTrades(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Trade", func(gtx C) D {
		return pg.supportedCoinSliderLayout(gtx)
	})
}

func (pg *OverviewPage) recentProposal(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Proposals", func(gtx C) D {
		return pg.supportedCoinSliderLayout(gtx)
	})
}

func (pg *OverviewPage) pageContentWrapper(gtx C, sectionTitle string, body layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.Theme.Body2(sectionTitle).Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.Theme.Card().Layout(gtx, body)
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
