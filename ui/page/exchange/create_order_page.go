package exchange

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/crypto-power/cryptopower/wallet"

	api "github.com/crypto-power/instantswap/instantswap"
)

const CreateOrderPageID = "CreateOrder"

type (
	C = layout.Context
	D = layout.Dimensions
)

type CreateOrderPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	*listeners.OrderNotificationListener
	*ext.RateListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	scroll           *components.Scroll
	ordersList       *cryptomaterial.ClickableList
	exchangeSelector *ExSelector
	selectedExchange *Exchange

	exchangeRateInfo string
	amountErrorText  string
	fetchingRate     bool
	rateError        bool
	inited           bool

	materialLoader material.LoaderStyle

	fromAmountEditor components.SelectAssetEditor
	toAmountEditor   components.SelectAssetEditor

	backButton cryptomaterial.IconButton

	createOrderBtn         cryptomaterial.Button
	swapButton             cryptomaterial.IconButton
	refreshExchangeRateBtn cryptomaterial.IconButton
	infoButton             cryptomaterial.IconButton
	settingsButton         cryptomaterial.IconButton
	iconClickable          *cryptomaterial.Clickable
	refreshClickable       *cryptomaterial.Clickable
	refreshIcon            *cryptomaterial.Image
	viewAllButton          cryptomaterial.Button
	navToSettingsBtn       cryptomaterial.Button

	min          float64
	max          float64
	exchangeRate float64

	errMsg string

	*orderData
}

type orderData struct {
	exchange       api.IDExchange
	exchangeServer instantswap.ExchangeServer

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	sourceWalletID           int
	sourceAccountNumber      int32
	destinationWalletID      int
	destinationAccountNumber int32

	invoicedAmount float64
	orderedAmount  float64

	fromCurrency libutils.AssetType
	toCurrency   libutils.AssetType

	refundAddress      string
	destinationAddress string

	scheduler *cryptomaterial.Switch
}

func NewCreateOrderPage(l *load.Load) *CreateOrderPage {
	pg := &CreateOrderPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(CreateOrderPageID),
		exchangeSelector: NewExSelector(l),
		orderData:        &orderData{},
		exchangeRate:     -1,
		refreshClickable: l.Theme.NewClickable(true),
		iconClickable:    l.Theme.NewClickable(true),
		refreshIcon:      l.Theme.Icons.Restore,
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	// pageSize defines the number of orders that can be fetched at ago.
	pageSize := int32(5)
	pg.scroll = components.NewScroll(l, pageSize, pg.fetchOrders)

	pg.scheduler = pg.Theme.Switch()
	pg.swapButton = l.Theme.IconButton(l.Theme.Icons.ActionSwapHoriz)
	pg.refreshExchangeRateBtn = l.Theme.IconButton(l.Theme.Icons.NavigationRefresh)
	pg.refreshExchangeRateBtn.Size = values.MarginPadding18

	pg.settingsButton = l.Theme.IconButton(l.Theme.Icons.ActionSettings)

	pg.viewAllButton = l.Theme.Button(values.String(values.StrViewAllOrders))
	pg.viewAllButton.Font.Weight = font.SemiBold
	pg.viewAllButton.Color = l.Theme.Color.Primary
	pg.viewAllButton.Inset = layout.UniformInset(values.MarginPadding4)
	pg.viewAllButton.Background = l.Theme.Color.DefaultThemeColors().SurfaceHighlight
	pg.viewAllButton.HighlightColor = cryptomaterial.GenHighlightColor(l.Theme.Color.GrayText4)

	pg.infoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	pg.infoButton.Size = values.MarginPadding18
	buttonInset := layout.UniformInset(values.MarginPadding0)
	pg.settingsButton.Inset, pg.infoButton.Inset,
		pg.swapButton.Inset, pg.refreshExchangeRateBtn.Inset = buttonInset, buttonInset, buttonInset, buttonInset

	pg.exchangeRateInfo = fmt.Sprintf(values.String(values.StrMinMax), pg.min, pg.max)
	pg.materialLoader = material.Loader(l.Theme.Base)

	pg.ordersList = pg.Theme.NewClickableList(layout.Vertical)
	pg.ordersList.IsShadowEnabled = true

	pg.toAmountEditor = *components.NewSelectAssetEditor(l)
	pg.fromAmountEditor = *components.NewSelectAssetEditor(l)

	pg.fromAmountEditor.AssetTypeSelector.AssetTypeSelected(func(ati *components.AssetTypeItem) {
		isMatching := pg.fromCurrency == pg.toCurrency && pg.fromCurrency != libutils.NilAsset
		if pg.fromCurrency == ati.Type || isMatching {
			return
		}
		pg.updateWalletAndAccountSelector([]libutils.AssetType{ati.Type}, nil)
	})

	pg.toAmountEditor.AssetTypeSelector.AssetTypeSelected(func(ati *components.AssetTypeItem) {
		isMatching := pg.fromCurrency == pg.toCurrency && pg.toCurrency != libutils.NilAsset
		if pg.toCurrency == ati.Type || isMatching {
			return
		}
		pg.updateWalletAndAccountSelector(nil, []libutils.AssetType{ati.Type})
	})

	pg.createOrderBtn = pg.Theme.Button(values.String(values.StrCreateOrder))
	pg.createOrderBtn.SetEnabled(false)

	pg.navToSettingsBtn = pg.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrExchange)))

	pg.exchangeSelector.ExchangeSelected(func(es *Exchange) {
		pg.selectedExchange = es

		// Initialize a new exchange using the selected exchange server
		exchange, err := pg.WL.AssetsManager.InstantSwap.NewExchangeServer(pg.selectedExchange.Server)
		if err != nil {
			log.Error(err)
			return
		}
		pg.exchange = exchange

		go func() {
			err := pg.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	})

	return pg
}

