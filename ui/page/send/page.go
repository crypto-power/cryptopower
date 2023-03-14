package send

import (
	"context"
	"fmt"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/app"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libUtil "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/utils"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const (
	SendPageID   = "Send"
	SendToWallet = 2

	// MaxTxLabelSize defines the maximum number of characters to be allowed on
	// txLabelInputEditor component.
	MaxTxLabelSize = 100
)

var (
	defaultCoinSelection = values.String(values.StrAutomatic)
	option1CoinSelection = values.String(values.StrManual)
)

type Page struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	pageContainer *widget.List

	sourceAccountSelector *components.WalletAndAccountSelector
	sendDestination       *destination
	amount                *sendAmount

	infoButton    cryptomaterial.IconButton
	retryExchange cryptomaterial.Button
	nextButton    cryptomaterial.Button

	shadowBox *cryptomaterial.Shadow
	backdrop  *widget.Clickable

	isFetchingExchangeRate bool

	exchangeRate        float64
	usdExchangeSet      bool
	exchangeRateMessage string
	confirmTxModal      *sendConfirmModal
	currencyExchange    string

	txLabelInputEditor cryptomaterial.Editor

	*authoredTxData
	selectedWallet  *load.WalletMapping
	feeRateSelector *components.FeeRateSelector

	coinSelectionCollapsible *cryptomaterial.Collapsible
	coinSelectionButton      cryptomaterial.Button
	selectedOption           string
}

type authoredTxData struct {
	destinationAddress  string
	destinationAccount  *sharedW.Account
	sourceAccount       *sharedW.Account
	txFee               string
	txFeeUSD            string
	totalCost           string
	totalCostUSD        string
	balanceAfterSend    string
	balanceAfterSendUSD string
	sendAmount          string
	sendAmountUSD       string
}

func NewSendPage(l *load.Load) *Page {
	pg := &Page{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(SendPageID),
		sendDestination:  newSendDestination(l),
		amount:           newSendAmount(l),

		exchangeRate: -1,

		authoredTxData: &authoredTxData{},
		shadowBox:      l.Theme.Shadow(),
		backdrop:       new(widget.Clickable),

		coinSelectionCollapsible: l.Theme.Collapsible(),
		coinSelectionButton:      l.Theme.OutlineButton(defaultCoinSelection),
		selectedOption:           defaultCoinSelection, // holds the default option until changed.
	}
	pg.selectedWallet = &load.WalletMapping{
		Asset: l.WL.SelectedWallet.Wallet,
	}

	pg.feeRateSelector = components.NewFeeRateSelector(l).ShowSizeAndCost()
	pg.feeRateSelector.TitleInset = layout.Inset{Bottom: values.MarginPadding10}
	pg.feeRateSelector.ContainerInset = layout.Inset{Bottom: values.MarginPadding100}
	pg.feeRateSelector.WrapperInset = layout.UniformInset(values.MarginPadding15)

	// Source account picker
	pg.sourceAccountSelector = components.NewWalletAndAccountSelector(l).
		Title(values.String(values.StrFrom)).
		AccountSelected(func(selectedAccount *sharedW.Account) {
			pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(
				pg.sendDestination.destinationWalletSelector.SelectedWallet())
			pg.validateAndConstructTx()
		}).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !pg.selectedWallet.IsWatchingOnlyWallet()

			if pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
				!pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) {
				// Spending unmixed fund isn't permitted for the selected wallet

				// only mixed accounts can send to address for wallet with privacy setup
				if pg.sendDestination.accountSwitch.SelectedIndex() == 1 {
					accountIsValid = account.Number == pg.selectedWallet.MixedAccountNumber()
				}
			}
			return accountIsValid
		}).
		SetActionInfoText(values.String(values.StrTxConfModalInfoTxt))
	pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)

	pg.sendDestination.destinationAccountSelector = pg.sendDestination.destinationAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
		accountIsValid := account.Number != load.MaxInt32
		// Filter mixed wallet
		destinationWallet := pg.sendDestination.destinationAccountSelector.SelectedWallet()
		isMixedAccount := destinationWallet.MixedAccountNumber() == account.Number
		// Filter the sending account.
		sourceWalletId := pg.sourceAccountSelector.SelectedAccount().WalletID
		isSameAccount := sourceWalletId == account.WalletID && account.Number == pg.sourceAccountSelector.SelectedAccount().Number
		if !accountIsValid || isSameAccount || isMixedAccount {
			return false
		}
		return true
	})

	pg.sendDestination.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		pg.validateAndConstructTx()
	})

	pg.sendDestination.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.sourceAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32

			if pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
				!pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) {
				if pg.sendDestination.accountSwitch.SelectedIndex() == SendToWallet {
					destinationWalletId := pg.sendDestination.destinationAccountSelector.SelectedAccount().WalletID
					if destinationWalletId != pg.selectedWallet.GetWalletID() {
						accountIsValid = account.Number == pg.selectedWallet.MixedAccountNumber()
					}
				} else {
					accountIsValid = account.Number == pg.selectedWallet.MixedAccountNumber()
				}
			}
			return accountIsValid
		})
		acc, _ := pg.selectedWallet.GetAccountsRaw()
		for _, acc := range acc.Accounts {
			if acc.Number == pg.selectedWallet.MixedAccountNumber() {
				pg.sourceAccountSelector.SetSelectedAccount(acc)
			}
		}
		pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	pg.sendDestination.addressChanged = func() {
		// refresh selected account when addressChanged is called
		pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)
		pg.validateAndConstructTx()
	}

	pg.amount.amountChanged = func() {
		pg.validateAndConstructTxAmountOnly()
	}

	pg.initLayoutWidgets()

	return pg
}

