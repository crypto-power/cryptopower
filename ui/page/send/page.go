package send

import (
	"fmt"
	"strings"

	"gioui.org/io/key"
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
	// modalLayout is initialized if this page will be displayed as a modal
	// rather than a full page. A modal display is used and a wallet selector is
	// displayed if this send page is opened from the home page.
	modalLayout *cryptomaterial.Modal

	pageContainer *widget.List

	sourceWalletSelector  *components.WalletAndAccountSelector
	sourceAccountSelector *components.WalletAndAccountSelector

	// recipient  *recipient
	recipients []*recipient

	infoButton cryptomaterial.IconButton
	// retryExchange cryptomaterial.Button // TODO not included in design
	nextButton     cryptomaterial.Button
	addRecipentBtn *cryptomaterial.Clickable

	isFetchingExchangeRate bool

	exchangeRate   float64
	usdExchangeSet bool
	confirmTxModal *sendConfirmModal

	*authoredTxData
	selectedWallet  sharedW.Asset
	feeRateSelector *components.FeeRateSelector

	toCoinSelection *cryptomaterial.Clickable
	advanceOptions  *cryptomaterial.Collapsible

	selectedUTXOs      selectedUTXOsInfo
	navigateToSyncBtn  cryptomaterial.Button
	currentIDRecipient int
}

type getPageFields func() pageFields

type pageFields struct {
	exchangeRate           float64
	usdExchangeSet         bool
	isFetchingExchangeRate bool
}

type authoredTxData struct {
	destinationAddress  []string
	destinationAccount  []*sharedW.Account
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

func NewSendPage(l *load.Load, wallet sharedW.Asset) *Page {
	pg := &Page{
		Load: l,

		authoredTxData:    &authoredTxData{},
		exchangeRate:      -1,
		navigateToSyncBtn: l.Theme.Button(values.String(values.StrStartSync)),
		addRecipentBtn:    l.Theme.NewClickable(false),
		recipients:        make([]*recipient, 0),
	}

	if wallet == nil {
		// When this page is opened from the home page, the wallet to use is not
		// specified. This page will be opened as a modal and a wallet selector
		// will be displayed.
		pg.modalLayout = l.Theme.ModalFloatTitle(values.String(values.StrSend), pg.IsMobileView())
		pg.GenericPageModal = pg.modalLayout.GenericPageModal
		pg.initModalWalletSelector() // will auto select the first wallet in the dropdown as pg.selectedWallet
	} else {
		pg.GenericPageModal = app.NewGenericPageModal(SendPageID)
		pg.selectedWallet = wallet
	}

	callbackFunc := func() libUtil.AssetType {
		return pg.selectedWallet.GetAssetType()
	}
	pg.feeRateSelector = components.NewFeeRateSelector(l, callbackFunc).ShowSizeAndCost()
	pg.addRecipient()

	pg.initializeAccountSelectors()
	pg.initLayoutWidgets()

	return pg
}

func (pg *Page) addRecipient() {
	rc := newRecipient(pg.Load, pg.selectedWallet, pg.pageFields, pg.currentIDRecipient)
	rc.onAddressChanged(func() {
		pg.validateAndConstructTx()
	})

	rc.onAmountChanged(func() {
		pg.validateAndConstructTx()
	})

	rc.onDeleteRecipient(func(id int) {
		pg.removeRecipient(id)
	})

	if pg.sourceAccountSelector != nil {
		rc.initializeAccountSelectors(pg.sourceAccountSelector.SelectedAccount())
	}
	rc.amount.setExchangeRate(pg.exchangeRate)
	pg.recipients = append(pg.recipients, rc)
	pg.currentIDRecipient++
}

func (pg *Page) removeRecipient(id int) {
	for i, re := range pg.recipients {
		if re.id == id {
			pg.recipients = append(pg.recipients[:i], pg.recipients[i+1:]...)
			break
		}
	}

	pg.selectedWallet.RemoveSendDestination(id)
}

func (pg *Page) pageFields() pageFields {
	return pageFields{
		exchangeRate:           pg.exchangeRate,
		usdExchangeSet:         pg.usdExchangeSet,
		isFetchingExchangeRate: pg.isFetchingExchangeRate,
	}
}

// initWalletSelector is used for the send modal for wallet selection.
func (pg *Page) initModalWalletSelector() {
	// initialize wallet selector
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrSelectWallet))
	pg.selectedWallet = pg.sourceWalletSelector.SelectedWallet()

	// Source wallet picker
	pg.sourceWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
		pg.selectedWallet = selectedWallet
		go load.GetAPIFeeRate(pg.selectedWallet)
		go pg.feeRateSelector.UpdatedFeeRate(pg.selectedWallet)
		pg.setAssetTypeForRecipients()
		pg.initializeAccountSelectors()
	})
}

