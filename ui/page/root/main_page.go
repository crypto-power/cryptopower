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

type MainPage struct {
	*app.MasterPage
	*load.Load

	pageNavigationTab      *cryptomaterial.SegmentedControl
	hideBalanceButton      *cryptomaterial.Clickable
	refreshExchangeRateBtn *cryptomaterial.Clickable
	openWalletSelector     *cryptomaterial.Clickable
	checkBox               cryptomaterial.CheckBoxStyle

	// page state variables
	totalBalance   sharedW.AssetAmount
	selectedWallet sharedW.Asset
	assetType      libutils.AssetType

	usdExchangeRate        float64
	usdExchangeSet         bool
	isFetchingExchangeRate bool
	isBalanceHidden        bool

	currencyExchangeValue string
	totalBalanceUSD       string

	activeTab         map[string]string
	PageNavigationMap map[string]string

	showNavigationFunc func()
}

func NewMainPage(l *load.Load, showNavigationFunc func()) *MainPage {
	mp := &MainPage{
		Load:               l,
		MasterPage:         app.NewMasterPage(MainPageID),
		checkBox:           l.Theme.CheckBox(new(widget.Bool), values.String(values.StrAwareOfRisk)),
		showNavigationFunc: showNavigationFunc,
	}

	mp.activeTab = make(map[string]string)

	mp.selectedWallet = mp.WL.SelectedWallet.Wallet

	mp.initTabOptions()

	mp.hideBalanceButton = mp.Theme.NewClickable(false)
	mp.openWalletSelector = mp.Theme.NewClickable(false)
	mp.refreshExchangeRateBtn = mp.Theme.NewClickable(true)

	return mp
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (mp *MainPage) ID() string {
	return MainPageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (mp *MainPage) OnNavigatedTo() {
	mp.assetType = mp.selectedWallet.GetAssetType()
	// load wallet account balance first before rendering page contents.
	// It loads balance for the current selected wallet.
	mp.updateBalance()
	// updateExchangeSetting also calls updateBalance() but because of the API
	// call it may take a while before the balance and USD conversion is updated.
	// updateBalance() is called above first to prevent crash when balance value
	// is required before updateExchangeSetting() returns.
	mp.updateExchangeSetting()

	backupLater := mp.selectedWallet.ReadBoolConfigValueForKey(sharedW.SeedBackupNotificationConfigKey, false)
	// reset the checkbox
	mp.checkBox.CheckBox.Value = false

	needBackup := mp.selectedWallet.GetEncryptedSeed() != ""
	if needBackup && !backupLater {
		mp.showBackupInfo()
	}

	if mp.CurrentPage() == nil {
		mp.Display(info.NewInfoPage(mp.Load)) // TODO: Should pagestack have a start page? YES!
	} else {
		mp.CurrentPage().OnNavigatedTo()
	}
	// set active tab value
	mp.activeTab[mp.pageNavigationTab.SelectedSegment()] = mp.CurrentPageID()

	mp.listenForNotifications() // ntfn listeners are stopped in OnNavigatedFrom().

	if mp.assetType == libutils.DCRWalletAsset {
		if mp.selectedWallet.ReadBoolConfigValueForKey(sharedW.FetchProposalConfigKey, false) && mp.isGovernanceAPIAllowed() {
			if mp.WL.AssetsManager.Politeia.IsSyncing() {
				return
			}
			go mp.WL.AssetsManager.Politeia.Sync(context.TODO()) // TODO: Politeia should be given a ctx when initialized.
		}
	}
}

// initTabOptions initializes the page navigation tabs
func (mp *MainPage) initTabOptions() {
	commonTabs := []string{
		values.String(values.StrInfo),
		values.String(values.StrSend),
		values.String(values.StrReceive),
		values.String(values.StrTransactions),
		values.String(values.StrSettings),
	}

	if mp.selectedWallet.GetAssetType() == libutils.DCRWalletAsset {
		dcrSpecificTabs := []string{
			values.String(values.StrStakeShuffle),
			values.String(values.StrStaking),
		}

		// update the tab options with additional items at specific index
		commonTabs = append(commonTabs[:4], append(dcrSpecificTabs, commonTabs[4:]...)...)
	}

	mp.pageNavigationTab = mp.Theme.SegmentedControl(commonTabs, cryptomaterial.SegmentTypeSplit)
}

func (mp *MainPage) isGovernanceAPIAllowed() bool {
	return mp.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (mp *MainPage) updateExchangeSetting() {
	mp.usdExchangeSet = false
	if components.IsFetchExchangeRateAPIAllowed(mp.WL) {
		go mp.fetchExchangeRate()
	}
}

func (mp *MainPage) fetchExchangeRate() {
	if mp.isFetchingExchangeRate {
		return
	}

	mp.isFetchingExchangeRate = true
	var market string
	switch mp.assetType {
	case libutils.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libutils.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libutils.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", mp.assetType)
		mp.isFetchingExchangeRate = false
		return
	}

	rate := mp.WL.AssetsManager.RateSource.GetTicker(market)
	if rate == nil || rate.LastTradePrice <= 0 {
		mp.isFetchingExchangeRate = false
		return
	}

	mp.usdExchangeRate = rate.LastTradePrice
	mp.updateBalance()
	mp.usdExchangeSet = true
	mp.ParentWindow().Reload()
	mp.isFetchingExchangeRate = false
}

func (mp *MainPage) updateBalance() {
	totalBalance, err := components.CalculateTotalWalletsBalance(mp.Load)
	if err != nil {
		log.Error(err)
		return
	}
	mp.totalBalance = totalBalance.Total
	balanceInUSD := totalBalance.Total.MulF64(mp.usdExchangeRate).ToCoin()
	mp.totalBalanceUSD = utils.FormatAsUSDString(mp.Printer, balanceInUSD)
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed
// to enable restyling UI elements where necessary.
// Satisfies the load.AppSettingsChangeHandler interface.
func (mp *MainPage) OnDarkModeChanged(isDarkModeOn bool) {
	// TODO: currentPage will likely be the Settings page when this method
	// is called. If that page implements the AppSettingsChangeHandler interface,
	// the following code will trigger the OnDarkModeChanged method of that
	// page.
	if currentPage, ok := mp.CurrentPage().(load.AppSettingsChangeHandler); ok {
		currentPage.OnDarkModeChanged(isDarkModeOn)
	}
}

func (mp *MainPage) OnCurrencyChanged() {
	mp.updateExchangeSetting()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (mp *MainPage) HandleUserInteractions() {
	if mp.CurrentPage() != nil {
		mp.CurrentPage().HandleUserInteractions()
	}

	if mp.refreshExchangeRateBtn.Clicked() {
		go mp.fetchExchangeRate()
	}

	for mp.openWalletSelector.Clicked() {
		mp.showNavigationFunc()
	}

	displayPage := func(pg app.Page) {
		// Load the current wallet balance on page reload.
		mp.updateBalance()
		mp.Display(pg)
	}

	if mp.pageNavigationTab.Changed() {
		var pg app.Page
		switch mp.pageNavigationTab.SelectedSegment() {
		case values.String(values.StrSend):
			pg = send.NewSendPage(mp.Load, false)
		case values.String(values.StrReceive):
			pg = NewReceivePage(mp.Load)
		case values.String(values.StrInfo):
			pg = info.NewInfoPage(mp.Load)
		case values.String(values.StrTransactions):
			pg = transaction.NewTransactionsPage(mp.Load, false)
		case values.String(values.StrStakeShuffle):
			dcrUniqueImpl := mp.selectedWallet.(*dcr.Asset)
			if dcrUniqueImpl != nil {
				if !dcrUniqueImpl.AccountMixerConfigIsSet() {
					pg = privacy.NewSetupPrivacyPage(mp.Load)
				} else {
					pg = privacy.NewAccountMixerPage(mp.Load)
				}
			}
		case values.String(values.StrStaking):
			pg = staking.NewStakingPage(mp.Load)
		case values.String(values.StrSettings):
			pg = NewWalletSettingsPage(mp.Load, mp.showNavigationFunc)
		}

		mp.activeTab[mp.pageNavigationTab.SelectedSegment()] = pg.ID()

		displayPage(pg)
	}

	// update active page tab. This is needed for scenarios where a page is
	// navigated to without using the page navigation tab. An example is
	// the redirection action from the info page to the mixer page.
	if mp.CurrentPageID() != mp.activeTab[mp.pageNavigationTab.SelectedSegment()] {
		for tabTitle, pageID := range PageNavigationMap {
			if mp.CurrentPageID() == pageID {
				mp.activeTab[tabTitle] = mp.CurrentPageID()
				mp.pageNavigationTab.SetSelectedSegment(tabTitle)
			}
		}
	}

	for mp.hideBalanceButton.Clicked() {
		mp.isBalanceHidden = !mp.isBalanceHidden
		mp.selectedWallet.SetBoolConfigValueForKey(sharedW.HideBalanceConfigKey, mp.isBalanceHidden)
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (mp *MainPage) KeysToHandle() key.Set {
	if currentPage := mp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			return keyEvtHandler.KeysToHandle()
		}
	}
	return ""
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (mp *MainPage) HandleKeyPress(evt *key.Event) {
	if currentPage := mp.CurrentPage(); currentPage != nil {
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
func (mp *MainPage) OnNavigatedFrom() {
	// Also disappear all child pages.
	if mp.CurrentPage() != nil {
		mp.CurrentPage().OnNavigatedFrom()
	}

	// The encrypted seed exists by default and is cleared after wallet is backed up.
	// Activate the modal requesting the user to backup their current wallet on
	// every wallet open request until the encrypted seed is cleared (backup happens).
	if mp.selectedWallet.GetEncryptedSeed() != "" {
		mp.selectedWallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, false)
	}

	mp.stopNtfnListeners()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (mp *MainPage) Layout(gtx C) D {
	// TODO: mobile layout
	// if mp.Load.IsMobileView() {
	// 	return mp.layoutMobile(gtx)
	// }
	return mp.layoutDesktop(gtx)
}

func (mp *MainPage) layoutDesktop(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
				Alignment:   layout.Middle,
			}.Layout(gtx,
				layout.Rigid(mp.LayoutTopBar),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return mp.pageNavigationTab.Layout(gtx, func(gtx C) D {
							if mp.CurrentPage() == nil {
								return D{}
							}
							switch mp.CurrentPage().ID() {
							case ReceivePageID, send.SendPageID, staking.OverviewPageID,
								transaction.TransactionsPageID, privacy.AccountMixerPageID:
								// Disable page functionality if a page is not synced or rescanning is in progress.
								if !mp.selectedWallet.IsSynced() || mp.selectedWallet.IsRescanning() {
									return components.DisablePageWithOverlay(mp.Load, mp.CurrentPage(), gtx,
										values.String(values.StrFunctionUnavailable), nil)
								}
								fallthrough
							default:
								return mp.CurrentPage().Layout(gtx)
							}
						})
					})
				}),
			)
		}),
	)
}

