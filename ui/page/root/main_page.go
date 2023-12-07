package root

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/accounts"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/info"
	"github.com/crypto-power/cryptopower/ui/page/privacy"
	"github.com/crypto-power/cryptopower/ui/page/seedbackup"
	"github.com/crypto-power/cryptopower/ui/page/send"
	"github.com/crypto-power/cryptopower/ui/page/staking"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/gen2brain/beeep"
)

const (
	MainPageID = "Main"
)

var PageNavigationMap = map[string]string{
	values.String(values.StrInfo):         info.InfoID,
	values.String(values.StrSend):         send.SendPageID,
	values.String(values.StrReceive):      ReceivePageID,
	values.String(values.StrTransactions): transaction.TransactionsPageID,
	values.String(values.StrSettings):     WalletSettingsPageID,
	values.String(values.StrStakeShuffle): privacy.AccountMixerPageID,
	values.String(values.StrStaking):      staking.OverviewPageID,
}

// SingleWalletMasterPage is a master page for interacting with a single wallet.
// It has sub pages for viewing a wallet's info, sending and receiving funds for
// a wallet, viewing a wallet's transactions, etc.
// TODO: Rename this file in a separate commit, for easier review.
type SingleWalletMasterPage struct {
	*app.MasterPage
	*load.Load

	selectedWallet sharedW.Asset
	// walletBalance is cached here to avoid repeatedly fetching the balance
	// from the wallet on each layout. TODO: Ensure this is updated on new
	// blocks and txs, or read realtime balance directly from wallet and don't
	// cache.
	walletBalance sharedW.AssetAmount

	pageNavigationTab      *cryptomaterial.SegmentedControl
	hideBalanceButton      *cryptomaterial.Clickable
	refreshExchangeRateBtn *cryptomaterial.Clickable
	openWalletSelector     *cryptomaterial.Clickable
	checkBox               cryptomaterial.CheckBoxStyle

	usdExchangeRate        float64
	usdExchangeSet         bool
	isFetchingExchangeRate bool
	isBalanceHidden        bool

	totalBalanceUSD string

	activeTab         map[string]string
	PageNavigationMap map[string]string

	showNavigationFunc func()
}