func (pg *Page) initializeAccountSelectors() {
	// Source account picker
	pg.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrFrom)).
		AccountSelected(func(selectedAccount *sharedW.Account) {
			// this resets the selected destination account based on the
			// selected source account. This is done to prevent sending to
			// an account that is invalid either because the destination
			// account is the same as the source account or because the
			// destination account needs to change based on if the selected
			// wallet has privacy enabled.
			pg.initAccountsSelectorForRecipients(selectedAccount)
		}).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !pg.selectedWallet.IsWatchingOnlyWallet()

			if pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
				!pg.selectedWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) {
				// Spending unmixed fund isn't permitted for the selected wallet

				// only mixed accounts can send to address/wallets for wallet with privacy setup
				// don't need to check account the same with destination account
				accountIsValid = account.Number == load.MixedAccountNumber(pg.selectedWallet)
			}
			return accountIsValid
		}).
		SetActionInfoText(values.String(values.StrTxConfModalInfoTxt))
	// if a source account exists, don't overwrite it.
	if pg.sourceAccountSelector.SelectedAccount() == nil {
		pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)
	}
}

// RestyleWidgets restyles select widgets to match the current theme. This is
// especially necessary when the dark mode setting is changed.
func (pg *Page) RestyleWidgets() {
	// pg.recipient.restyleWidgets()
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

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	pg.RestyleWidgets()

	if !pg.selectedWallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.sourceAccountSelector.ListenForTxNotifications(pg.ParentWindow()) // listener is stopped in OnNavigatedFrom()

	pg.usdExchangeSet = false
	if pg.AssetsManager.ExchangeRateFetchingEnabled() {
		pg.usdExchangeSet = pg.AssetsManager.RateSource.Ready()
		go pg.fetchExchangeRate()
	} else {
		// If exchange rate is not supported, validate and construct the TX.
		pg.validateAndConstructTx()
	}

	if pg.selectedWallet.GetAssetType() == libUtil.BTCWalletAsset && pg.isFeerateAPIApproved() {
		// This API call may take sometime to return. Call this before and cache
		// results.
		go load.GetAPIFeeRate(pg.selectedWallet)
		go pg.feeRateSelector.UpdatedFeeRate(pg.selectedWallet)
	}
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed
// to enable restyling UI elements where necessary.
// Satisfies the load.DarkModeChangeHandler interface.
func (pg *Page) OnDarkModeChanged(_ bool) {
	pg.RestyleWidgets()
}

func (pg *Page) fetchExchangeRate() {
	if pg.isFetchingExchangeRate {
		return
	}
	pg.isFetchingExchangeRate = true
	var market string
	switch pg.selectedWallet.GetAssetType() {
	case libUtil.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libUtil.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libUtil.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", pg.selectedWallet.GetAssetType())
		pg.isFetchingExchangeRate = false
		return
	}

	rate := pg.AssetsManager.RateSource.GetTicker(market, false) // okay to fetch latest rate, this is a goroutine
	if rate == nil || rate.LastTradePrice <= 0 {
		pg.isFetchingExchangeRate = false
		return
	}

	pg.exchangeRate = rate.LastTradePrice
	pg.updateRecipientExchangeRate()
	pg.validateAndConstructTx() // convert estimates to usd

	pg.isFetchingExchangeRate = false
	pg.ParentWindow().Reload()
}

func (pg *Page) validateAndConstructTx() {
	// delete all the previous errors set earlier.
	pg.cleanAllRecipientErrors()

	if pg.isAllRecipientValidated() {
		pg.constructTx()
	} else {
		pg.clearEstimates()
		pg.showBalanceAfterSend()
	}
}

func (pg *Page) constructTx() {
	sourceAccount := pg.sourceAccountSelector.SelectedAccount()
	selectedUTXOs := make([]*sharedW.UnspentOutput, 0)
	if sourceAccount == pg.selectedUTXOs.sourceAccount {
		selectedUTXOs = pg.selectedUTXOs.selectedUTXOs
	}

	err := pg.selectedWallet.NewUnsignedTx(sourceAccount.Number, selectedUTXOs)
	if err != nil {
		pg.setRecipientsAmountErr(err)
		pg.clearEstimates()
		return
	}

	totalCost, balanceAfterSend, totalAmount, err := pg.addSendDestination()
	if err != nil {
		return
	}

	feeAndSize, err := pg.selectedWallet.EstimateFeeAndSize()
	if err != nil {
		pg.setRecipientsAmountErr(err)
		pg.clearEstimates()
		return
	}

	feeAtom := feeAndSize.Fee.UnitValue
	wal := pg.selectedWallet

	// populate display data
	pg.txFee = wal.ToAmount(feeAtom).String()

	pg.feeRateSelector.EstSignedSize = fmt.Sprintf("%d Bytes", feeAndSize.EstimatedSignedSize)
	pg.feeRateSelector.TxFee = pg.txFee
	pg.feeRateSelector.SetFeerate(feeAndSize.FeeRate)
	pg.totalCost = totalCost.String()
	pg.balanceAfterSend = balanceAfterSend.String()
	pg.sendAmount = wal.ToAmount(totalAmount).String()
	pg.destinationAddress = pg.getDestinationAddresses()
	pg.destinationAccount = pg.getDestinationAccounts()
	pg.sourceAccount = sourceAccount

	// if SendMax {
	// TODO: this workaround ignores the change events from the
	// amount input to avoid construct tx cycle.
	// 	pg.recipient.setAmount(amountAtom)
	// }

	if pg.exchangeRate != -1 && pg.usdExchangeSet {
		pg.feeRateSelector.USDExchangeSet = true
		pg.txFeeUSD = fmt.Sprintf("$%.4f", utils.CryptoToUSD(pg.exchangeRate, feeAndSize.Fee.CoinValue))
		pg.feeRateSelector.TxFeeUSD = pg.txFeeUSD
		pg.totalCostUSD = utils.FormatAsUSDString(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, totalCost.ToCoin() /*totalSendingAmount.ToCoin()*/))
		pg.balanceAfterSendUSD = utils.FormatAsUSDString(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))

		usdAmount := utils.CryptoToUSD(pg.exchangeRate, wal.ToAmount( /*amountAtom*/ totalAmount).ToCoin())
		pg.sendAmountUSD = utils.FormatAsUSDString(pg.Printer, usdAmount)
	}
}