// RestyleWidgets restyles select widgets to match the current theme. This is
// especially necessary when the dark mode setting is changed.
func (pg *Page) RestyleWidgets() {
	pg.amount.styleWidgets()
	pg.sendDestination.styleWidgets()
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	pg.RestyleWidgets()

	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	if !pg.WL.SelectedWallet.Wallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.sourceAccountSelector.ListenForTxNotifications(pg.ctx, pg.ParentWindow())
	pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(pg.sendDestination.destinationWalletSelector.SelectedWallet())
	pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)
	pg.sendDestination.destinationAddressEditor.Editor.Focus()

	pg.usdExchangeSet = false
	if components.IsFetchExchangeRateAPIAllowed(pg.WL) {
		pg.currencyExchange = pg.WL.AssetsManager.GetCurrencyConversionExchange()
		pg.usdExchangeSet = true
		go pg.fetchExchangeRate()
	}

	if pg.selectedWallet.GetAssetType() == libUtil.BTCWalletAsset && pg.isFeerateAPIApproved() {
		// This API call may take sometime to return. Call this before and cache
		// results.
		go pg.selectedWallet.GetAPIFeeRate()
	}
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed
// to enable restyling UI elements where necessary.
// Satisfies the load.DarkModeChangeHandler interface.
func (pg *Page) OnDarkModeChanged(isDarkModeOn bool) {
	pg.amount.styleWidgets()
}

func (pg *Page) fetchExchangeRate() {
	if pg.isFetchingExchangeRate {
		return
	}
	pg.isFetchingExchangeRate = true
	market := values.DCRUSDTMarket
	if pg.selectedWallet.Asset.GetAssetType() == libUtil.BTCWalletAsset {
		market = values.BTCUSDTMarket
	}
	rate, err := pg.WL.AssetsManager.ExternalService.GetTicker(pg.currencyExchange, market)
	if err != nil {
		log.Errorf("Error fetching exchange rate : %v", err)
		return
	}

	pg.exchangeRate = rate.LastTradePrice
	pg.amount.setExchangeRate(pg.exchangeRate)
	pg.validateAndConstructTx() // convert estimates to usd

	pg.isFetchingExchangeRate = false
	pg.ParentWindow().Reload()
}

func (pg *Page) validateAndConstructTx() {
	if pg.validate() {
		pg.constructTx()
	} else {
		pg.clearEstimates()
		pg.showBalaceAfterSend()
	}
}

func (pg *Page) validateAndConstructTxAmountOnly() {
	defer pg.RefreshTheme(pg.ParentWindow())

	if !pg.sendDestination.validate() && pg.amount.amountIsValid() {
		pg.constructTx()
	} else {
		pg.validateAndConstructTx()
	}
}

func (pg *Page) validate() bool {
	amountIsValid := pg.amount.amountIsValid()
	addressIsValid := pg.sendDestination.validate()
	noErrMsg := pg.amount.amountErrorText == ""

	// validForSending
	return amountIsValid && addressIsValid && noErrMsg
}