func NewSingleWalletMasterPage(l *load.Load, wallet sharedW.Asset, showNavigationFunc func()) *SingleWalletMasterPage {
	swmp := &SingleWalletMasterPage{
		Load:               l,
		MasterPage:         app.NewMasterPage(MainPageID),
		selectedWallet:     wallet,
		checkBox:           l.Theme.CheckBox(new(widget.Bool), values.String(values.StrAwareOfRisk)),
		showNavigationFunc: showNavigationFunc,
	}

	swmp.activeTab = make(map[string]string)
	swmp.hideBalanceButton = swmp.Theme.NewClickable(false)
	swmp.openWalletSelector = swmp.Theme.NewClickable(false)
	swmp.refreshExchangeRateBtn = swmp.Theme.NewClickable(true)

	swmp.initTabOptions()

	return swmp
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (swmp *SingleWalletMasterPage) ID() string {
	return MainPageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (swmp *SingleWalletMasterPage) OnNavigatedTo() {
	// load wallet account balance first before rendering page contents.
	// It loads balance for the current selected wallet.
	swmp.updateBalance()
	// updateExchangeSetting also calls updateBalance() but because of the API
	// call it may take a while before the balance and USD conversion is updated.
	// updateBalance() is called above first to prevent crash when balance value
	// is required before updateExchangeSetting() returns.
	swmp.updateExchangeSetting()

	backupLater := swmp.selectedWallet.ReadBoolConfigValueForKey(sharedW.SeedBackupNotificationConfigKey, false)
	// reset the checkbox
	swmp.checkBox.CheckBox.Value = false

	needBackup := swmp.selectedWallet.GetEncryptedSeed() != ""
	if needBackup && !backupLater {
		swmp.showBackupInfo()
	}

	if swmp.CurrentPage() == nil {
		swmp.Display(info.NewInfoPage(swmp.Load, swmp.selectedWallet)) // TODO: Should pagestack have a start page? YES!
	} else {
		swmp.CurrentPage().OnNavigatedTo()
	}
	// set active tab value
	swmp.activeTab[swmp.pageNavigationTab.SelectedSegment()] = swmp.CurrentPageID()

	swmp.listenForNotifications() // ntfn listeners are stopped in OnNavigatedFrom().

	if swmp.selectedWallet.GetAssetType() == libutils.DCRWalletAsset {
		if swmp.selectedWallet.ReadBoolConfigValueForKey(sharedW.FetchProposalConfigKey, false) && swmp.isGovernanceAPIAllowed() {
			if swmp.AssetsManager.Politeia.IsSyncing() {
				return
			}
			go swmp.AssetsManager.Politeia.Sync(context.TODO()) // TODO: Politeia should be given a ctx when initialized.
		}
	}
}

// initTabOptions initializes the page navigation tabs
func (swmp *SingleWalletMasterPage) initTabOptions() {
	commonTabs := []string{
		values.String(values.StrInfo),
		values.String(values.StrReceive),
		values.String(values.StrTransactions),
		values.String(values.StrAccounts),
		values.String(values.StrSettings),
	}

	if !swmp.selectedWallet.IsWatchingOnlyWallet() {
		restrictedAccessTabs := []string{
			values.String(values.StrSend),
		}

		// update the tab options with additional items at specific index
		commonTabs = append(commonTabs[:1], append(restrictedAccessTabs, commonTabs[1:]...)...)
	}

	if swmp.selectedWallet.GetAssetType() == libutils.DCRWalletAsset {
		dcrSpecificTabs := []string{
			values.String(values.StrStakeShuffle),
			values.String(values.StrStaking),
		}

		insertIndex := 4
		if len(commonTabs) == 4 {
			insertIndex = 3
		}

		// update the tab options with additional items at specific index
		commonTabs = append(commonTabs[:insertIndex], append(dcrSpecificTabs, commonTabs[insertIndex:]...)...)
	}

	swmp.pageNavigationTab = swmp.Theme.SegmentedControl(commonTabs, cryptomaterial.SegmentTypeSplit)
}

func (swmp *SingleWalletMasterPage) isGovernanceAPIAllowed() bool {
	return swmp.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (swmp *SingleWalletMasterPage) updateExchangeSetting() {
	swmp.usdExchangeSet = false
	if swmp.AssetsManager.ExchangeRateFetchingEnabled() {
		go swmp.fetchExchangeRate()
	}
}

func (swmp *SingleWalletMasterPage) fetchExchangeRate() {
	if swmp.isFetchingExchangeRate {
		return
	}

	swmp.isFetchingExchangeRate = true
	var market string
	switch swmp.selectedWallet.GetAssetType() {
	case libutils.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libutils.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libutils.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Asset type %q is not supported for exchange rate fetching", swmp.selectedWallet.GetAssetType())
		swmp.isFetchingExchangeRate = false
		return
	}

	rate := swmp.AssetsManager.RateSource.GetTicker(market)
	if rate == nil || rate.LastTradePrice <= 0 {
		swmp.isFetchingExchangeRate = false
		return
	}

	swmp.usdExchangeRate = rate.LastTradePrice
	swmp.updateBalance()
	swmp.usdExchangeSet = true
	swmp.ParentWindow().Reload()
	swmp.isFetchingExchangeRate = false
}

func (swmp *SingleWalletMasterPage) updateBalance() {
	totalBalance, err := components.CalculateTotalWalletsBalance(swmp.selectedWallet)
	if err != nil {
		log.Error(err)
		return
	}
	swmp.walletBalance = totalBalance.Total
	balanceInUSD := totalBalance.Total.MulF64(swmp.usdExchangeRate).ToCoin()
	swmp.totalBalanceUSD = utils.FormatAsUSDString(swmp.Printer, balanceInUSD)
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed
// to enable restyling UI elements where necessary.
// Satisfies the load.AppSettingsChangeHandler interface.
func (swmp *SingleWalletMasterPage) OnDarkModeChanged(isDarkModeOn bool) {
	// TODO: currentPage will likely be the Settings page when this method
	// is called. If that page implements the AppSettingsChangeHandler interface,
	// the following code will trigger the OnDarkModeChanged method of that
	// page.
	if currentPage, ok := swmp.CurrentPage().(load.AppSettingsChangeHandler); ok {
		currentPage.OnDarkModeChanged(isDarkModeOn)
	}
}

func (swmp *SingleWalletMasterPage) OnCurrencyChanged() {
	swmp.updateExchangeSetting()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (swmp *SingleWalletMasterPage) HandleUserInteractions() {
	if swmp.CurrentPage() != nil {
		swmp.CurrentPage().HandleUserInteractions()
	}

	if swmp.refreshExchangeRateBtn.Clicked() {
		go swmp.fetchExchangeRate()
	}

	for swmp.openWalletSelector.Clicked() {
		swmp.showNavigationFunc()
	}

	displayPage := func(pg app.Page) {
		// Load the current wallet balance on page reload.
		swmp.updateBalance()
		swmp.Display(pg)
	}

	if swmp.pageNavigationTab.Changed() {
		var pg app.Page
		switch swmp.pageNavigationTab.SelectedSegment() {
		case values.String(values.StrSend):
			pg = send.NewSendPage(swmp.Load, swmp.selectedWallet)
		case values.String(values.StrReceive):
			pg = NewReceivePage(swmp.Load, swmp.selectedWallet)
		case values.String(values.StrInfo):
			pg = info.NewInfoPage(swmp.Load, swmp.selectedWallet)
		case values.String(values.StrTransactions):
			pg = transaction.NewTransactionsPage(swmp.Load, swmp.selectedWallet)
		case values.String(values.StrStakeShuffle):
			dcrW := swmp.selectedWallet.(*dcr.Asset)
			if dcrW != nil {
				if !dcrW.AccountMixerConfigIsSet() {
					pg = privacy.NewSetupPrivacyPage(swmp.Load, dcrW)
				} else {
					pg = privacy.NewAccountMixerPage(swmp.Load, dcrW)
				}
			}
		case values.String(values.StrStaking):
			dcrW := swmp.selectedWallet.(*dcr.Asset)
			if dcrW == nil {
				log.Error(values.ErrDCRSupportedOnly)
			} else {
				pg = staking.NewStakingPage(swmp.Load, dcrW)
			}
		case values.String(values.StrAccounts):
			pg = accounts.NewAccountPage(swmp.Load, swmp.selectedWallet)
		case values.String(values.StrSettings):
			pg = NewWalletSettingsPage(swmp.Load, swmp.selectedWallet, swmp.showNavigationFunc)
		}

		swmp.activeTab[swmp.pageNavigationTab.SelectedSegment()] = pg.ID()

		displayPage(pg)
	}

	// update active page tab. This is needed for scenarios where a page is
	// navigated to without using the page navigation tab. An example is
	// the redirection action from the info page to the mixer page.
	if swmp.CurrentPageID() != swmp.activeTab[swmp.pageNavigationTab.SelectedSegment()] {
		for tabTitle, pageID := range PageNavigationMap {
			if swmp.CurrentPageID() == pageID {
				swmp.activeTab[tabTitle] = swmp.CurrentPageID()
				swmp.pageNavigationTab.SetSelectedSegment(tabTitle)
			}
		}
	}

	for swmp.hideBalanceButton.Clicked() {
		swmp.isBalanceHidden = !swmp.isBalanceHidden
		swmp.selectedWallet.SetBoolConfigValueForKey(sharedW.HideBalanceConfigKey, swmp.isBalanceHidden)
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (swmp *SingleWalletMasterPage) KeysToHandle() key.Set {
	if currentPage := swmp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			return keyEvtHandler.KeysToHandle()
		}
	}
	return ""
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (swmp *SingleWalletMasterPage) HandleKeyPress(evt *key.Event) {
	if currentPage := swmp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			keyEvtHandler.HandleKeyPress(evt)
		}
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (swmp *SingleWalletMasterPage) OnNavigatedFrom() {
	// Also disappear all child pages.
	if swmp.CurrentPage() != nil {
		swmp.CurrentPage().OnNavigatedFrom()
	}

	// The encrypted seed exists by default and is cleared after wallet is backed up.
	// Activate the modal requesting the user to backup their current wallet on
	// every wallet open request until the encrypted seed is cleared (backup happens).
	if swmp.selectedWallet.GetEncryptedSeed() != "" {
		swmp.selectedWallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, false)
	}

	swmp.stopNtfnListeners()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (swmp *SingleWalletMasterPage) Layout(gtx C) D {
	// TODO: mobile layout
	// if swmp.Load.IsMobileView() {
	// 	return swmp.layoutMobile(gtx)
	// }
	return swmp.layoutDesktop(gtx)
}

func (swmp *SingleWalletMasterPage) layoutDesktop(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			alignment := layout.Middle
			if swmp.Load.IsMobileView() {
				alignment = layout.Start
			}
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
				Alignment:   alignment,
			}.Layout(gtx,
				layout.Rigid(swmp.LayoutTopBar),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return swmp.pageNavigationTab.Layout(gtx, func(gtx C) D {
							if swmp.CurrentPage() == nil {
								return D{}
							}
							switch swmp.CurrentPage().ID() {
							case ReceivePageID, send.SendPageID, staking.OverviewPageID,
								transaction.TransactionsPageID, privacy.AccountMixerPageID:
								// Disable page functionality if a page is not synced or rescanning is in progress.
								if !swmp.selectedWallet.IsSynced() || swmp.selectedWallet.IsRescanning() {
									return components.DisablePageWithOverlay(swmp.Load, swmp.CurrentPage(), gtx,
										values.String(values.StrFunctionUnavailable), nil)
								}
								fallthrough
							default:
								return swmp.CurrentPage().Layout(gtx)
							}
						})
					})
				}),
			)
		}),
	)
}

