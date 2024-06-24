package governance

import (
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/values"
)

const GovernancePageID = "Governance"

type Page struct {
	*load.Load
	*app.MasterPage

	modal *cryptomaterial.Modal

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
		modal:           l.Theme.ModalFloatTitle(values.String(values.StrSettings), l.IsMobileView()),
		tabCategoryList: l.Theme.NewClickableList(layout.Horizontal),
	}

	pg.tab = l.Theme.SegmentedControl(governanceTabTitles, cryptomaterial.SegmentTypeGroup)

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
	return pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
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

func (pg *Page) HandleUserInteractions(gtx C) {
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.HandleUserInteractions(gtx)
	}

	if pg.navigateToSettingsBtn.Button.Clicked(gtx) {
		pg.ParentWindow().Display(settings.NewAppSettingsPage(pg.Load))
	}

	if pg.splashScreenInfoButton.Button.Clicked(gtx) {
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
		activeTab.HandleUserInteractions(gtx)
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
	if !pg.isGovernanceAPIAllowed() {
		return cryptomaterial.UniformPadding(gtx, pg.splashScreen)
	}
	return pg.tab.Layout(gtx, pg.CurrentPage().Layout, pg.IsMobileView())
}