func (pg *CreateOrderPage) updateWalletAndAccountSelector(selectedFromAsset []utils.AssetType, selectedToAsset []utils.AssetType) {
	pg.updateAssetSelection(selectedFromAsset, selectedToAsset)
	pg.updateExchangeRate()
}

func (pg *CreateOrderPage) ID() string {
	return CreateOrderPageID
}

func (pg *CreateOrderPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	if pg.isExchangeAPIAllowed() && pg.isMultipleAssetTypeWalletAvailable() {
		pg.initPage()
	}
}

// initPage initializes required data on this page and should be called only
// once after it has been displayed.
func (pg *CreateOrderPage) initPage() {
	pg.inited = true
	pg.scheduler.SetChecked(pg.WL.AssetsManager.IsOrderSchedulerRunning())
	pg.listenForNotifications()
	pg.loadOrderConfig()
	go pg.scroll.FetchScrollData(false, pg.ParentWindow())
}

func (pg *CreateOrderPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *CreateOrderPage) HandleUserInteractions() {
	pg.createOrderBtn.SetEnabled(pg.canCreateOrder())

	if pg.swapButton.Button.Clicked() {
		pg.swapCurrency()
		if pg.exchange != nil {
			go func() {
				err := pg.getExchangeRateInfo()
				if err != nil {
					log.Error(err)
				}
			}()
		}
	}

	if clicked, selectedItem := pg.ordersList.ItemClicked(); clicked {
		orderItems := pg.scroll.FetchedData().([]*instantswap.Order)
		pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, orderItems[selectedItem]))
	}

	if pg.refreshExchangeRateBtn.Button.Clicked() {
		go func() {
			err := pg.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	}

	if pg.createOrderBtn.Clicked() {
		pg.showConfirmOrderModal()
	}

	if pg.settingsButton.Button.Clicked() {
		orderSettingsModal := newOrderSettingsModalModal(pg.Load, pg.orderData).
			OnSettingsSaved(func(params *callbackParams) {
				pg.orderData.sourceAccountSelector = params.sourceAccountSelector
				pg.orderData.sourceWalletSelector = params.sourceWalletSelector
				pg.orderData.destinationAccountSelector = params.destinationAccountSelector
				pg.orderData.destinationWalletSelector = params.destinationWalletSelector
				infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrOrderSettingsSaved), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(infoModal)
			}).
			OnCancel(func() { // needed to satisfy the modal instance
			})
		pg.ParentWindow().ShowModal(orderSettingsModal)
	}

	if pg.viewAllButton.Clicked() {
		pg.ParentNavigator().Display(NewOrderHistoryPage(pg.Load))
	}

	if pg.infoButton.Button.Clicked() {
		info := modal.NewCustomModal(pg.Load).
			SetContentAlignment(layout.Center, layout.Center, layout.Center).
			Body(values.String(values.StrCreateOrderPageInfo)).
			PositiveButtonWidth(values.MarginPadding100)
		pg.ParentWindow().ShowModal(info)
	}

	for _, evt := range pg.fromAmountEditor.Edit.Editor.Events() {
		if pg.fromAmountEditor.Edit.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if pg.inputsNotEmpty(pg.fromAmountEditor.Edit.Editor) {
					fromAmt, err := strconv.ParseFloat(pg.fromAmountEditor.Edit.Editor.Text(), 32)
					if err != nil {
						// empty usd input
						pg.toAmountEditor.Edit.Editor.SetText("")
						pg.amountErrorText = values.String(values.StrInvalidAmount)
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						return
					}
					pg.amountErrorText = ""
					if pg.exchangeRate != -1 {
						value := fromAmt * pg.exchangeRate
						v := strconv.FormatFloat(value, 'f', 8, 64)
						pg.amountErrorText = ""
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Edit.Editor.SetText(v) // 2 decimal places
					}
				} else {
					pg.toAmountEditor.Edit.Editor.SetText("")
				}
			}
		}
	}

	for _, evt := range pg.toAmountEditor.Edit.Editor.Events() {
		if pg.toAmountEditor.Edit.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if pg.inputsNotEmpty(pg.toAmountEditor.Edit.Editor) {
					f, err := strconv.ParseFloat(pg.toAmountEditor.Edit.Editor.Text(), 32)
					if err != nil {
						// empty usd input
						pg.fromAmountEditor.Edit.Editor.SetText("")
						pg.amountErrorText = values.String(values.StrInvalidAmount)
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						return
					}
					pg.amountErrorText = ""
					if pg.exchangeRate != -1 {
						value := f * pg.exchangeRate
						v := strconv.FormatFloat(value, 'f', 8, 64)
						pg.amountErrorText = ""
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.fromAmountEditor.Edit.Editor.SetText(v)
					}
				} else {
					pg.fromAmountEditor.Edit.Editor.SetText("")
				}
			}
		}
	}

	if pg.refreshClickable.Clicked() {
		go pg.WL.AssetsManager.InstantSwap.Sync(context.Background())
	}

	if pg.scheduler.Changed() {
		if pg.scheduler.IsChecked() {

			orderSettingsModal := newOrderSettingsModalModal(pg.Load, pg.orderData).
				OnSettingsSaved(func(params *callbackParams) {
					refundAddress, _ := pg.sourceWalletSelector.SelectedWallet().CurrentAddress(pg.sourceAccountSelector.SelectedAccount().Number)
					destinationAddress, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
					pg.sourceWalletID = pg.sourceWalletSelector.SelectedWallet().GetWalletID()
					pg.sourceAccountNumber = pg.sourceAccountSelector.SelectedAccount().Number
					pg.destinationWalletID = pg.destinationWalletSelector.SelectedWallet().GetWalletID()
					pg.destinationAccountNumber = pg.destinationAccountSelector.SelectedAccount().Number

					pg.refundAddress = refundAddress
					pg.destinationAddress = destinationAddress

					orderSchedulerModal := newOrderSchedulerModalModal(pg.Load, pg.orderData).
						OnOrderSchedulerStarted(func() {
							infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrSchedulerRunning), modal.DefaultClickFunc())
							pg.ParentWindow().ShowModal(infoModal)
						}).
						OnCancel(func() { // needed to satisfy the modal instance
							pg.scheduler.SetChecked(false)
						})
					pg.ParentWindow().ShowModal(orderSchedulerModal)
				}).
				OnCancel(func() { // needed to satisfy the modal instance
					pg.scheduler.SetChecked(false)
				})
			pg.ParentWindow().ShowModal(orderSettingsModal)
		} else {
			pg.WL.AssetsManager.StopScheduler()
		}
	}

	if pg.navToSettingsBtn.Button.Clicked() {
		pg.ParentWindow().Display(settings.NewSettingsPage(pg.Load))
	}
}