func (pg *Page) constructTx() {
	destinationAddress, err := pg.sendDestination.destinationAddress()
	if err != nil {
		pg.feeEstimationError(err.Error())
		return
	}
	destinationAccount := pg.sendDestination.destinationAccount()

	amountAtom, SendMax, err := pg.amount.validAmount()
	if err != nil {
		pg.feeEstimationError(err.Error())
		return
	}

	sourceAccount := pg.sourceAccountSelector.SelectedAccount()
	err = pg.selectedWallet.NewUnsignedTx(sourceAccount.Number)
	if err != nil {
		pg.feeEstimationError(err.Error())
		return
	}

	err = pg.selectedWallet.AddSendDestination(destinationAddress, amountAtom, SendMax)
	if err != nil {
		pg.feeEstimationError(err.Error())
		return
	}

	feeAndSize, err := pg.selectedWallet.EstimateFeeAndSize()
	if err != nil {
		pg.feeEstimationError(err.Error())
		return
	}

	feeAtom := feeAndSize.Fee.UnitValue
	if SendMax {
		amountAtom = sourceAccount.Balance.Spendable.ToInt() - feeAtom
	}

	wal := pg.WL.SelectedWallet.Wallet
	totalSendingAmount := wal.ToAmount(amountAtom + feeAtom)
	balanceAfterSend := wal.ToAmount(sourceAccount.Balance.Spendable.ToInt() - totalSendingAmount.ToInt())

	// populate display data
	pg.txFee = wal.ToAmount(feeAtom).String()

	pg.feeRateSelector.EstSignedSize = fmt.Sprintf("%d Bytes", feeAndSize.EstimatedSignedSize)
	pg.feeRateSelector.TxFee = pg.txFee
	pg.totalCost = totalSendingAmount.String()
	pg.balanceAfterSend = balanceAfterSend.String()
	pg.sendAmount = wal.ToAmount(amountAtom).String()
	pg.destinationAddress = destinationAddress
	pg.destinationAccount = destinationAccount
	pg.sourceAccount = sourceAccount

	if SendMax {
		// TODO: this workaround ignores the change events from the
		// amount input to avoid construct tx cycle.
		pg.amount.setAmount(amountAtom)
	}

	if pg.exchangeRate != -1 && pg.usdExchangeSet {
		pg.feeRateSelector.USDExchangeSet = true
		pg.txFeeUSD = fmt.Sprintf("$%.4f", utils.CryptoToUSD(pg.exchangeRate, feeAndSize.Fee.CoinValue))
		pg.feeRateSelector.TxFeeUSD = pg.txFeeUSD
		pg.totalCostUSD = utils.FormatUSDBalance(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, totalSendingAmount.ToCoin()))
		pg.balanceAfterSendUSD = utils.FormatUSDBalance(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))

		usdAmount := utils.CryptoToUSD(pg.exchangeRate, wal.ToAmount(amountAtom).ToCoin())
		pg.sendAmountUSD = utils.FormatUSDBalance(pg.Printer, usdAmount)
	}
}

func (pg *Page) showBalaceAfterSend() {
	if pg.sourceAccountSelector != nil {
		sourceAccount := pg.sourceAccountSelector.SelectedAccount()
		balanceAfterSend := sourceAccount.Balance.Spendable
		pg.balanceAfterSend = balanceAfterSend.String()
		pg.balanceAfterSendUSD = utils.FormatUSDBalance(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))
	}
}

func (pg *Page) feeEstimationError(err string) {
	pg.amount.setError(err)
	pg.clearEstimates()
}

func (pg *Page) clearEstimates() {
	pg.txFee = " - " + string(pg.selectedWallet.GetAssetType())
	pg.feeRateSelector.TxFee = pg.txFee
	pg.txFeeUSD = " - "
	pg.feeRateSelector.TxFeeUSD = pg.txFeeUSD
	pg.totalCost = " - " + string(pg.selectedWallet.GetAssetType())
	pg.totalCostUSD = " - "
	pg.balanceAfterSend = " - " + string(pg.selectedWallet.GetAssetType())
	pg.balanceAfterSendUSD = " - "
	pg.sendAmount = " - "
	pg.sendAmountUSD = " - "
}

