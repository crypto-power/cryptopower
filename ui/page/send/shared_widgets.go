package send

import (
	"fmt"
	"strings"

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
	// MaxTxLabelSize defines the maximum number of characters to be allowed on
	// txLabelInputEditor component.
	MaxTxLabelSize = 100
)

var (
	automaticCoinSelection = values.String(values.StrAutomatic)
	manualCoinSelection    = values.String(values.StrManual)
)

type sharedProperties struct {
	*load.Load

	parentWindow  app.WindowNavigator
	pageContainer *widget.List

	sourceWalletSelector  *components.WalletAndAccountSelector
	sourceAccountSelector *components.WalletAndAccountSelector
	sendDestination       *destination
	amount                *sendAmount

	infoButton    cryptomaterial.IconButton
	retryExchange cryptomaterial.Button
	nextButton    cryptomaterial.Button

	shadowBox *cryptomaterial.Shadow
	backdrop  *widget.Clickable

	isFetchingExchangeRate bool
	isModalLayout          bool

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

func newSharedProperties(l *load.Load, isModalLayout bool) *sharedProperties {
	wi := &sharedProperties{
		Load:         l,
		exchangeRate: -1,

		authoredTxData: &authoredTxData{},
		shadowBox:      l.Theme.Shadow(),
		backdrop:       new(widget.Clickable),
		isModalLayout:  isModalLayout,
	}

	if isModalLayout {
		wi.initWalletSelector()
	} else {
		wi.selectedWallet = &load.WalletMapping{
			Asset: l.WL.SelectedWallet.Wallet,
		}
	}

	wi.amount = newSendAmount(l.Theme, wi.selectedWallet.GetAssetType())
	wi.sendDestination = newSendDestination(l, wi.selectedWallet.Asset)

	callbackFunc := func() libUtil.AssetType {
		return wi.selectedWallet.GetAssetType()
	}
	wi.feeRateSelector = components.NewFeeRateSelector(l, callbackFunc).ShowSizeAndCost()
	wi.feeRateSelector.TitleInset = layout.Inset{Bottom: values.MarginPadding10}
	wi.feeRateSelector.ContainerInset = layout.Inset{Bottom: values.MarginPadding100}
	wi.feeRateSelector.WrapperInset = layout.UniformInset(values.MarginPadding15)

	wi.initializeAccountSelectors()

	wi.sendDestination.addressChanged = func() {
		wi.validateAndConstructTx()
	}

	wi.amount.amountChanged = func() {
		wi.validateAndConstructTxAmountOnly()
	}

	wi.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	buttonInset := layout.Inset{
		Top:    values.MarginPadding4,
		Right:  values.MarginPadding8,
		Bottom: values.MarginPadding4,
		Left:   values.MarginPadding8,
	}

	wi.nextButton = wi.Theme.Button(values.String(values.StrNext))
	wi.nextButton.TextSize = values.TextSize18
	wi.nextButton.Inset = layout.Inset{Top: values.MarginPadding15, Bottom: values.MarginPadding15}
	wi.nextButton.SetEnabled(false)

	_, wi.infoButton = components.SubpageHeaderButtons(wi.Load)

	wi.retryExchange = wi.Theme.Button(values.String(values.StrRetry))
	wi.retryExchange.Background = wi.Theme.Color.Gray1
	wi.retryExchange.Color = wi.Theme.Color.Surface
	wi.retryExchange.TextSize = values.TextSize12
	wi.retryExchange.Inset = buttonInset

	wi.txLabelInputEditor = wi.Theme.Editor(new(widget.Editor), values.String(values.StrNote))
	wi.txLabelInputEditor.Editor.SingleLine = false
	wi.txLabelInputEditor.Editor.SetText("")
	// Set the maximum characters the editor can accept.
	wi.txLabelInputEditor.Editor.MaxLen = MaxTxLabelSize

	wi.toCoinSelection = wi.Theme.NewClickable(false)

	return wi
}

// RestyleWidgets restyles select widgets to match the current theme. This is
// especially necessary when the dark mode setting is changed.
func (wi *sharedProperties) restyleWidgets() {
	wi.amount.styleWidgets()
	wi.sendDestination.styleWidgets()
}

func (wi *sharedProperties) onLoaded() {
	wi.restyleWidgets()

	if !wi.selectedWallet.Asset.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	// destinationAccountSelector does not have a default value,
	// so assign it an initial value here
	wi.sendDestination.destinationAccountSelector.SelectFirstValidAccount(wi.sendDestination.destinationWalletSelector.SelectedWallet())
	wi.sendDestination.destinationAddressEditor.Editor.Focus()

	wi.usdExchangeSet = false
	if components.IsFetchExchangeRateAPIAllowed(wi.WL) {
		wi.currencyExchange = wi.WL.AssetsManager.GetCurrencyConversionExchange()
		wi.usdExchangeSet = true
		go wi.fetchExchangeRate()
	} else {
		// If exchange rate is not supported, validate and construct the TX.
		wi.validateAndConstructTx()
	}

	if wi.selectedWallet.GetAssetType() == libUtil.BTCWalletAsset && wi.isFeerateAPIApproved() {
		// This API call may take sometime to return. Call this before and cache
		// results.
		go wi.selectedWallet.GetAPIFeeRate()
	}
}

// initWalletSelector is used for the send modal to for wallet selection.
func (wi *sharedProperties) initWalletSelector() {
	// initialize wallet selector
	wi.sourceWalletSelector = components.NewWalletAndAccountSelector(wi.Load).
		Title(values.String(values.StrSelectWallet))
	wi.selectedWallet = wi.sourceWalletSelector.SelectedWallet()

	// Source wallet picker
	wi.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		wi.selectedWallet = selectedWallet
		wi.initializeAccountSelectors()
		wi.amount.setAssetType(wi.selectedWallet.GetAssetType())
		wi.sendDestination.initDestinationWalletSelector(selectedWallet)
	})
}