func (pg *CreateOrderPage) updateAmount() {
	if pg.inputsNotEmpty(pg.fromAmountEditor.Edit.Editor) {
		fromAmt, err := strconv.ParseFloat(pg.fromAmountEditor.Edit.Editor.Text(), 32)
		if err != nil {
			pg.toAmountEditor.Edit.Editor.SetText("")
			pg.amountErrorText = values.String(values.StrInvalidAmount)
			pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
			pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
			return
		}
		pg.amountErrorText = ""
		if pg.exchangeRate != -1 {
			value := fromAmt * pg.exchangeRate
			v := strconv.FormatFloat(value, 'f', 8, 64)
			pg.amountErrorText = ""
			pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
			pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
			pg.toAmountEditor.Edit.Editor.SetText(v) // 2 decimal places
		}
	} else {
		pg.toAmountEditor.Edit.Editor.SetText("")
	}
}

func (pg *CreateOrderPage) canCreateOrder() bool {
	if pg.selectedExchange == nil {
		return false
	}

	if pg.exchangeRate == 0 {
		return false
	}

	if pg.fromAmountEditor.Edit.Editor.Text() == "" {
		return false
	}

	if pg.toAmountEditor.Edit.Editor.Text() == "" {
		return false
	}

	if pg.amountErrorText != "" {
		return false
	}

	if pg.fromCurrency == pg.toCurrency {
		return false
	}

	return true
}

func (pg *CreateOrderPage) inputsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if e.Text() == "" {
			pg.amountErrorText = ""
			return false
		}
	}
	return true
}

func (pg *CreateOrderPage) updateAssetSelection(selectedFromAsset []utils.AssetType, selectedToAsset []utils.AssetType) {
	if len(selectedFromAsset) > 0 {
		pg.fromCurrency = selectedFromAsset[0]
		pg.sourceWalletSelector.SetSelectedAsset(pg.fromCurrency)

		pg.fromAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.fromCurrency)

		// If the to and from asset are the same, select a new to asset.
		if selectedFromAsset[0] == pg.toCurrency {
			// Get all available assets.
			allAssets := pg.WL.AssetsManager.AllAssetTypes()
			for _, asset := range allAssets {
				if asset != selectedFromAsset[0] {

					// Select the first available asset as the new to asset.
					pg.toCurrency = asset
					pg.destinationWalletSelector.SetSelectedAsset(pg.toCurrency)
					pg.toAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.toCurrency)

					break
				}
			}
		}
	}

	if len(selectedToAsset) > 0 {
		pg.toCurrency = selectedToAsset[0]
		pg.destinationWalletSelector.SetSelectedAsset(pg.toCurrency)

		pg.toAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.toCurrency)

		// If the to and from asset are the same, select a new from asset.
		if selectedToAsset[0] == pg.fromCurrency {

			// Get all available assets.
			allAssets := pg.WL.AssetsManager.AllAssetTypes()
			for _, asset := range allAssets {
				if asset != selectedToAsset[0] {

					// Select the first available asset as the new from asset.
					pg.fromCurrency = asset
					pg.sourceWalletSelector.SetSelectedAsset(pg.fromCurrency)
					pg.fromAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.fromCurrency)
				}
			}
		}
	}

	// check the watch only wallet on destination
	if pg.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet() {
		pg.sourceWalletSelector.SetSelectedAsset(pg.fromCurrency)
	}

	// update title of wallet selector
	pg.sourceWalletSelector.Title(values.String(values.StrSource)).EnableWatchOnlyWallets(false)
	pg.destinationWalletSelector.Title(values.String(values.StrDestination)).EnableWatchOnlyWallets(true)

	// Save the exchange configuration changes.
	pg.updateExchangeConfig()
}

