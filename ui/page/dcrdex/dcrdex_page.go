package dcrdex

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/values"
)

const DCRDEXPageID = "DCRDEXPageID"

type (
	C = layout.Context
	D = layout.Dimensions
)

type DEXPage struct {
	*app.MasterPage

	*load.Load

	generalSettingsBtn   cryptomaterial.Button
	openTradeMainPage    *cryptomaterial.Clickable
	splashPageInfoButton cryptomaterial.IconButton
	splashPageContainer  *widget.List
	startTradingBtn      cryptomaterial.Button
	isDexFirstVisit      bool
}

func NewDEXPage(l *load.Load) *DEXPage {
	dp := &DEXPage{
		MasterPage:        app.NewMasterPage(DCRDEXPageID),
		Load:              l,
		openTradeMainPage: l.Theme.NewClickable(false),
		startTradingBtn:   l.Theme.Button(values.String(values.StrStartTrading)),
		splashPageContainer: &widget.List{List: layout.List{
			Alignment: layout.Middle,
			Axis:      layout.Vertical,
		}},
		isDexFirstVisit:    true,
		generalSettingsBtn: l.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrExchange))),
	}

	// Init splash page more info widget.
	_, dp.splashPageInfoButton = components.SubpageHeaderButtons(l)
	return dp
}

// ID is a unique string that identifies the page and may be used to
// differentiate this page from other pages.
// Part of the load.Page interface.
func (pg *DEXPage) ID() string {
	return DCRDEXPageID
}

// OnNavigatedTo is called when the page is about to be displayed and may be
// used to initialize page features that are only relevant when the page is
// displayed.
// Part of the load.Page interface.
func (pg *DEXPage) OnNavigatedTo() {
	if !pg.AssetsManager.DexcReady() {
		return
	}

	if pg.CurrentPage() != nil {
		pg.CurrentPage().OnNavigatedTo()
	} else if len(pg.AssetsManager.DexClient().Exchanges()) > 0 {
		pg.Display(NewDEXMarketPage(pg.Load))
	} else {
		pg.Display(NewDEXOnboarding(pg.Load))
	}
}

// Layout draws the page UI components into the provided layout context to be
// eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXPage) Layout(gtx C) D {
	if pg.isDexFirstVisit {
		return pg.Theme.List(pg.splashPageContainer).Layout(gtx, 1, func(gtx C, i int) D {
			return pg.splashPage(gtx)
		})
	}

	hasMultipleWallets := pg.isMultipleAssetTypeWalletAvailable()
	privacyModeOff := pg.AssetsManager.IsHTTPAPIPrivacyModeOff(utils.ExchangeHTTPAPI)
	var msg string
	var actionBtn *cryptomaterial.Button
	if !privacyModeOff {
		actionBtn = &pg.generalSettingsBtn
		msg = values.StringF(values.StrNotAllowed, values.String(values.StrExchange))
	} else if !hasMultipleWallets {
		msg = values.String(values.StrMultipleAssetRequiredMsg)
	} else if pg.AssetsManager.DexClient() == nil {
		msg = values.String(values.StrDEXInitErrorMsg)
	}

	if msg != "" {
		return components.DisablePageWithOverlay(pg.Load, nil, gtx, msg, actionBtn)
	}

	return pg.CurrentPage().Layout(gtx)
}

// isMultipleAssetTypeWalletAvailable checks if wallets exist for more than 1
// asset type. If not, dex functionality is disable till different asset type
// wallets are created.
func (pg *DEXPage) isMultipleAssetTypeWalletAvailable() bool {
	allWallets := pg.AssetsManager.AllWallets()
	assetTypes := make(map[libutils.AssetType]bool)
	for _, wallet := range allWallets {
		assetTypes[wallet.GetAssetType()] = true
		if len(assetTypes) > 1 {
			return true
		}
	}
	return false
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *DEXPage) HandleUserInteractions() {
	if pg.generalSettingsBtn.Button.Clicked() {
		pg.ParentWindow().Display(settings.NewSettingsPage(pg.Load))
	}

	if pg.CurrentPage() != nil {
		pg.CurrentPage().HandleUserInteractions()
	}
	if pg.splashPageInfoButton.Button.Clicked() {
		pg.showInfoModal()
	}
	if pg.startTradingBtn.Button.Clicked() {
		pg.isDexFirstVisit = false
	}
}

// OnNavigatedFrom is called when the page is about to be removed from the
// displayed window. This method should ideally be used to disable features that
// are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *DEXPage) OnNavigatedFrom() {}
