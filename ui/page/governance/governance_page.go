package governance

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/values"
)

const GovernancePageID = "Governance"

type Page struct {
	*load.Load
	*app.MasterPage

	modal *cryptomaterial.Modal

	// selectedTabIdx int
	tab *cryptomaterial.SegmentedControl

	tabCategoryList        *cryptomaterial.ClickableList
	splashScreenInfoButton cryptomaterial.IconButton
	enableGovernanceBtn    cryptomaterial.Button
	navigateToSettingsBtn  cryptomaterial.Button
}

var governanceTabTitles = []string{
	values.String(values.StrProposal),
	values.String(values.StrConsensusChange),
	values.String(values.StrTreasurySpending),
}

func NewGovernancePage(l *load.Load) *Page {
	pg := &Page{
		Load:            l,
		MasterPage:      app.NewMasterPage(GovernancePageID),
		modal:           l.Theme.ModalFloatTitle(values.String(values.StrSettings)),
		tabCategoryList: l.Theme.NewClickableList(layout.Horizontal),
	}

	pg.tab = l.Theme.SegmentedControl(governanceTabTitles)

	pg.tabCategoryList.IsHoverable = false

	pg.initSplashScreenWidgets()
	pg.navigateToSettingsBtn = pg.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrGovernance)))

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.OnNavigatedTo()
	} else {
		pg.Display(NewProposalsPage(pg.Load))
	}
}

func (pg *Page) isGovernanceAPIAllowed() bool {
	return pg.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (pg *Page) sectionNavTab(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.tab.Layout)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.OnNavigatedFrom()
	}
}

func (pg *Page) HandleUserInteractions() {
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.HandleUserInteractions()
	}

	if pg.navigateToSettingsBtn.Button.Clicked() {
		pg.ParentWindow().Display(settings.NewSettingsPage(pg.Load))
	}

	if pg.splashScreenInfoButton.Button.Clicked() {
		pg.showInfoModal()
	}

	if tabItemClicked, clickedTabIndex := pg.tabCategoryList.ItemClicked(); tabItemClicked {
		if clickedTabIndex == 0 {
			pg.Display(NewProposalsPage(pg.Load)) // Display should do nothing if the page is already displayed.
		} else if clickedTabIndex == 1 {
			pg.Display(NewConsensusPage(pg.Load))
		} else {
			pg.Display(NewTreasuryPage(pg.Load))
		}
	}

	// Handle individual page user interactions.
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.HandleUserInteractions()
	}

	if pg.tab.Changed() {
		selectedTabIdx := pg.tab.SelectedIndex()
		if selectedTabIdx == 0 {
			pg.Display(NewProposalsPage(pg.Load)) // Display should do nothing if the page is already displayed.
		} else if selectedTabIdx == 1 {
			pg.Display(NewConsensusPage(pg.Load))
		} else {
			pg.Display(NewTreasuryPage(pg.Load))
		}
	}
}

func (pg *Page) Layout(gtx C) D {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *Page) layoutDesktop(gtx layout.Context) layout.Dimensions {
	if !pg.isGovernanceAPIAllowed() {
		return components.UniformPadding(gtx, pg.splashScreen)
	}

	return components.UniformPadding(gtx, func(gtx C) D {
		proposalListView := layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				// layout.Rigid(pg.layoutPageTopNav),
				// layout.Rigid(pg.Theme.Separator().Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
						return pg.CurrentPage().Layout(gtx)
					})
				}),
			)
		})

		items := []layout.FlexChild{}
		items = append(items, layout.Rigid(pg.sectionNavTab))
		items = append(items, proposalListView)
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, items...)
	})
}

func (pg *Page) layoutMobile(gtx layout.Context) layout.Dimensions {
	return components.UniformMobile(gtx, false, true, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.layoutPageTopNav),
			layout.Rigid(pg.layoutTabs),
			layout.Rigid(pg.Theme.Separator().Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.CurrentPage().Layout(gtx)
				})
			}),
		)
	})
}

func (pg *Page) selectedTabIndex() int {
	switch pg.CurrentPageID() {
	case ProposalsPageID:
		return 0
	case ConsensusPageID:
		return 1
	case TreasuryPageID:
		return 2
	default:
		return -1
	}
}

func (pg *Page) layoutTabs(gtx C) D {
	var selectedTabDims layout.Dimensions

	return layout.Inset{
		Top: values.MarginPadding20,
	}.Layout(gtx, func(gtx C) D {
		return pg.tabCategoryList.Layout(gtx, len(governanceTabTitles), func(gtx C, i int) D {
			isSelectedTab := pg.selectedTabIndex() == i
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					return layout.Inset{
						Right:  values.MarginPadding24,
						Bottom: values.MarginPadding8,
					}.Layout(gtx, func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							lbl := pg.Theme.Label(values.TextSize16, governanceTabTitles[i])
							lbl.Color = pg.Theme.Color.GrayText1
							if isSelectedTab {
								lbl.Color = pg.Theme.Color.Primary
								selectedTabDims = lbl.Layout(gtx)
							}

							return lbl.Layout(gtx)
						})
					})
				}),
				layout.Stacked(func(gtx C) D {
					if !isSelectedTab {
						return D{}
					}

					tabHeight := gtx.Dp(values.MarginPadding2)
					tabRect := image.Rect(0, 0, selectedTabDims.Size.X, tabHeight)

					return layout.Inset{
						Left: values.MarginPaddingMinus22,
					}.Layout(gtx, func(gtx C) D {
						paint.FillShape(gtx.Ops, pg.Theme.Color.Primary, clip.Rect(tabRect).Op())
						return layout.Dimensions{
							Size: image.Point{X: selectedTabDims.Size.X, Y: tabHeight},
						}
					})
				}),
			)
		})
	})
}

func (pg *Page) layoutPageTopNav(gtx C) D {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// layout.Rigid(pg.Theme.Icons.GovernanceActiveIcon.Layout24dp),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Left: values.MarginPadding20,
			}.Layout(gtx, func(gtx C) D {
				txt := pg.Theme.Label(values.TextSize20, values.String(values.StrGovernance))
				txt.Font.Weight = font.SemiBold
				return txt.Layout(gtx)
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return D{}
				// TODO: governance syncing functionality.
				// TODO: Split wallet sync from governance
			})
		}),
	)
}