// swapCurrency swaps the values of the from and to currency fields.
func (pg *CreateOrderPage) swapCurrency() {
	// store the current value of the from currency in a temp variable
	tempSourceWalletSelector := pg.sourceWalletSelector
	tempSourceAccountSelector := pg.sourceAccountSelector
	tempFromCurrencyType := pg.fromCurrency
	tempFromCurrencyValue := pg.fromAmountEditor.Edit.Editor.Text()

	// Swap values
	pg.sourceWalletSelector = pg.destinationWalletSelector
	pg.sourceAccountSelector = pg.destinationAccountSelector
	pg.fromCurrency = pg.toCurrency
	pg.fromAmountEditor.Edit.Editor.SetText(pg.toAmountEditor.Edit.Editor.Text())
	pg.fromAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.fromCurrency)

	pg.destinationWalletSelector = tempSourceWalletSelector
	pg.destinationAccountSelector = tempSourceAccountSelector
	pg.toCurrency = tempFromCurrencyType
	pg.toAmountEditor.Edit.Editor.SetText(tempFromCurrencyValue)
	pg.toAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.toCurrency)

	// check the watch only wallet on destination
	if pg.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet() {
		pg.sourceWalletSelector.SetSelectedAsset(pg.fromCurrency)
	}

	// update title of wallet selector
	pg.sourceWalletSelector.Title(values.String(values.StrSource)).EnableWatchOnlyWallets(false)
	pg.destinationWalletSelector.Title(values.String(values.StrDestination)).EnableWatchOnlyWallets(true)

	// Save the exchange configuration changes.
	pg.updateExchangeConfig()
}

func (pg *CreateOrderPage) isExchangeAPIAllowed() bool {
	isAllowed := pg.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.ExchangeHTTPAPI)
	if !isAllowed {
		pg.errMsg = values.StringF(values.StrNotAllowed, values.String(values.StrExchange))
	}
	return isAllowed
}

// isMultipleAssetTypeWalletAvailable checks if multiple asset types are
// available for exchange functionality to run smoothly. Otherwise exchange
// functionality is disable till different asset type wallets are created.
func (pg *CreateOrderPage) isMultipleAssetTypeWalletAvailable() bool {
	pg.errMsg = values.String(values.StrMinimumAssetType)
	allWallets := len(pg.WL.AssetsManager.AllWallets())
	btcWallets := len(pg.WL.AssetsManager.AllBTCWallets())
	dcrWallets := len(pg.WL.AssetsManager.AllDCRWallets())
	ltcWallets := len(pg.WL.AssetsManager.AllLTCWallets())
	if allWallets == 0 {
		// no wallets exist
		return false
	}

	switch {
	case allWallets > btcWallets && btcWallets > 0:
		// BTC and some other wallets exists
	case allWallets > dcrWallets && dcrWallets > 0:
		// DCR and some other wallets exists
	case allWallets > ltcWallets && ltcWallets > 0:
		// LTC and some other wallets exists
	default:
		return false
	}
	pg.errMsg = ""
	return true
}

func (pg *CreateOrderPage) Layout(gtx C) D {
	var msg string
	var overlaySet bool
	var navBtn *cryptomaterial.Button

	isTestNet := pg.Load.WL.AssetsManager.NetType() != libutils.Mainnet
	switch {
	case isTestNet:
		msg = values.String(values.StrNoExchangeOnTestnet)
		overlaySet = true

	case !pg.isExchangeAPIAllowed():
		msg = pg.errMsg
		navBtn = &pg.navToSettingsBtn
		overlaySet = true

	case !pg.isMultipleAssetTypeWalletAvailable():
		msg = pg.errMsg
		overlaySet = true
	}

	if !overlaySet && !pg.inited {
		pg.initPage()
	}

	pg.scroll.OnScrollChangeListener(pg.ParentWindow())

	sp := components.SubPage{
		Load:       pg.Load,
		Title:      values.String(values.StrCreateOrder),
		BackButton: pg.backButton,
		Back: func() {
			pg.ParentNavigator().CloseCurrentPage()
		},
		Body: func(gtx C) D {
			overlay := layout.Stacked(func(gtx C) D { return D{} })
			if overlaySet {
				gtxCopy := gtx
				overlay = layout.Stacked(func(gtx C) D {
					return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, msg, navBtn)
				})
				// Disable main page from receiving events.
				gtx = gtx.Disabled()
			}
			return layout.Stack{}.Layout(gtx, layout.Expanded(pg.layout), overlay)
		},
	}

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:     gtx.Dp(values.MarginPadding600),
			Height:    cryptomaterial.MatchParent,
			Alignment: layout.Middle,
			Padding:   layout.Inset{Top: values.MarginPadding20},
		}.Layout2(gtx, func(gtx C) D {
			return sp.Layout(pg.ParentWindow(), gtx)
		})
	})
}

