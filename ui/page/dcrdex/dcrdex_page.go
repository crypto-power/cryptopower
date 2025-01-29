package dcrdex

import (
	"decred.org/dcrdex/client/core"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
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

	openTradeMainPage    *cryptomaterial.Clickable
	splashPageInfoButton cryptomaterial.IconButton
	splashPageContainer  *widget.List
	startTradingBtn      cryptomaterial.Button
	createWalletBtn      cryptomaterial.Button
	showSplashPage       bool
	dexIsLoading         bool
	materialLoader       material.LoaderStyle
}

func NewDEXPage(l *load.Load) *DEXPage {
	dp := &DEXPage{
		MasterPage:        app.NewMasterPage(DCRDEXPageID),
		Load:              l,
		openTradeMainPage: l.Theme.NewClickable(false),
		startTradingBtn:   l.Theme.Button(values.String(values.StrStartTrading)),
		createWalletBtn:   l.Theme.Button(values.String(values.StrCreateANewWallet)),
		splashPageContainer: &widget.List{List: layout.List{
			Alignment: layout.Middle,
			Axis:      layout.Vertical,
		}},
		showSplashPage: true,
		materialLoader: material.Loader(l.Theme.Base),
	}

	if dp.AssetsManager.DEXCInitialized() && dp.AssetsManager.DexClient().InitializedWithPassword() {
		dp.showSplashPage = false
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
	if !pg.AssetsManager.DEXCInitialized() {
		return
	}

	if pg.CurrentPage() != nil {
		pg.CurrentPage().OnNavigatedTo()
	} else {
		pg.dexIsLoading = true
		go func() {
			pg.prepareInitialPage()
			pg.dexIsLoading = false
			pg.showSplashPage = false
		}()
	}
}

// prepareInitialPage starts a goroutine that waits for dexc to get ready before
// displaying an appropriate page.
func (pg *DEXPage) prepareInitialPage() {
	dexClient := pg.AssetsManager.DexClient()
	if dexClient == nil {
		return
	}

	<-dexClient.Ready()

	showOnBoardingPage := true
	if len(dexClient.Exchanges()) != 0 { // has at least one exchange
		_, _, pendingBond := pendingBondConfirmation(pg.AssetsManager, "")
		showOnBoardingPage = pendingBond != nil
	}

	if showOnBoardingPage {
		pg.Display(NewDEXOnboarding(pg.Load, "", nil))
	} else {
		pg.Display(NewDEXMarketPage(pg.Load, ""))
	}
}

// Layout draws the page UI components into the provided layout context to be
// eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXPage) Layout(gtx C) D {
	if !pg.AssetsManager.DEXCInitialized() || pg.CurrentPage() == nil { // dexc must have been reset.
		pg.showSplashPage = true
	}

	if pg.showSplashPage || pg.dexIsLoading {
		return pg.Theme.List(pg.splashPageContainer).Layout(gtx, 1, func(gtx C, _ int) D {
			return pg.splashPage(gtx)
		})
	}

	if hasMultipleWallets := pg.isMultipleAssetTypeWalletAvailable(); !hasMultipleWallets {
		return components.DisablePageWithOverlay(pg.Load, nil, gtx, values.String(values.StrMultipleAssetRequiredMsg), "", &pg.createWalletBtn)
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
func (pg *DEXPage) HandleUserInteractions(gtx C) {
	if pg.CurrentPage() != nil {
		pg.CurrentPage().HandleUserInteractions(gtx)
	}

	if pg.splashPageInfoButton.Button.Clicked(gtx) {
		pg.showInfoModal()
	}

	if pg.startTradingBtn.Button.Clicked(gtx) {
		if !pg.AssetsManager.DEXDBExists() && !pg.AssetsManager.DEXCInitialized() {
			// Attempt to initialize dex again.
			pg.dexIsLoading = true
			go func() {
				pg.AssetsManager.InitializeDEX()
				pg.prepareInitialPage()
				pg.dexIsLoading = false
				pg.showSplashPage = false
			}()
		}
	}

	if pg.createWalletBtn.Button.Clicked(gtx) {
		assetToCreate := pg.AssetsManager.AssetToCreate()
		pg.ParentWindow().Display(components.NewCreateWallet(pg.Load, func(_ sharedW.Asset) {
			pg.walletCreationSuccessFunc()
		}, assetToCreate))
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

// pendingBondConfirmation is a convenience function based on arbitrary
// heuristics to determine when to show bond confirmation step.
func pendingBondConfirmation(am *libwallet.AssetsManager, host string) (string, *core.BondAsset, *core.PendingBondState) {
	for _, xc := range am.DexClient().Exchanges() {
		if (host != "" && xc.Host != host) || len(xc.Auth.PendingBonds) == 0 {
			continue
		}

		for _, bond := range xc.Auth.PendingBonds {
			bondAsset := xc.BondAssets[bond.Symbol]
			if bond.Confs < bondAsset.Confs {
				return xc.Host, bondAsset, bond
			}
		}
	}
	return "", nil, nil
}

func (pg *DEXPage) walletCreationSuccessFunc() {
	pg.OnNavigatedTo()
	pg.ParentWindow().CloseCurrentPage()
	pg.ParentWindow().Reload()
}