func (mp *MainPage) LayoutTopBar(gtx C) D {
	assetType := mp.assetType
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			h := values.MarginPadding24
			v := values.MarginPadding8
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Alignment:   layout.Middle,
				Padding: layout.Inset{
					Right:  h,
					Left:   values.MarginPadding10,
					Top:    v,
					Bottom: v,
				},
			}.GradientLayout(gtx, assetType,
				layout.Rigid(func(gtx C) D {
					isWatchOnlyWallet := mp.selectedWallet.IsWatchingOnlyWallet()
					return layout.W.Layout(gtx, func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Alignment:   layout.Middle,
							Clickable:   mp.openWalletSelector,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Left:  values.MarginPadding12,
									Right: values.MarginPadding12,
								}.Layout(gtx, mp.Theme.Icons.ChevronLeft.Layout24dp)
							}),
							layout.Rigid(func(gtx C) D {
								image := components.CoinImageBySymbol(mp.Load, assetType, isWatchOnlyWallet)
								if image != nil {
									return image.Layout24dp(gtx)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								lbl := mp.Theme.H6(mp.selectedWallet.GetWalletName())
								lbl.Color = mp.Theme.Color.PageNavText
								return layout.Inset{
									Left: values.MarginPadding10,
								}.Layout(gtx, lbl.Layout)
							}),
							layout.Rigid(func(gtx C) D {
								if isWatchOnlyWallet {
									return layout.Inset{
										Left: values.MarginPadding10,
									}.Layout(gtx, func(gtx C) D {
										return components.WalletHightlighLabel(mp.Theme, gtx, values.TextSize16, values.String(values.StrWatchOnly))
									})
								}
								return D{}
							}),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								icon := mp.Theme.Icons.RevealIcon
								if mp.isBalanceHidden {
									icon = mp.Theme.Icons.ConcealIcon
								}
								return layout.Inset{
									Top:   values.MarginPadding5,
									Right: values.MarginPadding9,
								}.Layout(gtx, func(gtx C) D {
									return mp.hideBalanceButton.Layout(gtx, icon.Layout16dp)
								})
							}),
							layout.Rigid(mp.totalAssetBalance),
							layout.Rigid(func(gtx C) D {
								if !mp.isBalanceHidden {
									return mp.LayoutUSDBalance(gtx)
								}
								return D{}
							}),
						)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return mp.Theme.Separator().Layout(gtx)
		}),
	)
}