func (pg *Page) addSendDestination() (sharedW.AssetAmount, sharedW.AssetAmount, int64, error) {
	var totalCost int64 = 0

	sourceAccount := pg.sourceAccountSelector.SelectedAccount()
	selectedUTXOs := make([]*sharedW.UnspentOutput, 0)
	if sourceAccount == pg.selectedUTXOs.sourceAccount {
		selectedUTXOs = pg.selectedUTXOs.selectedUTXOs
	}

	feeAndSize, err := pg.selectedWallet.EstimateFeeAndSize()
	if err != nil {
		pg.setRecipientsAmountErr(err)
		return nil, nil, 0, err
	}
	feeAtom := feeAndSize.Fee.UnitValue
	spendableAmount := sourceAccount.Balance.Spendable.ToInt()
	if len(selectedUTXOs) > 0 {
		spendableAmount = pg.selectedUTXOs.totalUTXOsAmount
	}

	wal := pg.selectedWallet
	var totalSendAmount int64 = 0
	for _, recipient := range pg.recipients {
		destinationAddress := recipient.destinationAddress()
		amountAtom, SendMax := recipient.validAmount()
		err := pg.selectedWallet.AddSendDestination(recipient.id, destinationAddress, amountAtom, SendMax)
		if err != nil {
			if strings.Contains(err.Error(), "amount") {
				recipient.amountValidationError(err.Error())
			} else {
				recipient.addressValidationError(err.Error())
			}

			pg.clearEstimates()
		}

		if SendMax {
			amountAtom = spendableAmount - feeAtom
			recipient.setAmount(amountAtom)
		}
		totalSendAmount += amountAtom
		cost := amountAtom + feeAtom
		totalCost += cost
	}
	balanceAfterSend := wal.ToAmount(spendableAmount - totalCost)
	return wal.ToAmount(totalCost), balanceAfterSend, totalSendAmount, nil

}