func (pg *Page) resetFields() {
	pg.sendDestination.clearAddressInput()
	pg.txLabelInputEditor.Editor.SetText("")

	pg.amount.resetFields()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions() {
	if pg.feeRateSelector.FetchRates.Clicked() {
		go pg.feeRateSelector.FetchFeeRate(pg.ParentWindow(), pg.selectedWallet)
	}
	if pg.feeRateSelector.EditRates.Clicked() {
		pg.feeRateSelector.OnEditRateCliked(pg.selectedWallet)
	}
	pg.nextButton.SetEnabled(pg.validate())
	pg.sendDestination.handle()
	pg.amount.handle()

	if pg.infoButton.Button.Clicked() {
		textWithUnit := values.String(values.StrSend) + " " + string(pg.selectedWallet.GetAssetType())
		info := modal.NewCustomModal(pg.Load).
			Title(textWithUnit).
			Body(values.String(values.StrSendInfo)).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(info)
	}

	if pg.retryExchange.Clicked() {
		go pg.fetchExchangeRate()
	}

	if pg.coinSelectionButton.Clicked() {
		pg.selectedOption = pg.coinSelectionButton.Text

		// if manual has been selected, navigate to the manual utxo selection page.
		if pg.selectedOption == option1CoinSelection {
			pg.ParentWindow().Display(NewManualCoinSelectionPage(pg.Load,
				pg.feeRateSelector.EstSignedSize, pg.totalCost))
		}
	}

	if pg.nextButton.Clicked() {
		if pg.selectedWallet.IsUnsignedTxExist() {
			pg.confirmTxModal = newSendConfirmModal(pg.Load, pg.authoredTxData, *pg.selectedWallet)
			pg.confirmTxModal.exchangeRateSet = pg.exchangeRate != -1 && pg.usdExchangeSet
			pg.confirmTxModal.txLabel = pg.txLabelInputEditor.Editor.Text()

			pg.confirmTxModal.txSent = func() {
				pg.resetFields()
				pg.clearEstimates()
			}

			pg.ParentWindow().ShowModal(pg.confirmTxModal)
		}
	}

	modalShown := pg.confirmTxModal != nil && pg.confirmTxModal.IsShown()
	isAmountEditorActive := pg.amount.amountEditor.Editor.Focused() ||
		pg.amount.usdAmountEditor.Editor.Focused()

	if !modalShown && !isAmountEditorActive {
		isSendToWallet := pg.sendDestination.accountSwitch.SelectedIndex() == 2
		isDestinationEditorFocused := pg.sendDestination.destinationAddressEditor.Editor.Focused()

		switch {
		// If accounts switch selects the wallet option.
		case isSendToWallet && !isDestinationEditorFocused:
			pg.amount.amountEditor.Editor.Focus()
		}
	}

	// if destination switch is equal to Address
	if pg.sendDestination.sendToAddress {
		if pg.sendDestination.validate() {
			if !components.IsFetchExchangeRateAPIAllowed(pg.WL) {
				if len(pg.amount.amountEditor.Editor.Text()) == 0 {
					pg.amount.SendMax = false
				}
			} else {
				if len(pg.amount.amountEditor.Editor.Text()) == 0 {
					pg.amount.usdAmountEditor.Editor.SetText("")
					pg.amount.SendMax = false
				}
			}
		}
	} else {
		if !components.IsFetchExchangeRateAPIAllowed(pg.WL) {
			if len(pg.amount.amountEditor.Editor.Text()) == 0 {
				pg.amount.SendMax = false
			}
		} else {
			if len(pg.amount.amountEditor.Editor.Text()) == 0 {
				pg.amount.usdAmountEditor.Editor.SetText("")
				pg.amount.SendMax = false
			}
		}
	}

	if len(pg.amount.amountEditor.Editor.Text()) > 0 && pg.sourceAccountSelector.Changed() {
		pg.amount.validateAmount()
		pg.validateAndConstructTxAmountOnly()
	}

	if pg.amount.IsMaxClicked() {
		pg.amount.setError("")
		pg.amount.SendMax = true
		pg.amount.amountChanged()
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *Page) KeysToHandle() key.Set {
	return cryptomaterial.AnyKeyWithOptionalModifier(key.ModShift, key.NameTab)
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *Page) HandleKeyPress(evt *key.Event) {}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {
	pg.ctxCancel() // causes crash if nil, when the main page is closed if send page is created but never displayed (because sync in progress)
}

func (pg *Page) isFeerateAPIApproved() bool {
	return pg.WL.AssetsManager.IsHttpAPIPrivacyModeOff(libUtil.FeeRateHttpAPI)
}
