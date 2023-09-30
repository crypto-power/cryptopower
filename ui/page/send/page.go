package send

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	SendPageID = "Send"

	// MaxTxLabelSize defines the maximum number of characters to be allowed on
	// txLabelInputEditor component.
	MaxTxLabelSize = 100
)

var (
	automaticCoinSelection = values.String(values.StrAutomatic)
	manualCoinSelection    = values.String(values.StrManual)
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

	toCoinSelection *cryptomaterial.Clickable

	selectedUTXOs selectedUTXOsInfo
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

type selectedUTXOsInfo struct {
	sourceAccount    *sharedW.Account
	selectedUTXOs    []*sharedW.UnspentOutput
	totalUTXOsAmount int64
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
	}
	pg.selectedWallet = &load.WalletMapping{
		Asset: l.WL.SelectedWallet.Wallet,
	}

	callbackFunc := func() libUtil.AssetType {
		return pg.selectedWallet.GetAssetType()
	}
	pg.feeRateSelector = components.NewFeeRateSelector(l, callbackFunc).ShowSizeAndCost()
	pg.feeRateSelector.TitleInset = layout.Inset{Bottom: values.MarginPadding10}
	pg.feeRateSelector.ContainerInset = layout.Inset{Bottom: values.MarginPadding100}
	pg.feeRateSelector.WrapperInset = layout.UniformInset(values.MarginPadding15)

	// Source account picker
	pg.sourceAccountSelector = components.NewWalletAndAccountSelector(l).
		Title(values.String(values.StrFrom)).
		AccountSelected(func(selectedAccount *sharedW.Account) {
			// this resets the selected destination account based on the
			// selected source account. This is done to prevent sending to an
			// account that is invalid either because the destination account is
			// the same as the source account or because the destination account
			// needs to change based on if the selected wallet has privacy
			// enabled.
			pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(
				pg.sendDestination.destinationWalletSelector.SelectedWallet())
			pg.validateAndConstructTx()
		}).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !pg.selectedWallet.IsWatchingOnlyWallet()

			if pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
				!pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) {
				// Spending unmixed fund isn't permitted for the selected wallet.

				// Only mixed accounts can send to address/wallets for wallet
				// with privacy setup.
				switch pg.sendDestination.accountSwitch.SelectedIndex() {
				case sendToAddress:
					accountIsValid = account.Number == pg.selectedWallet.MixedAccountNumber()
				case SendToWallet:
					destinationWalletID := pg.sendDestination.destinationWalletSelector.SelectedWallet().GetWalletID()
					if destinationWalletID != pg.selectedWallet.GetWalletID() {
						accountIsValid = account.Number == pg.selectedWallet.MixedAccountNumber()
					}
				}
			}
			return accountIsValid
		}).
		SetActionInfoText(values.String(values.StrTxConfModalInfoTxt))

	// if a source account exists, don't overwrite it.
	if pg.sourceAccountSelector.SelectedAccount() == nil {
		pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)
	}

	pg.sendDestination.destinationAccountSelector = pg.sendDestination.destinationAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
		accountIsValid := account.Number != load.MaxInt32
		// Filter mixed wallet
		destinationWallet := pg.sendDestination.destinationAccountSelector.SelectedWallet()
		isMixedAccount := destinationWallet.MixedAccountNumber() == account.Number
		// Filter the sending account.
		sourceWalletID := pg.sourceAccountSelector.SelectedAccount().WalletID
		isSameAccount := sourceWalletID == account.WalletID && account.Number == pg.sourceAccountSelector.SelectedAccount().Number
		if !accountIsValid || isSameAccount || isMixedAccount {
			return false
		}
		return true
	})

	pg.sendDestination.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		pg.validateAndConstructTx()
	})

	pg.sendDestination.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		if pg.selectedWallet.Asset.GetAssetType() == libUtil.DCRWalletAsset {
			pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)
		}
	})

	pg.sendDestination.addressChanged = func() {
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

func (pg *Page) UpdateSelectedUTXOs(utxos []*sharedW.UnspentOutput) {
	pg.selectedUTXOs = selectedUTXOsInfo{
		selectedUTXOs: utxos,
		sourceAccount: pg.sourceAccountSelector.SelectedAccount(),
	}
	if len(utxos) > 0 {
		for _, elem := range utxos {
			pg.selectedUTXOs.totalUTXOsAmount += elem.Amount.ToInt()
		}
	}
}

// OnNavigatedTo is called when the page is about to be displayed and may be
// used to initialize page features that are only relevant when the page is
// displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	pg.RestyleWidgets()

	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	if !pg.WL.SelectedWallet.Wallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.sourceAccountSelector.ListenForTxNotifications(pg.ctx, pg.ParentWindow())
	// destinationAccountSelector does not have a default value, so assign it an
	// initial value here
	pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(pg.sendDestination.destinationWalletSelector.SelectedWallet())
	pg.sendDestination.destinationAddressEditor.Editor.Focus()

	pg.usdExchangeSet = false
	if components.IsFetchExchangeRateAPIAllowed(pg.WL) {
		pg.currencyExchange = pg.WL.AssetsManager.GetCurrencyConversionExchange()
		pg.usdExchangeSet = true
		go pg.fetchExchangeRate()
	} else {
		// If exchange rate is not supported, validate and construct the TX.
		pg.validateAndConstructTx()
	}

	if pg.selectedWallet.GetAssetType() == libUtil.BTCWalletAsset && pg.isFeerateAPIApproved() {
		// This API call may take sometime to return. Call this before and cache
		// results.
		go pg.selectedWallet.GetAPIFeeRate()
	}
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed to
// enable restyling UI elements where necessary.
// Satisfies the load.DarkModeChangeHandler interface.
func (pg *Page) OnDarkModeChanged(_ bool) {
	pg.amount.styleWidgets()
}

func (pg *Page) fetchExchangeRate() {
	if pg.isFetchingExchangeRate {
		return
	}
	pg.isFetchingExchangeRate = true
	var market string
	switch pg.WL.SelectedWallet.Wallet.GetAssetType() {
	case libUtil.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libUtil.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libUtil.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", pg.WL.SelectedWallet.Wallet.GetAssetType())
		pg.isFetchingExchangeRate = false
		return
	}

	rate, err := pg.WL.AssetsManager.ExternalService.GetTicker(pg.currencyExchange, market)
	if err != nil {
		log.Error(err)
		pg.isFetchingExchangeRate = false
		return
	}

	pg.exchangeRate = rate.LastTradePrice
	pg.amount.setExchangeRate(pg.exchangeRate)
	pg.validateAndConstructTx() // convert estimates to usd

	pg.isFetchingExchangeRate = false
	pg.ParentWindow().Reload()
}

func (pg *Page) validateAndConstructTx() {
	// delete all the previous errors set earlier.
	pg.amountValidationError("")
	pg.addressValidationError("")

	if pg.validate() {
		pg.constructTx()
	} else {
		pg.clearEstimates()
		pg.showBalanceAfterSend()
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

	// No need for checking the err message since it is as result of amount and
	// address validation. validForSending
	return amountIsValid && addressIsValid
}

func (pg *Page) constructTx() {
	destinationAddress, err := pg.sendDestination.destinationAddress()
	if err != nil {
		pg.addressValidationError(err.Error())
		return
	}
	destinationAccount := pg.sendDestination.destinationAccount()

	amountAtom, SendMax, err := pg.amount.validAmount()
	if err != nil {
		pg.amountValidationError(err.Error())
		return
	}

	sourceAccount := pg.sourceAccountSelector.SelectedAccount()
	selectedUTXOs := make([]*sharedW.UnspentOutput, 0)
	if sourceAccount == pg.selectedUTXOs.sourceAccount {
		selectedUTXOs = pg.selectedUTXOs.selectedUTXOs
	}

	err = pg.selectedWallet.NewUnsignedTx(sourceAccount.Number, selectedUTXOs)
	if err != nil {
		pg.amountValidationError(err.Error())
		return
	}

	err = pg.selectedWallet.AddSendDestination(destinationAddress, amountAtom, SendMax)
	if err != nil {
		if strings.Contains(err.Error(), "amount") {
			pg.amountValidationError(err.Error())
			return
		}
		pg.addressValidationError(err.Error())
		return
	}

	feeAndSize, err := pg.selectedWallet.EstimateFeeAndSize()
	if err != nil {
		pg.amountValidationError(err.Error())
		return
	}

	feeAtom := feeAndSize.Fee.UnitValue
	spendableAmount := sourceAccount.Balance.Spendable.ToInt()
	if len(selectedUTXOs) > 0 {
		spendableAmount = pg.selectedUTXOs.totalUTXOsAmount
	}

	if SendMax {
		amountAtom = spendableAmount - feeAtom
	}

	wal := pg.WL.SelectedWallet.Wallet
	totalSendingAmount := wal.ToAmount(amountAtom + feeAtom)
	balanceAfterSend := wal.ToAmount(spendableAmount - totalSendingAmount.ToInt())

	// populate display data
	pg.txFee = wal.ToAmount(feeAtom).String()

	pg.feeRateSelector.EstSignedSize = fmt.Sprintf("%d Bytes", feeAndSize.EstimatedSignedSize)
	pg.feeRateSelector.TxFee = pg.txFee
	pg.feeRateSelector.SetFeerate(feeAndSize.FeeRate)
	pg.totalCost = totalSendingAmount.String()
	pg.balanceAfterSend = balanceAfterSend.String()
	pg.sendAmount = wal.ToAmount(amountAtom).String()
	pg.destinationAddress = destinationAddress
	pg.destinationAccount = destinationAccount
	pg.sourceAccount = sourceAccount

	if SendMax {
		// TODO: this workaround ignores the change events from the amount input
		// to avoid construct tx cycle.
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

func (pg *Page) showBalanceAfterSend() {
	if pg.sourceAccountSelector != nil {
		sourceAccount := pg.sourceAccountSelector.SelectedAccount()
		if sourceAccount.Balance == nil {
			return
		}
		balanceAfterSend := sourceAccount.Balance.Spendable
		pg.balanceAfterSend = balanceAfterSend.String()
		pg.balanceAfterSendUSD = utils.FormatUSDBalance(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))
	}
}

func (pg *Page) amountValidationError(err string) {
	pg.amount.setError(err)
	pg.clearEstimates()
}

func (pg *Page) addressValidationError(err string) {
	pg.sendDestination.setError(err)
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
	pg.feeRateSelector.SetFeerate(0)
}

func (pg *Page) resetFields() {
	pg.sendDestination.clearAddressInput()
	pg.txLabelInputEditor.Editor.SetText("")

	pg.amount.resetFields()
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions() {
	if pg.feeRateSelector.FetchRates.Clicked() {
		go pg.feeRateSelector.FetchFeeRate(pg.ParentWindow(), pg.selectedWallet)
	}

	if pg.feeRateSelector.EditRates.Clicked() {
		pg.feeRateSelector.OnEditRateClicked(pg.selectedWallet)
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

	if pg.toCoinSelection.Clicked() {
		_, err := pg.sendDestination.destinationAddress()
		if err != nil {
			pg.addressValidationError(err.Error())
			pg.sendDestination.destinationAddressEditor.Editor.Focus()
		} else {
			pg.ParentNavigator().Display(NewManualCoinSelectionPage(pg.Load, pg))
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
func (pg *Page) HandleKeyPress(_ *key.Event) {}

// OnNavigatedFrom is called when the page is about to be removed from the
// displayed window. This method should ideally be used to disable features that
// are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {
	pg.ctxCancel() // causes crash if nil, when the main page is closed if send page is created but never displayed (because sync in progress)
}

func (pg *Page) isFeerateAPIApproved() bool {
	return pg.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libUtil.FeeRateHTTPAPI)
}