func (pg *CreateOrderPage) layout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Bottom: values.MarginPadding16,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(0.65, func(gtx C) D {
						return layout.E.Layout(gtx, func(gtx C) D {
							return layout.Flex{
								Axis:      layout.Horizontal,
								Alignment: layout.Middle,
							}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Right: values.MarginPadding10,
									}.Layout(gtx, func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												txt := pg.Theme.Label(values.TextSize16, values.String(values.StrSelectServerTitle))
												return txt.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												return pg.exchangeSelector.Layout(pg.ParentWindow(), gtx)
											}),
										)
									})
								}),
							)
						})
					}),
					layout.Flexed(0.35, func(gtx C) D {
						return layout.E.Layout(gtx, func(gtx C) D {
							return layout.Flex{
								Axis:      layout.Horizontal,
								Alignment: layout.Middle,
							}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Flex{
										Axis: layout.Vertical,
									}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											title := pg.Theme.Label(values.TextSize16, values.String(values.StrScheduler))
											title.Color = pg.Theme.Color.GrayText2
											return title.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											if pg.WL.AssetsManager.IsOrderSchedulerRunning() {
												return layout.Flex{
													Axis: layout.Horizontal,
												}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														return layout.Inset{
															Top:   values.MarginPadding5,
															Right: values.MarginPadding2,
														}.Layout(gtx, pg.Theme.Icons.TimerIcon.Layout12dp)
													}),
													layout.Rigid(func(gtx C) D {
														title := pg.Theme.Label(values.TextSize16, pg.WL.AssetsManager.GetShedulerRuntime())
														title.Color = pg.Theme.Color.GrayText2
														return title.Layout(gtx)
													}),
												)
											}

											return D{}
										}),
									)
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Left: values.MarginPadding4,
									}.Layout(gtx, pg.scheduler.Layout)
								}),
								layout.Rigid(func(gtx C) D {
									if pg.WL.AssetsManager.IsOrderSchedulerRunning() {
										return layout.Inset{Left: values.MarginPadding4, Top: unit.Dp(2)}.Layout(gtx, func(gtx C) D {
											gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding16)
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											loader := material.Loader(pg.Theme.Base)
											loader.Color = pg.Theme.Color.Gray1
											return loader.Layout(gtx)
										})
									}
									return D{}
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Right: values.MarginPadding10,
										Left:  values.MarginPadding10,
									}.Layout(gtx, pg.infoButton.Layout)
								}),
								layout.Rigid(pg.settingsButton.Layout),
							)
						})
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Flexed(0.45, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							walletName := "----"
							if pg.sourceWalletSelector != nil && pg.sourceWalletSelector.SelectedWallet() != nil {
								walletName = pg.sourceWalletSelector.SelectedWallet().GetWalletName()
							}
							accountName := "----"
							if pg.sourceAccountSelector != nil && pg.sourceAccountSelector.SelectedAccount() != nil {
								accountName = pg.sourceAccountSelector.SelectedAccount().Name
							}
							txt := fmt.Sprintf("%s: %s[%s]", values.String(values.StrSource), walletName, accountName)
							lb := pg.Theme.Label(values.TextSize16, txt)
							lb.Font.Weight = font.SemiBold
							return lb.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.fromAmountEditor.Layout(pg.ParentWindow(), gtx)
						}),
					)
				}),
				layout.Flexed(0.1, func(gtx C) D {
					return layout.Center.Layout(gtx, pg.swapButton.Layout)
				}),
				layout.Flexed(0.45, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							walletName := "----"
							if pg.destinationWalletSelector != nil && pg.destinationWalletSelector.SelectedWallet() != nil {
								walletName = pg.destinationWalletSelector.SelectedWallet().GetWalletName()
							}
							accountName := "----"
							if pg.destinationAccountSelector != nil && pg.destinationAccountSelector.SelectedAccount() != nil {
								accountName = pg.destinationAccountSelector.SelectedAccount().Name
							}
							txt := fmt.Sprintf("%s: %s[%s]", values.String(values.StrDestination), walletName, accountName)
							lb := pg.Theme.Label(values.TextSize16, txt)
							lb.Font.Weight = font.SemiBold
							return lb.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.toAmountEditor.Layout(pg.ParentWindow(), gtx)
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Bottom: values.MarginPadding16,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Flexed(0.55, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if pg.amountErrorText != "" {
									txt := pg.Theme.Label(values.TextSize14, pg.amountErrorText)
									txt.Font.Weight = font.SemiBold
									txt.Color = pg.Theme.Color.Danger
									return txt.Layout(gtx)
								}

								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										if pg.fetchingRate {
											gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding16)
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return pg.materialLoader.Layout(gtx)
										}
										txt := pg.Theme.Label(values.TextSize14, pg.exchangeRateInfo)
										txt.Color = pg.Theme.Color.Gray1
										txt.Font.Weight = font.SemiBold
										return txt.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										if !pg.fetchingRate && pg.rateError {
											return pg.refreshExchangeRateBtn.Layout(gtx)
										}
										return D{}
									}),
								)
							}),
						)
					}),
					layout.Flexed(0.45, func(gtx C) D {
						if pg.fetchingRate {
							gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding16)
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return pg.materialLoader.Layout(gtx)
						}

						fromCur := strings.ToUpper(pg.fromCurrency.String())
						toCur := strings.ToUpper(pg.toCurrency.String())
						missingAsset := fromCur == "" || toCur == ""
						if pg.exchangeRate > 0 && !missingAsset {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									serverName := pg.exchangeSelector.SelectedExchange().Name
									exchangeRate := values.StringF(values.StrServerRate, serverName, fromCur, pg.exchangeRate, toCur)
									txt := pg.Theme.Label(values.TextSize14, exchangeRate)
									txt.Font.Weight = font.SemiBold
									txt.Color = pg.Theme.Color.Gray1
									return txt.Layout(gtx)
								}),
								layout.Rigid(func(gtx C) D {
									ticker := pg.WL.AssetsManager.RateSource.GetTicker(fromCur + ext.MktSep + toCur)
									if ticker == nil || ticker.LastTradePrice <= 0 {
										return D{}
									}

									rate := ticker.LastTradePrice
									//  Binance and Bittrex always returns
									// ticker.LastTradePrice in's the quote
									// asset unit e.g DCR-BTC, LTC-BTC. We will
									// also do this when and if USDT is
									// supported.
									if pg.fromCurrency == libutils.BTCWalletAsset {
										rate = 1 / ticker.LastTradePrice
									}

									binanceRate := values.StringF(values.StrCurrencyConverterRate, pg.WL.AssetsManager.RateSource.Name(), fromCur, rate, toCur)
									txt := pg.Theme.Label(values.TextSize14, binanceRate)
									txt.Font.Weight = font.SemiBold
									txt.Color = pg.Theme.Color.Gray1
									return txt.Layout(gtx)
								}),
							)
						}
						return D{}
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Inset{
					Top: values.MarginPadding16,
				}.Layout(gtx, pg.createOrderBtn.Layout)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top: values.MarginPadding24,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								txt := pg.Theme.Label(values.TextSize18, values.StringF(values.StrRecentOrders, pg.scroll.ItemsCount()))
								txt.Font.Weight = font.SemiBold
								return txt.Layout(gtx)
							}),
							layout.Flexed(1, func(gtx C) D {
								body := func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,

										layout.Rigid(func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													var text string
													if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
														text = values.String(values.StrSyncingState)
													} else {
														text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp())

														if pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp() == 0 {
															text = values.String(values.StrNeverSynced)
														}
													}

													lastUpdatedInfo := pg.Theme.Label(values.TextSize12, text)
													lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
													return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
												}),
												layout.Rigid(func(gtx C) D {
													return cryptomaterial.LinearLayout{
														Width:     cryptomaterial.WrapContent,
														Height:    cryptomaterial.WrapContent,
														Clickable: pg.refreshClickable,
														Direction: layout.Center,
														Alignment: layout.Middle,
														Margin:    layout.Inset{Left: values.MarginPadding10},
													}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
																gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding8)
																gtx.Constraints.Min.X = gtx.Constraints.Max.X
																return layout.Inset{Bottom: values.MarginPadding1}.Layout(gtx, pg.materialLoader.Layout)
															}
															return layout.Inset{Right: values.MarginPadding16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
																return pg.refreshIcon.LayoutSize(gtx, values.MarginPadding18)
															})
														}),
													)
												}),
											)
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Inset{Right: values.MarginPadding16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return layout.E.Layout(gtx, pg.viewAllButton.Layout)
											})
										}),
									)
								}
								return layout.E.Layout(gtx, body)
							}),
						)
					}),
					layout.Flexed(1, func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							return layout.Stack{}.Layout(gtx,
								layout.Expanded(func(gtx C) D {
									return layout.Inset{}.Layout(gtx, pg.layoutHistory)
								}),
							)
						})
					}),
				)
			})
		}),
	)
}