func (mp *MainPage) LayoutUSDBalance(gtx C) D {
	if !mp.usdExchangeSet {
		return D{}
	}
	switch {
	case mp.isFetchingExchangeRate && mp.usdExchangeRate == 0:
		gtx.Constraints.Max.Y = gtx.Dp(values.MarginPadding18)
		gtx.Constraints.Max.X = gtx.Constraints.Max.Y
		return layout.Inset{
			Top:  values.MarginPadding8,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			loader := material.Loader(mp.Theme.Base)
			return loader.Layout(gtx)
		})
	case !mp.isFetchingExchangeRate && mp.usdExchangeRate == 0:
		return layout.Inset{
			Top:  values.MarginPadding7,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			return mp.refreshExchangeRateBtn.Layout(gtx, func(gtx C) D {
				return mp.Theme.Icons.Restore.Layout16dp(gtx)
			})
		})
	case len(mp.totalBalanceUSD) > 0:
		lbl := mp.Theme.Label(values.TextSize20, fmt.Sprintf("/ %s", mp.totalBalanceUSD))
		lbl.Color = mp.Theme.Color.PageNavText
		inset := layout.Inset{Left: values.MarginPadding8}
		return inset.Layout(gtx, lbl.Layout)
	default:
		return D{}
	}
}