func (pg *Page) isAllRecipientValidated() bool {
	isValid := true
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		isValid = isValid && recipient.isValidated()
	}
	return isValid
}

func (pg *Page) cleanAllRecipientErrors() {
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.cleanAllErrors()
	}
}

func (pg *Page) updateRecipientExchangeRate() {
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.amount.setExchangeRate(pg.exchangeRate)
	}
}

func (pg *Page) setAssetTypeForRecipients() {
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.setDestinationAssetType(pg.selectedWallet.GetAssetType())
	}
}

func (pg *Page) initAccountsSelectorForRecipients(account *sharedW.Account) {
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.initializeAccountSelectors(account)
	}
}

func (pg *Page) setRecipientsAmountErr(err error) {
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.amountValidationError(err.Error())
	}
	pg.clearEstimates()
}

func (pg *Page) allRecipientsIsValid() bool {
	isValid := true
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		isValid = isValid && recipient.isValidated()
	}
	return isValid
}

func (pg *Page) validateAllRecipientsAmount() bool {
	isValid := true
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.validateAmount()
	}
	return isValid
}

func (pg *Page) resetRecipientsFields() {
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		recipient.resetFields()
	}
}

func (pg *Page) getDestinationAccounts() []*sharedW.Account {
	accounts := make([]*sharedW.Account, 0)
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		destinationAccount := recipient.destinationAccount()
		if destinationAccount != nil && !recipient.isSendToAddress() {
			accounts = append(accounts, destinationAccount)
		}
	}
	return accounts
}

func (pg *Page) getDestinationAddresses() []string {
	addresses := make([]string, 0)
	for i := range pg.recipients {
		recipient := pg.recipients[i]
		destinationAddress := recipient.destinationAddress()
		if destinationAddress != "" && recipient.isSendToAddress() {
			addresses = append(addresses, destinationAddress)
		}
	}
	return addresses
}

