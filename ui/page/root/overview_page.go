package root

import (
	"context"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
	// "github.com/crypto-power/cryptopower/ui/page/components"
)

const (
	OverviewPageID = "Overview"
)

type OverviewPage struct {
	*app.GenericPageModal
	*load.Load

	ctx       context.Context
	ctxCancel context.CancelFunc

	slider          *cryptomaterial.Slider
	pageContainer   layout.List
	scrollContainer *widget.List
}

func NewOverviewPage(l *load.Load) *OverviewPage {
	pg := &OverviewPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OverviewPageID),
		pageContainer: layout.List{
			Axis: layout.Vertical,
		},
		scrollContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}

	sliderItems := []cryptomaterial.SliderItem{
		{
			Title:           "DECRED",
			MainText:        "20000.199",
			SubText:         "$1000",
			Image:           pg.Theme.Icons.DCRGroupIcon,
			BackgroundImage: pg.Theme.Icons.DCRBackground,
		},
		{
			Title:           "Litecoin",
			MainText:        "50000.199",
			SubText:         "$9000",
			Image:           pg.Theme.Icons.LTGroupIcon,
			BackgroundImage: pg.Theme.Icons.LTBackground,
		},
		{
			Title:           "Bitcoin",
			MainText:        "100000.199",
			SubText:         "$89000",
			Image:           pg.Theme.Icons.BTCGroupIcon,
			BackgroundImage: pg.Theme.Icons.BTCBackground,
		},
	}

	pg.slider = l.Theme.Slider(sliderItems, values.MarginPadding368, values.MarginPadding221)

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
		pg.topSection,
		pg.marketOverview,
		pg.txStakingSection,
		pg.recentTrades,
		pg.recentProposal,
	}

	return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
		return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
			return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
				return pageContent[i](gtx)
			})
		})
	})
}

func (pg *OverviewPage) layoutMobile(gtx C) D {
	return D{}
}

func (pg *OverviewPage) topSection(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
	}.Layout(gtx,
		layout.Rigid(pg.slider.Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Left: values.MarginPadding20,
			}.Layout(gtx, func(gtx C) D {
				return pg.slider.Layout(gtx)
			})
		}),
	)
}

func (pg *OverviewPage) marketOverview(gtx C) D {
	return pg.pageContentWrapper(gtx, "Market Overview", func(gtx C) D {
		return pg.slider.Layout(gtx)
	})
}

func (pg *OverviewPage) txStakingSection(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.pageContentWrapper(gtx, "Recent Proposals", func(gtx C) D {
				return pg.slider.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return pg.pageContentWrapper(gtx, "Staking Activity", func(gtx C) D {
				return pg.slider.Layout(gtx)
			})
		}),
	)
}

func (pg *OverviewPage) recentTrades(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Trade", func(gtx C) D {
		return pg.slider.Layout(gtx)
	})
}

func (pg *OverviewPage) recentProposal(gtx C) D {
	return pg.pageContentWrapper(gtx, "Recent Proposals", func(gtx C) D {
		return pg.slider.Layout(gtx)
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
