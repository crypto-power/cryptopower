package dcrdex

import (
	"decred.org/dcrdex/client/core"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
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

	switchToTestnetBtn   cryptomaterial.Button
	openTradeMainPage    *cryptomaterial.Clickable
	splashPageInfoButton cryptomaterial.IconButton
	splashPageContainer  *widget.List
	startTradingBtn      cryptomaterial.Button
	isDexFirstVisit      bool
	dexIsLoading         bool
	materialLoader       material.LoaderStyle
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
		switchToTestnetBtn: l.Theme.Button(values.String(values.StrSwitchToTestnet)),
		materialLoader:     material.Loader(l.Theme.Base),
	}

	if dp.AssetsManager.DEXCInitialized() && dp.AssetsManager.DexClient().InitializedWithPassword() {
		dp.isDexFirstVisit = false
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
		return
	}

	pg.dexIsLoading = true
	go func() {
		dexc := pg.AssetsManager.DexClient()
		<-dexc.Ready()

		showOnBoardingPage := true
		if len(dexc.Exchanges()) != 0 { // has at least one exchange
			_, _, pendingBond := pendingBondConfirmation(pg.AssetsManager, "")
			showOnBoardingPage = pendingBond != nil
		}

		if showOnBoardingPage {
			pg.Display(NewDEXOnboarding(pg.Load, ""))
		} else {
			pg.Display(NewDEXMarketPage(pg.Load, ""))
		}

		pg.dexIsLoading = false
	}()
}

// Layout draws the page UI components into the provided layout context to be
// eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXPage) Layout(gtx C) D {
	if pg.isDexFirstVisit || pg.dexIsLoading {
		return pg.Theme.List(pg.splashPageContainer).Layout(gtx, 1, func(gtx C, i int) D {
			return pg.splashPage(gtx)
		})
	}

	hasMultipleWallets := pg.isMultipleAssetTypeWalletAvailable()
	isMainnet := pg.AssetsManager.NetType() == utils.Mainnet
	var msg string
	var actionBtn *cryptomaterial.Button
	if isMainnet {
		if pg.CanChangeNetworkType() {
			actionBtn = &pg.switchToTestnetBtn
		}
		msg = values.String(values.StrDexMainnetNotReady)
	} else if !hasMultipleWallets {
		msg = values.String(values.StrMultipleAssetRequiredMsg)
	} else if !pg.AssetsManager.DEXCInitialized() || pg.CurrentPage() == nil {
		msg = values.String(values.StrDEXInitErrorMsg)
	}

	if msg != "" {
		return components.DisablePageWithOverlay(pg.Load, nil, gtx, msg, "", actionBtn)
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
	if pg.switchToTestnetBtn.Button.Clicked() {
		settings.ChangeNetworkType(pg.Load, pg.ParentWindow(), string(libutils.Testnet))
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

// pendingBondConfirmation is a convenience function based on arbitrary
// heuristics to determine when to show bond confirmation step.
func pendingBondConfirmation(am *libwallet.AssetsManager, host string) (*core.Exchange, *core.BondAsset, *core.PendingBondState) {
	for _, xc := range am.DexClient().Exchanges() {
		if (host != "" && xc.Host != host) || len(xc.Auth.PendingBonds) == 0 {
			continue
		}

		for _, bond := range xc.Auth.PendingBonds {
			bondAsset := xc.BondAssets[bond.Symbol]
			if bond.Confs < bondAsset.Confs {
				return xc, bondAsset, bond
			}
		}
	}
	return nil, nil, nil
}