func (swmp *SingleWalletMasterPage) LayoutTopBar(gtx C) D {
	assetType := swmp.selectedWallet.GetAssetType()
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			h := values.MarginPadding24
			v := values.MarginPadding8
			orientation := layout.Horizontal
			if swmp.Load.IsMobileView() {
				orientation = layout.Vertical
			}
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.WrapContent,
				Orientation: orientation,
				Alignment:   layout.Start,
				Padding: layout.Inset{
					Right:  h,
					Left:   values.MarginPadding10,
					Top:    v,
					Bottom: v,
				},
			}.GradientLayout(gtx, assetType,
				layout.Rigid(func(gtx C) D {
					clickable := swmp.openWalletSelector
					return layout.Flex{
						Axis: layout.Horizontal,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return cryptomaterial.LinearLayout{
								Width:       cryptomaterial.WrapContent,
								Height:      cryptomaterial.WrapContent,
								Orientation: orientation,
								Clickable:   clickable,
							}.Layout2(gtx, swmp.Theme.Icons.ChevronLeft.Layout24dp)
						}),
						layout.Flexed(1, func(gtx C) D {
							isWatchOnlyWallet := swmp.selectedWallet.IsWatchingOnlyWallet()
							return layout.Center.Layout(gtx, func(gtx C) D {
								alignment := layout.Start
								orientation := layout.Horizontal
								if swmp.Load.IsMobileView() {
									alignment = layout.Middle
									orientation = layout.Vertical
									clickable = nil
								}
								return cryptomaterial.LinearLayout{
									Width:       cryptomaterial.WrapContent,
									Height:      cryptomaterial.WrapContent,
									Orientation: orientation,
									Alignment:   alignment,
									Clickable:   clickable,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Flex{
											Axis:      layout.Horizontal,
											Alignment: layout.Middle,
										}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												image := components.CoinImageBySymbol(swmp.Load, assetType, isWatchOnlyWallet)
												if image != nil {
													if swmp.Load.IsMobileView() {
														return image.Layout16dp(gtx)
													}
													return image.Layout24dp(gtx)
												}
												return D{}
											}),
											layout.Rigid(func(gtx C) D {
												lbl := swmp.Theme.H6(swmp.selectedWallet.GetWalletName())
												lbl.Color = swmp.Theme.Color.PageNavText
												if swmp.Load.IsMobileView() {
													lbl.TextSize = values.TextSize16
												}
												return layout.Inset{
													Left: values.MarginPadding10,
												}.Layout(gtx, lbl.Layout)
											}),
											layout.Rigid(func(gtx C) D {
												if isWatchOnlyWallet {
													return layout.Inset{
														Left: values.MarginPadding10,
													}.Layout(gtx, func(gtx C) D {
														textSize := values.TextSize16
														if swmp.Load.IsMobileView() {
															textSize = values.TextSize12
														}
														return components.WalletHightlighLabel(swmp.Theme, gtx, textSize, values.String(values.StrWatchOnly))
													})
												}
												return D{}
											}),
										)
									}),
									layout.Rigid(func(gtx C) D {
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										layoutPosition := layout.E
										if swmp.Load.IsMobileView() {
											layoutPosition = layout.Center
										}
										return layoutPosition.Layout(gtx, func(gtx C) D {
											return layout.Flex{}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													icon := swmp.Theme.Icons.RevealIcon
													if swmp.isBalanceHidden {
														icon = swmp.Theme.Icons.ConcealIcon
													}
													return layout.Inset{
														Top:   values.MarginPadding5,
														Right: values.MarginPadding9,
													}.Layout(gtx, func(gtx C) D {
														return swmp.hideBalanceButton.Layout(gtx, icon.Layout16dp)
													})
												}),
												layout.Rigid(swmp.totalAssetBalance),
												layout.Rigid(func(gtx C) D {
													if !swmp.isBalanceHidden {
														return swmp.LayoutUSDBalance(gtx)
													}
													return D{}
												}),
											)
										})
									}),
								)
							})
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return swmp.Theme.Separator().Layout(gtx)
		}),
	)
}