func (wi *sharedProperties) initializeAccountSelectors() {
	// Source account picker
	wi.sourceAccountSelector = components.NewWalletAndAccountSelector(wi.Load).
		Title(values.String(values.StrFrom)).
		AccountSelected(func(selectedAccount *sharedW.Account) {
			// this resets the selected destination account based on the
			// selected source account. This is done to prevent sending to
			// an account that is invalid either because the destination
			// account is the same as the source account or because the
			// destination account needs to change based on if the selected
			// wallet has privacy enabled.
			wi.sendDestination.destinationAccountSelector.SelectFirstValidAccount(
				wi.sendDestination.destinationWalletSelector.SelectedWallet())
			wi.validateAndConstructTx()
		}).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !wi.selectedWallet.IsWatchingOnlyWallet()

			if wi.selectedWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
				!wi.selectedWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) {
				// Spending unmixed fund isn't permitted for the selected wallet

				// only mixed accounts can send to address/wallets for wallet with privacy setup
				switch wi.sendDestination.accountSwitch.SelectedIndex() {
				case sendToAddress:
					accountIsValid = account.Number == wi.selectedWallet.MixedAccountNumber()
				case SendToWallet:
					destinationWalletID := wi.sendDestination.destinationWalletSelector.SelectedWallet().GetWalletID()
					if destinationWalletID != wi.selectedWallet.GetWalletID() {
						accountIsValid = account.Number == wi.selectedWallet.MixedAccountNumber()
					}
				}
			}
			return accountIsValid
		}).
		SetActionInfoText(values.String(values.StrTxConfModalInfoTxt))

	// if a source account exists, don't overwrite it.
	if wi.sourceAccountSelector.SelectedAccount() == nil {
		wi.sourceAccountSelector.SelectFirstValidAccount(wi.selectedWallet)
	}

	wi.sendDestination.destinationAccountSelector = wi.sendDestination.destinationAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
		accountIsValid := account.Number != load.MaxInt32
		// Filter mixed wallet
		destinationWallet := wi.sendDestination.destinationAccountSelector.SelectedWallet()
		isMixedAccount := destinationWallet.MixedAccountNumber() == account.Number
		// Filter the sending account.
		sourceWalletID := wi.sourceAccountSelector.SelectedAccount().WalletID
		isSameAccount := sourceWalletID == account.WalletID && account.Number == wi.sourceAccountSelector.SelectedAccount().Number
		if !accountIsValid || isSameAccount || isMixedAccount {
			return false
		}
		return true
	})

	wi.sendDestination.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		wi.validateAndConstructTx()
	})

	wi.sendDestination.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		wi.sendDestination.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		if wi.selectedWallet.Asset.GetAssetType() == libUtil.DCRWalletAsset {
			wi.sourceAccountSelector.SelectFirstValidAccount(wi.selectedWallet)
		}
	})
}