func (pg *CreateOrderPage) fetchOrders(offset, pageSize int32) (interface{}, int, bool, error) {
	orders := components.LoadOrders(pg.Load, offset, pageSize, true)
	return orders, len(orders), false, nil
}

func (pg *CreateOrderPage) layoutHistory(gtx C) D {
	if pg.scroll.ItemsCount() <= 0 {
		return components.LayoutNoOrderHistory(gtx, pg.Load, false)
	}
	orderItems := pg.scroll.FetchedData().([]*instantswap.Order)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.ordersList.Layout(gtx, len(orderItems), func(gtx C, i int) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Surface,
							Direction:   layout.W,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.UniformInset(values.MarginPadding15),
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4},
						}.
							Layout2(gtx, func(gtx C) D {
								return components.OrderItemWidget(gtx, pg.Load, orderItems[i])
							})
					})
				})
			})
		}),
	)
}

func (pg *CreateOrderPage) showConfirmOrderModal() {
	invoicedAmount, _ := strconv.ParseFloat(pg.fromAmountEditor.Edit.Editor.Text(), 32)
	orderedAmount, _ := strconv.ParseFloat(pg.toAmountEditor.Edit.Editor.Text(), 32)
	refundAddress, _ := pg.sourceWalletSelector.SelectedWallet().CurrentAddress(pg.sourceAccountSelector.SelectedAccount().Number)
	destinationAddress, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
	pg.exchangeServer = pg.selectedExchange.Server
	pg.sourceWalletID = pg.sourceWalletSelector.SelectedWallet().GetWalletID()
	pg.sourceAccountNumber = pg.sourceAccountSelector.SelectedAccount().Number
	pg.destinationWalletID = pg.destinationWalletSelector.SelectedWallet().GetWalletID()
	pg.destinationAccountNumber = pg.destinationAccountSelector.SelectedAccount().Number

	pg.invoicedAmount = invoicedAmount
	pg.orderedAmount = orderedAmount

	pg.refundAddress = refundAddress
	pg.destinationAddress = destinationAddress

	confirmOrderModal := newConfirmOrderModal(pg.Load, pg.orderData).
		OnOrderCompleted(func(order *instantswap.Order) {
			pg.scroll.FetchScrollData(false, pg.ParentWindow())
			successModal := modal.NewCustomModal(pg.Load).
				Title(values.String(values.StrOrderSubmitted)).
				SetCancelable(true).
				SetContentAlignment(layout.Center, layout.Center, layout.Center).
				SetNegativeButtonText(values.String(values.StrOK)).
				SetNegativeButtonCallback(func() {
				}).
				PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
				SetPositiveButtonText(values.String(values.StrOrderDetails)).
				SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
					pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, order))
					return true
				})
			pg.ParentWindow().ShowModal(successModal)
		})

	pg.ParentWindow().ShowModal(confirmOrderModal)
}