func (swmp *SingleWalletMasterPage) LayoutUSDBalance(gtx C) D {
	if !swmp.usdExchangeSet {
		return D{}
	}
	switch {
	case swmp.isFetchingExchangeRate && swmp.usdExchangeRate == 0:
		gtx.Constraints.Max.Y = gtx.Dp(values.MarginPadding18)
		gtx.Constraints.Max.X = gtx.Constraints.Max.Y
		return layout.Inset{
			Top:  values.MarginPadding8,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			loader := material.Loader(swmp.Theme.Base)
			return loader.Layout(gtx)
		})
	case !swmp.isFetchingExchangeRate && swmp.usdExchangeRate == 0:
		return layout.Inset{
			Top:  values.MarginPadding7,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			return swmp.refreshExchangeRateBtn.Layout(gtx, func(gtx C) D {
				return swmp.Theme.Icons.Restore.Layout16dp(gtx)
			})
		})
	case len(swmp.totalBalanceUSD) > 0:
		textSize := values.TextSize20
		if swmp.Load.IsMobileView() {
			textSize = values.TextSize16
		}
		lbl := swmp.Theme.Label(textSize, fmt.Sprintf("/ %s", swmp.totalBalanceUSD))
		lbl.Color = swmp.Theme.Color.PageNavText
		inset := layout.Inset{Left: values.MarginPadding8}
		return inset.Layout(gtx, lbl.Layout)
	default:
		return D{}
	}
}