func (wi *sharedProperties) updateSelectedUTXOs(utxos []*sharedW.UnspentOutput) {
	wi.selectedUTXOs = selectedUTXOsInfo{
		selectedUTXOs: utxos,
		sourceAccount: wi.sourceAccountSelector.SelectedAccount(),
	}
	if len(utxos) > 0 {
		for _, elem := range utxos {
			wi.selectedUTXOs.totalUTXOsAmount += elem.Amount.ToInt()
		}
	}
}

func (wi *sharedProperties) fetchExchangeRate() {
	if wi.isFetchingExchangeRate {
		return
	}
	wi.isFetchingExchangeRate = true
	var market string
	switch wi.selectedWallet.Asset.GetAssetType() {
	case libUtil.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libUtil.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libUtil.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", wi.selectedWallet.Asset.GetAssetType())
		wi.isFetchingExchangeRate = false
		return
	}

	rate, err := wi.WL.AssetsManager.ExternalService.GetTicker(wi.currencyExchange, market)
	if err != nil {
		log.Error(err)
		wi.isFetchingExchangeRate = false
		return
	}

	wi.exchangeRate = rate.LastTradePrice
	wi.amount.setExchangeRate(wi.exchangeRate)
	wi.validateAndConstructTx() // convert estimates to usd

	wi.isFetchingExchangeRate = false
	wi.parentWindow.Reload()
}

func (wi *sharedProperties) validateAndConstructTx() {
	// delete all the previous errors set earlier.
	wi.amountValidationError("")
	wi.addressValidationError("")

	if wi.validate() {
		wi.constructTx()
	} else {
		wi.clearEstimates()
		wi.showBalaceAfterSend()
	}
}

func (wi *sharedProperties) validateAndConstructTxAmountOnly() {
	defer wi.RefreshTheme(wi.parentWindow)

	if !wi.sendDestination.validate() && wi.amount.amountIsValid() {
		wi.constructTx()
	} else {
		wi.validateAndConstructTx()
	}
}

func (wi *sharedProperties) validate() bool {
	amountIsValid := wi.amount.amountIsValid()
	addressIsValid := wi.sendDestination.validate()

	// No need for checking the err message since it is as result of amount and
	// address validation.
	// validForSending
	return amountIsValid && addressIsValid
}