func (pg *CreateOrderPage) updateExchangeRate() {
	if pg.fromCurrency == pg.toCurrency {
		return
	}
	if pg.exchange != nil {
		go func() {
			err := pg.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	}
}

func (pg *CreateOrderPage) getExchangeRateInfo() error {
	pg.exchangeRate = -1
	pg.fetchingRate = true
	fromCur := pg.fromCurrency.String()
	toCur := pg.toCurrency.String()
	params := api.ExchangeRateRequest{
		From:   fromCur,
		To:     toCur,
		Amount: libwallet.DefaultRateRequestAmount, // amount needs to be greater than 0 to get the exchange rate
	}
	res, err := pg.WL.AssetsManager.InstantSwap.GetExchangeRateInfo(pg.exchange, params)
	if err != nil {
		pg.exchangeRateInfo = values.String(values.StrFetchRateError)
		pg.rateError = true
		pg.fetchingRate = false
		return err
	}

	pg.exchangeRate = res.EstimatedAmount // estimated receivable value for libwallet.DefaultRateRequestAmount (1)
	pg.min = res.Min
	pg.max = res.Max

	pg.exchangeRateInfo = fmt.Sprintf(values.String(values.StrMinMax), pg.min, pg.max)
	pg.updateAmount()

	pg.fetchingRate = false
	pg.rateError = false
	return nil
}

// loadOrderConfig loads the existing exchange configuration or creates a new
// one if none existed before.
func (pg *CreateOrderPage) loadOrderConfig() {
	sourceAccount, destinationAccount := int32(-1), int32(-1)
	var sourceWallet, destinationWallet *load.WalletMapping

	// isConfigUpdateRequired is set to true when updating the configuration is
	// necessary.
	var isConfigUpdateRequired bool

	if pg.WL.AssetsManager.IsExchangeConfigSet() {
		// Use preset exchange configuration.
		exchangeConfig := pg.WL.AssetsManager.GetExchangeConfig()
		pg.fromCurrency = exchangeConfig.SourceAsset
		pg.toCurrency = exchangeConfig.DestinationAsset

		sourceWallet = &load.WalletMapping{
			Asset: pg.WL.AssetsManager.WalletWithID(int(exchangeConfig.SourceWalletID)),
		}
		destinationWallet = &load.WalletMapping{
			Asset: pg.WL.AssetsManager.WalletWithID(int(exchangeConfig.DestinationWalletID)),
		}

		sourceAccount = exchangeConfig.SourceAccountNumber
		destinationAccount = exchangeConfig.DestinationAccountNumber
	} else {
		// New exchange configuration will be generated using the set asset
		// types since none existed before. It two distinct asset type wallet
		// don't exist execution does get here.
		wallets := pg.WL.AssetsManager.AllWallets()
		pg.fromCurrency = wallets[0].GetAssetType()

		for _, w := range wallets {
			if w.GetAssetType() != pg.fromCurrency {
				pg.toCurrency = w.GetAssetType()
				break
			}
		}
	}

	// Source wallet picker
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, pg.fromCurrency).
		Title(values.String(values.StrSource))

	if sourceWallet == nil {
		isConfigUpdateRequired = true
		pg.sourceWalletSelector.SetSelectedAsset(pg.fromCurrency)
		sourceWallet = pg.sourceWalletSelector.SelectedWallet()
	} else {
		pg.sourceWalletSelector.SetSelectedWallet(sourceWallet)
	}

	// Source account picker
	pg.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrAccount)).
		AccountValidator(func(account *sharedW.Account) bool {
			return account.Number != load.MaxInt32 && !sourceWallet.IsWatchingOnlyWallet()
		})

	if sourceAccount != -1 {
		if _, err := sourceWallet.GetAccount(sourceAccount); err != nil {
			log.Error(err)
		} else {
			pg.sourceAccountSelector.SelectAccount(sourceWallet, sourceAccount)
		}
	}

	if pg.sourceAccountSelector.SelectedAccount() == nil {
		isConfigUpdateRequired = true
		pg.sourceAccountSelector.SelectFirstValidAccount(sourceWallet)
	}

	pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	// Destination wallet picker
	pg.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load, pg.toCurrency).
		Title(values.String(values.StrDestination))

	if destinationWallet == nil {
		isConfigUpdateRequired = true
		pg.destinationWalletSelector.SetSelectedAsset(pg.toCurrency)
		destinationWallet = pg.destinationWalletSelector.SelectedWallet()
	} else {
		pg.destinationWalletSelector.SetSelectedWallet(destinationWallet)
	}

	// Destination account picker
	pg.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrAccount)).
		AccountValidator(func(account *sharedW.Account) bool {
			return account.Number != load.MaxInt32
		})

	if destinationAccount != -1 {
		if _, err := destinationWallet.GetAccount(destinationAccount); err != nil {
			log.Error(err)
		} else {
			pg.destinationAccountSelector.SelectAccount(destinationWallet, destinationAccount)
		}
	}

	if pg.destinationAccountSelector.SelectedAccount() == nil {
		isConfigUpdateRequired = true
		pg.destinationAccountSelector.SelectFirstValidAccount(destinationWallet)
	}

	pg.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	if isConfigUpdateRequired {
		pg.updateExchangeConfig()
	}

	pg.fromAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.fromCurrency)
	pg.toAmountEditor.AssetTypeSelector.SetSelectedAssetType(pg.toCurrency)
}