func (swmp *SingleWalletMasterPage) totalAssetBalance(gtx C) D {
	textSize := values.TextSize20
	if swmp.Load.IsMobileView() {
		textSize = values.TextSize16
	}
	if swmp.isBalanceHidden || swmp.walletBalance == nil {
		hiddenBalanceText := swmp.Theme.Label(textSize*0.8, "*******************")
		return layout.Inset{Bottom: values.MarginPadding0, Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
			hiddenBalanceText.Color = swmp.Theme.Color.PageNavText
			return hiddenBalanceText.Layout(gtx)
		})
	}
	return components.LayoutBalanceWithUnitSize(gtx, swmp.Load, swmp.walletBalance.String(), textSize)
}

func (swmp *SingleWalletMasterPage) postTransactionNotification(t *sharedW.Transaction) {
	var notification string
	wal := swmp.selectedWallet
	switch t.Type {
	case dcr.TxTypeRegular:
		if t.Direction != dcr.TxDirectionReceived {
			return
		}
		// remove trailing zeros from amount and convert to string
		amount := strconv.FormatFloat(wal.ToAmount(t.Amount).ToCoin(), 'f', -1, 64)
		notification = values.StringF(values.StrDcrReceived, amount)
	case dcr.TxTypeVote:
		reward := strconv.FormatFloat(wal.ToAmount(t.VoteReward).ToCoin(), 'f', -1, 64)
		notification = values.StringF(values.StrTicektVoted, reward)
	case dcr.TxTypeRevocation:
		notification = values.String(values.StrTicketRevoked)
	default:
		return
	}

	if swmp.AssetsManager.OpenedWalletsCount() > 1 {
		notification = fmt.Sprintf("[%s] %s", wal.GetWalletName(), notification)
	}

	initializeBeepNotification(notification)
}

func (swmp *SingleWalletMasterPage) postProposalNotification(propName string, status libutils.ProposalStatus) {
	proposalNotification := swmp.selectedWallet.ReadBoolConfigValueForKey(sharedW.ProposalNotificationConfigKey, false) ||
		!swmp.AssetsManager.IsPrivacyModeOn()
	if !proposalNotification {
		return
	}

	var notification string
	switch status {
	case libutils.ProposalStatusNewProposal:
		notification = values.StringF(values.StrProposalAddedNotif, propName)
	case libutils.ProposalStatusVoteStarted:
		notification = values.StringF(values.StrVoteStartedNotif, propName)
	case libutils.ProposalStatusVoteFinished:
		notification = values.StringF(values.StrVoteEndedNotif, propName)
	default:
		notification = values.StringF(values.StrNewProposalUpdate, propName)
	}
	initializeBeepNotification(notification)
}