func (wi *sharedProperties) constructTx() {
	destinationAddress, err := wi.sendDestination.destinationAddress()
	if err != nil {
		wi.addressValidationError(err.Error())
		return
	}
	destinationAccount := wi.sendDestination.destinationAccount()

	amountAtom, SendMax, err := wi.amount.validAmount()
	if err != nil {
		wi.amountValidationError(err.Error())
		return
	}

	sourceAccount := wi.sourceAccountSelector.SelectedAccount()
	selectedUTXOs := make([]*sharedW.UnspentOutput, 0)
	if sourceAccount == wi.selectedUTXOs.sourceAccount {
		selectedUTXOs = wi.selectedUTXOs.selectedUTXOs
	}

	err = wi.selectedWallet.NewUnsignedTx(sourceAccount.Number, selectedUTXOs)
	if err != nil {
		wi.amountValidationError(err.Error())
		return
	}

	err = wi.selectedWallet.AddSendDestination(destinationAddress, amountAtom, SendMax)
	if err != nil {
		if strings.Contains(err.Error(), "amount") {
			wi.amountValidationError(err.Error())
			return
		}
		wi.addressValidationError(err.Error())
		return
	}

	feeAndSize, err := wi.selectedWallet.EstimateFeeAndSize()
	if err != nil {
		wi.amountValidationError(err.Error())
		return
	}

	feeAtom := feeAndSize.Fee.UnitValue
	spendableAmount := sourceAccount.Balance.Spendable.ToInt()
	if len(selectedUTXOs) > 0 {
		spendableAmount = wi.selectedUTXOs.totalUTXOsAmount
	}

	if SendMax {
		amountAtom = spendableAmount - feeAtom
	}

	wal := wi.selectedWallet.Asset
	totalSendingAmount := wal.ToAmount(amountAtom + feeAtom)
	balanceAfterSend := wal.ToAmount(spendableAmount - totalSendingAmount.ToInt())

	// populate display data
	wi.txFee = wal.ToAmount(feeAtom).String()

	wi.feeRateSelector.EstSignedSize = fmt.Sprintf("%d Bytes", feeAndSize.EstimatedSignedSize)
	wi.feeRateSelector.TxFee = wi.txFee
	wi.feeRateSelector.SetFeerate(feeAndSize.FeeRate)
	wi.totalCost = totalSendingAmount.String()
	wi.balanceAfterSend = balanceAfterSend.String()
	wi.sendAmount = wal.ToAmount(amountAtom).String()
	wi.destinationAddress = destinationAddress
	wi.destinationAccount = destinationAccount
	wi.sourceAccount = sourceAccount

	if SendMax {
		// TODO: this workaround ignores the change events from the
		// amount input to avoid construct tx cycle.
		wi.amount.setAmount(amountAtom)
	}

	if wi.exchangeRate != -1 && wi.usdExchangeSet {
		wi.feeRateSelector.USDExchangeSet = true
		wi.txFeeUSD = fmt.Sprintf("$%.4f", utils.CryptoToUSD(wi.exchangeRate, feeAndSize.Fee.CoinValue))
		wi.feeRateSelector.TxFeeUSD = wi.txFeeUSD
		wi.totalCostUSD = utils.FormatUSDBalance(wi.Printer, utils.CryptoToUSD(wi.exchangeRate, totalSendingAmount.ToCoin()))
		wi.balanceAfterSendUSD = utils.FormatUSDBalance(wi.Printer, utils.CryptoToUSD(wi.exchangeRate, balanceAfterSend.ToCoin()))

		usdAmount := utils.CryptoToUSD(wi.exchangeRate, wal.ToAmount(amountAtom).ToCoin())
		wi.sendAmountUSD = utils.FormatUSDBalance(wi.Printer, usdAmount)
	}
}

func (wi *sharedProperties) showBalaceAfterSend() {
	if wi.sourceAccountSelector != nil {
		sourceAccount := wi.sourceAccountSelector.SelectedAccount()
		if sourceAccount.Balance == nil {
			return
		}
		balanceAfterSend := sourceAccount.Balance.Spendable
		wi.balanceAfterSend = balanceAfterSend.String()
		wi.balanceAfterSendUSD = utils.FormatUSDBalance(wi.Printer, utils.CryptoToUSD(wi.exchangeRate, balanceAfterSend.ToCoin()))
	}
}

func (wi *sharedProperties) amountValidationError(err string) {
	wi.amount.setError(err)
	wi.clearEstimates()
}

func (wi *sharedProperties) addressValidationError(err string) {
	wi.sendDestination.setError(err)
	wi.clearEstimates()
}