// updateExchangeConfig Updates the newly created or modified exchange
// configuration.
func (pg *CreateOrderPage) updateExchangeConfig() {
	configInfo := sharedW.ExchangeConfig{
		SourceAsset:              pg.fromCurrency,
		DestinationAsset:         pg.toCurrency,
		SourceWalletID:           int32(pg.sourceWalletSelector.SelectedWallet().GetWalletID()),
		DestinationWalletID:      int32(pg.destinationWalletSelector.SelectedWallet().GetWalletID()),
		SourceAccountNumber:      pg.sourceAccountSelector.SelectedAccount().Number,
		DestinationAccountNumber: pg.destinationAccountSelector.SelectedAccount().Number,
	}

	pg.WL.AssetsManager.SetExchangeConfig(configInfo)
}

func (pg *CreateOrderPage) listenForNotifications() {
	if pg.OrderNotificationListener != nil && pg.RateListener != nil {
		return
	}

	pg.WL.AssetsManager.InstantSwap.RemoveNotificationListener(CreateOrderPageID) // clear if any
	pg.OrderNotificationListener = listeners.NewOrderNotificationListener()
	err := pg.WL.AssetsManager.InstantSwap.AddNotificationListener(pg.OrderNotificationListener, CreateOrderPageID)
	if err != nil {
		log.Errorf("Error adding instantswap notification listener: %v", err)
		return
	}

	pg.WL.AssetsManager.RateSource.RemoveRateListener(CreateOrderPageID) // clear if any
	pg.RateListener = ext.NewRateListener()
	err = pg.WL.AssetsManager.RateSource.AddRateListener(pg.RateListener, CreateOrderPageID)
	if err != nil {
		log.Errorf("Error adding rate notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.OrderNotifChan:
				switch n.OrderStatus {
				case wallet.OrderStatusSynced, wallet.OrderCreated:
					pg.scroll.FetchScrollData(false, pg.ParentWindow())
					pg.ParentWindow().Reload()
				case wallet.OrderSchedulerStarted:
					pg.scheduler.SetChecked(pg.WL.AssetsManager.IsOrderSchedulerRunning())
				case wallet.OrderSchedulerEnded:
					pg.scheduler.SetChecked(false)
				}
			case <-pg.RateUpdateChan:
				pg.ParentWindow().Reload()
			case <-pg.ctx.Done():
				pg.WL.AssetsManager.RateSource.RemoveRateListener(CreateOrderPageID)
				pg.WL.AssetsManager.InstantSwap.RemoveNotificationListener(CreateOrderPageID)
				close(pg.OrderNotifChan)
				close(pg.RateUpdateChan)
				pg.RateListener = nil
				pg.OrderNotificationListener = nil

				return
			}
		}
	}()
}