func (mp *MainPage) totalAssetBalance(gtx C) D {
	if mp.isBalanceHidden || mp.totalBalance == nil {
		hiddenBalanceText := mp.Theme.Label(values.TextSize18*0.8, "*******************")
		return layout.Inset{Bottom: values.MarginPadding0, Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
			hiddenBalanceText.Color = mp.Theme.Color.PageNavText
			return hiddenBalanceText.Layout(gtx)
		})
	}
	return components.LayoutBalanceWithUnit(gtx, mp.Load, mp.totalBalance.String())
}

func (mp *MainPage) postTransactionNotification(t *sharedW.Transaction) {
	var notification string
	wal := mp.selectedWallet
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

	if mp.WL.AssetsManager.OpenedWalletsCount() > 1 {
		notification = fmt.Sprintf("[%s] %s", wal.GetWalletName(), notification)
	}

	initializeBeepNotification(notification)
}

func (mp *MainPage) postProposalNotification(propName string, status libutils.ProposalStatus) {
	proposalNotification := mp.selectedWallet.ReadBoolConfigValueForKey(sharedW.ProposalNotificationConfigKey, false) ||
		!mp.WL.AssetsManager.IsPrivacyModeOn()
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
func (mp *MainPage) listenForNotifications() {
	syncProgressListener := &sharedW.SyncProgressListener{
		OnSyncCompleted: func() {
			mp.updateBalance()
			mp.ParentWindow().Reload()
		},
	}
	err := mp.selectedWallet.AddSyncProgressListener(syncProgressListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(transaction *sharedW.Transaction) {
			mp.updateBalance()
			if mp.WL.AssetsManager.IsTransactionNotificationsOn() {
				// TODO: SPV wallets only receive mempool tx ntfn for txs that
				// were broadcast by the wallet. We should probably be posting
				// desktop ntfns for txs received from external parties, which
				// will can be gotten from the OnTransactionConfirmed callback.
				mp.postTransactionNotification(transaction)
			}
			mp.ParentWindow().Reload()
		},
		// OnBlockAttached is also called whenever OnTransactionConfirmed is
		// called, so use OnBlockAttached. Also, OnTransactionConfirmed may be
		// called multiple times whereas OnBlockAttached is only called once.
		OnBlockAttached: func(walletID int, blockHeight int32) {
			beep := mp.selectedWallet.ReadBoolConfigValueForKey(sharedW.BeepNewBlocksConfigKey, false)
			if beep {
				err := beeep.Beep(5, 1)
				if err != nil {
					log.Error(err.Error)
				}
			}

			mp.updateBalance()
			mp.ParentWindow().Reload()
		},
	}
	err = mp.selectedWallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	if mp.isGovernanceAPIAllowed() {
		proposalSyncCallback := func(propName string, status libutils.ProposalStatus) {
			// Post desktop notification for all events except the synced event.
			if status != libutils.ProposalStatusSynced {
				mp.postProposalNotification(propName, status)
			}
		}
		err = mp.WL.AssetsManager.Politeia.AddSyncCallback(proposalSyncCallback, MainPageID)
		if err != nil {
			log.Errorf("Error adding politeia notification listener: %v", err)
			return
		}
	}

	// TODO: Register trade order ntfn listener and post desktop ntfns for all
	// events except the synced event.
}

func (mp *MainPage) stopNtfnListeners() {
	mp.selectedWallet.RemoveSyncProgressListener(MainPageID)
	mp.selectedWallet.RemoveTxAndBlockNotificationListener(MainPageID)
	mp.WL.AssetsManager.Politeia.RemoveSyncCallback(MainPageID)
}

func (mp *MainPage) showBackupInfo() {
	backupNowOrLaterModal := modal.NewCustomModal(mp.Load).
		SetupWithTemplate(modal.WalletBackupInfoTemplate).
		SetCancelable(false).
		SetContentAlignment(layout.W, layout.W, layout.Center).
		CheckBox(mp.checkBox, true).
		SetNegativeButtonText(values.String(values.StrBackupLater)).
		SetNegativeButtonCallback(func() {
			mp.selectedWallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, true)
		}).
		PositiveButtonStyle(mp.Load.Theme.Color.Primary, mp.Load.Theme.Color.InvText).
		SetPositiveButtonText(values.String(values.StrBackupNow)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			mp.selectedWallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, true)
			mp.ParentNavigator().Display(seedbackup.NewBackupInstructionsPage(mp.Load, mp.selectedWallet, func(load *load.Load, navigator app.WindowNavigator) {
				navigator.ClosePagesAfter(mp.ParentWindow().CurrentPageID())
			}))
			return true
		})
	mp.ParentWindow().ShowModal(backupNowOrLaterModal)
}