func (wi *sharedProperties) clearEstimates() {
	wi.txFee = " - " + string(wi.selectedWallet.GetAssetType())
	wi.feeRateSelector.TxFee = wi.txFee
	wi.txFeeUSD = " - "
	wi.feeRateSelector.TxFeeUSD = wi.txFeeUSD
	wi.totalCost = " - " + string(wi.selectedWallet.GetAssetType())
	wi.totalCostUSD = " - "
	wi.balanceAfterSend = " - " + string(wi.selectedWallet.GetAssetType())
	wi.balanceAfterSendUSD = " - "
	wi.sendAmount = " - "
	wi.sendAmountUSD = " - "
	wi.feeRateSelector.SetFeerate(0)
}

func (wi *sharedProperties) resetFields() {
	wi.sendDestination.clearAddressInput()
	wi.txLabelInputEditor.Editor.SetText("")

	wi.amount.resetFields()
}

func (wi *sharedProperties) isFeerateAPIApproved() bool {
	return wi.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libUtil.FeeRateHTTPAPI)
}

func (wi *sharedProperties) handleFunc() {
	if wi.feeRateSelector.FetchRates.Clicked() {
		go wi.feeRateSelector.FetchFeeRate(wi.parentWindow, wi.selectedWallet)
	}

	if wi.feeRateSelector.EditRates.Clicked() {
		wi.feeRateSelector.OnEditRateClicked(wi.selectedWallet)
	}

	wi.nextButton.SetEnabled(wi.validate())
	wi.sendDestination.handle()
	wi.amount.handle()

	if wi.infoButton.Button.Clicked() {
		textWithUnit := values.String(values.StrSend) + " " + string(wi.selectedWallet.GetAssetType())
		info := modal.NewCustomModal(wi.Load).
			Title(textWithUnit).
			Body(values.String(values.StrSendInfo)).
			SetPositiveButtonText(values.String(values.StrGotIt))
		wi.parentWindow.ShowModal(info)
	}

	if wi.retryExchange.Clicked() {
		go wi.fetchExchangeRate()
	}

	if wi.nextButton.Clicked() {
		if wi.selectedWallet.IsUnsignedTxExist() {
			wi.confirmTxModal = newSendConfirmModal(wi.Load, wi.authoredTxData, *wi.selectedWallet)
			wi.confirmTxModal.exchangeRateSet = wi.exchangeRate != -1 && wi.usdExchangeSet
			wi.confirmTxModal.txLabel = wi.txLabelInputEditor.Editor.Text()

			wi.confirmTxModal.txSent = func() {
				wi.resetFields()
				wi.clearEstimates()
			}

			wi.parentWindow.ShowModal(wi.confirmTxModal)
		}
	}

	// if destination switch is equal to Address
	if wi.sendDestination.sendToAddress {
		if wi.sendDestination.validate() {
			if !components.IsFetchExchangeRateAPIAllowed(wi.WL) {
				if len(wi.amount.amountEditor.Editor.Text()) == 0 {
					wi.amount.SendMax = false
				}
			} else {
				if len(wi.amount.amountEditor.Editor.Text()) == 0 {
					wi.amount.usdAmountEditor.Editor.SetText("")
					wi.amount.SendMax = false
				}
			}
		}
	} else {
		if !components.IsFetchExchangeRateAPIAllowed(wi.WL) {
			if len(wi.amount.amountEditor.Editor.Text()) == 0 {
				wi.amount.SendMax = false
			}
		} else {
			if len(wi.amount.amountEditor.Editor.Text()) == 0 {
				wi.amount.usdAmountEditor.Editor.SetText("")
				wi.amount.SendMax = false
			}
		}
	}

	if len(wi.amount.amountEditor.Editor.Text()) > 0 && wi.sourceAccountSelector.Changed() {
		wi.amount.validateAmount()
		wi.validateAndConstructTxAmountOnly()
	}

	if wi.amount.IsMaxClicked() {
		wi.amount.setError("")
		wi.amount.SendMax = true
		wi.amount.amountChanged()
	}
}