func (pg *Page) showBalanceAfterSend() {
	if pg.sourceAccountSelector != nil {
		sourceAccount := pg.sourceAccountSelector.SelectedAccount()
		if sourceAccount.Balance == nil {
			return
		}
		balanceAfterSend := sourceAccount.Balance.Spendable
		pg.balanceAfterSend = balanceAfterSend.String()
		pg.balanceAfterSendUSD = utils.FormatAsUSDString(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))
	}
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

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions() {
	if pg.feeRateSelector.SaveRate.Clicked() {
		pg.feeRateSelector.OnEditRateClicked(pg.selectedWallet)
	}

	pg.nextButton.SetEnabled(pg.allRecipientsIsValid())

	if pg.infoButton.Button.Clicked() {
		textWithUnit := values.String(values.StrSend) + " " + string(pg.selectedWallet.GetAssetType())
		info := modal.NewCustomModal(pg.Load).
			Title(textWithUnit).
			Body(values.String(values.StrSendInfo)).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(info)
	}

	//TODO not included in design
	// if pg.retryExchange.Clicked() {
	// 	go pg.fetchExchangeRate()
	// }

	if pg.toCoinSelection.Clicked() {
		if len(pg.getDestinationAddresses()) == len(pg.recipients) {
			if pg.modalLayout != nil {
				pg.ParentWindow().ShowModal(NewManualCoinSelectionPage(pg.Load, pg))
			} else {
				pg.ParentNavigator().Display(NewManualCoinSelectionPage(pg.Load, pg))
			}
		}
	}

	if pg.nextButton.Clicked() {
		if pg.selectedWallet.IsUnsignedTxExist() {
			pg.confirmTxModal = newSendConfirmModal(pg.Load, pg.authoredTxData, pg.selectedWallet)
			pg.confirmTxModal.exchangeRateSet = pg.exchangeRate != -1 && pg.usdExchangeSet
			// TODO handle if have many descryption text
			// this workaround to show description text when have one recipient and don't show when have more
			descriptionText := ""
			if len(pg.recipients) == 1 {
				descriptionText = pg.recipients[0].descriptionText()
			}
			pg.confirmTxModal.txLabel = descriptionText
			pg.confirmTxModal.txSent = func() {
				pg.resetRecipientsFields()
				pg.clearEstimates()
				if pg.modalLayout != nil {
					pg.modalLayout.Dismiss()
				}
			}

			pg.ParentWindow().ShowModal(pg.confirmTxModal)
		}
	}

	if pg.sourceAccountSelector.Changed() {
		pg.validateAllRecipientsAmount()
		pg.validateAndConstructTx()
	}

	if pg.navigateToSyncBtn.Button.Clicked() {
		pg.ToggleSync(pg.selectedWallet, func(b bool) {
			pg.selectedWallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
		})
	}

	if pg.addRecipentBtn.Clicked() {
		pg.addRecipient()
	}
}

// Handle is like HandleUserInteractions but Handle is called if this page is
// displayed as a modal while HandleUserInteractions is called if this page
// is displayed as a full page. Either Handle or HandleUserInteractions will
// be called just before Layout() is called to determine if any user interaction
// recently occurred on the modal or page and may be used to update any affected
// UI components shortly before they are displayed by the Layout() method.
func (pg *Page) Handle() {
	if pg.modalLayout.BackdropClicked(true) {
		pg.modalLayout.Dismiss()
	} else {
		pg.HandleUserInteractions()
	}
}

// OnResume is called to initialize data and get UI elements ready to be
// displayed. This is called just before Handle() and Layout() are called (in
// that order).

// OnResume is like OnNavigatedTo but OnResume is called if this page is
// displayed as a modal while OnNavigatedTo is called if this page is displayed
// as a full page. Either OnResume or OnNavigatedTo is called to initialize
// data and get UI elements ready to be displayed. This is called just before
// Handle() and Layout() are called (in that order).
func (pg *Page) OnResume() {
	pg.OnNavigatedTo()
}

// OnDismiss is like OnNavigatedFrom but OnDismiss is called if this page is
// displayed as a modal while OnNavigatedFrom is called if this page is
// displayed as a full page. Either OnDismiss or OnNavigatedFrom is called
// after the modal is dismissed.
// NOTE: The modal may be re-displayed on the app's window, in which case
// OnResume() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnResume() method.
func (pg *Page) OnDismiss() {
	pg.OnNavigatedFrom()
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

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {
	pg.sourceAccountSelector.StopTxNtfnListener()
}

func (pg *Page) isFeerateAPIApproved() bool {
	return pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libUtil.FeeRateHTTPAPI)
}