func initializeBeepNotification(n string) {
	absoluteWdPath, err := utils.GetAbsolutePath()
	if err != nil {
		log.Error(err.Error())
	}

	err = beeep.Notify(values.String(values.StrAppWallet), n,
		filepath.Join(absoluteWdPath, "ui/assets/decredicons/qrcodeSymbol.png"))
	if err != nil {
		log.Info("could not initiate desktop notification, reason:", err.Error())
	}
}

// listenForNotifications starts a goroutine to watch for notifications
// and update the UI accordingly.
func (swmp *SingleWalletMasterPage) listenForNotifications() {
	syncProgressListener := &sharedW.SyncProgressListener{
		OnSyncCompleted: func() {
			swmp.updateBalance()
			swmp.ParentWindow().Reload()
		},
	}
	err := swmp.selectedWallet.AddSyncProgressListener(syncProgressListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(transaction *sharedW.Transaction) {
			swmp.updateBalance()
			if swmp.AssetsManager.IsTransactionNotificationsOn() {
				// TODO: SPV wallets only receive mempool tx ntfn for txs that
				// were broadcast by the wallet. We should probably be posting
				// desktop ntfns for txs received from external parties, which
				// will can be gotten from the OnTransactionConfirmed callback.
				swmp.postTransactionNotification(transaction)
			}
			swmp.ParentWindow().Reload()
		},
		// OnBlockAttached is also called whenever OnTransactionConfirmed is
		// called, so use OnBlockAttached. Also, OnTransactionConfirmed may be
		// called multiple times whereas OnBlockAttached is only called once.
		OnBlockAttached: func(walletID int, blockHeight int32) {
			beep := swmp.selectedWallet.ReadBoolConfigValueForKey(sharedW.BeepNewBlocksConfigKey, false)
			if beep {
				err := beeep.Beep(5, 1)
				if err != nil {
					log.Error(err.Error)
				}
			}

			swmp.updateBalance()
			swmp.ParentWindow().Reload()
		},
	}
	err = swmp.selectedWallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	if swmp.isGovernanceAPIAllowed() {
		proposalSyncCallback := func(propName string, status libutils.ProposalStatus) {
			// Post desktop notification for all events except the synced event.
			if status != libutils.ProposalStatusSynced {
				swmp.postProposalNotification(propName, status)
			}
		}
		err = swmp.AssetsManager.Politeia.AddSyncCallback(proposalSyncCallback, MainPageID)
		if err != nil {
			log.Errorf("Error adding politeia notification listener: %v", err)
			return
		}
	}

	// TODO: Register trade order ntfn listener and post desktop ntfns for all
	// events except the synced event.
}

func (swmp *SingleWalletMasterPage) stopNtfnListeners() {
	swmp.selectedWallet.RemoveSyncProgressListener(MainPageID)
	swmp.selectedWallet.RemoveTxAndBlockNotificationListener(MainPageID)
	swmp.AssetsManager.Politeia.RemoveSyncCallback(MainPageID)
}

func (swmp *SingleWalletMasterPage) showBackupInfo() {
	backupNowOrLaterModal := modal.NewCustomModal(swmp.Load).
		SetupWithTemplate(modal.WalletBackupInfoTemplate).
		SetCancelable(false).
		SetContentAlignment(layout.W, layout.W, layout.Center).
		CheckBox(swmp.checkBox, true).
		SetNegativeButtonText(values.String(values.StrBackupLater)).
		SetNegativeButtonCallback(func() {
			swmp.selectedWallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, true)
		}).
		PositiveButtonStyle(swmp.Load.Theme.Color.Primary, swmp.Load.Theme.Color.InvText).
		SetPositiveButtonText(values.String(values.StrBackupNow)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			swmp.selectedWallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, true)
			swmp.ParentNavigator().Display(seedbackup.NewBackupInstructionsPage(swmp.Load, swmp.selectedWallet, func(load *load.Load, navigator app.WindowNavigator) {
				navigator.ClosePagesAfter(swmp.ParentWindow().CurrentPageID())
			}))
			return true
		})
	swmp.ParentWindow().ShowModal(backupNowOrLaterModal)
}
