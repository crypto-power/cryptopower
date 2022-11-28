package send

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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
	SendPageID = "Send"
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
	editRates     cryptomaterial.Button
	fetchRates    cryptomaterial.Button

	ratesEditor cryptomaterial.Editor
	// editOrDisplay holds an editor or label component depending on the state of
	// editRates(Save -> holds Editor or Edit -> holds a Label) button.
	editOrDisplay interface{}

	shadowBox *cryptomaterial.Shadow
	backdrop  *widget.Clickable

	isFetchingExchangeRate bool

	exchangeRate        float64
	usdExchangeSet      bool
	exchangeRateMessage string
	confirmTxModal      *sendConfirmModal
	currencyExchange    string

	*authoredTxData
	selectedWallet *load.WalletMapping
}

type authoredTxData struct {
	destinationAddress  string
	destinationAccount  *sharedW.Account
	sourceAccount       *sharedW.Account
	txFee               string
	txFeeUSD            string
	priority            string
	estSignedSize       string
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
	}
	pg.selectedWallet = &load.WalletMapping{
		Asset: l.WL.SelectedWallet.Wallet,
	}

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

	pg.sendDestination.destinationAccountSelector =
		pg.sendDestination.destinationAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !pg.selectedWallet.IsWatchingOnlyWallet()
			// Filter the sending account.
			sourceWalletId := pg.sourceAccountSelector.SelectedAccount().WalletID
			isSameAccount := sourceWalletId == account.WalletID && account.Number == pg.sourceAccountSelector.SelectedAccount().Number
			if !accountIsValid || isSameAccount {
				return false
			}
			return true
		})

	pg.sendDestination.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		pg.validateAndConstructTx()
	})

	pg.sendDestination.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
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
	pg.sourceAccountSelector.ListenForTxNotifications(pg.ctx, pg.ParentWindow())
	pg.sendDestination.destinationAccountSelector.SelectFirstValidAccount(pg.sendDestination.destinationWalletSelector.SelectedWallet())
	pg.sourceAccountSelector.SelectFirstValidAccount(pg.selectedWallet)
	pg.sendDestination.destinationAddressEditor.Editor.Focus()

	pg.usdExchangeSet = false
	pg.currencyExchange = pg.WL.MultiWallet.GetCurrencyConversionExchange()
	if pg.currencyExchange != values.DefaultExchangeValue {
		pg.usdExchangeSet = true
		go pg.fetchExchangeRate()
	}

	if pg.selectedWallet.GetAssetType() == libUtil.BTCWalletAsset {
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
	rate, err := pg.WL.MultiWallet.ExternalService.GetTicker(pg.currencyExchange, market)
	if err != nil {
		log.Printf("Error fetching exchange rate : %s \n", err)
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

	validForSending := amountIsValid && addressIsValid

	return validForSending
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
	pg.editOrDisplay = pg.addRatesUnits(feeAndSize.FeeRate)
	pg.estSignedSize = fmt.Sprintf("%d Bytes", feeAndSize.EstimatedSignedSize)
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
		pg.txFeeUSD = fmt.Sprintf("$%.4f", utils.CryptoToUSD(pg.exchangeRate, feeAndSize.Fee.CoinValue))
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
	pg.txFeeUSD = " - "
	pg.estSignedSize = " - "
	pg.totalCost = " - " + string(pg.selectedWallet.GetAssetType())
	pg.totalCostUSD = " - "
	pg.balanceAfterSend = " - " + string(pg.selectedWallet.GetAssetType())
	pg.balanceAfterSendUSD = " - "
	pg.sendAmount = " - "
	pg.sendAmountUSD = " - "
}

func (pg *Page) resetFields() {
	pg.sendDestination.clearAddressInput()

	pg.amount.resetFields()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions() {
	pg.feeRateAPIHandler()
	pg.editsOrDisplayRatesHandler()
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

	if pg.nextButton.Clicked() {
		if pg.selectedWallet.IsUnsignedTxExist() {
			pg.confirmTxModal = newSendConfirmModal(pg.Load, pg.authoredTxData, *pg.selectedWallet)
			pg.confirmTxModal.exchangeRateSet = pg.exchangeRate != -1 && pg.usdExchangeSet

			pg.confirmTxModal.txSent = func() {
				pg.resetFields()
				pg.clearEstimates()
			}

			pg.ParentWindow().ShowModal(pg.confirmTxModal)
		}
	}

	modalShown := pg.confirmTxModal != nil && pg.confirmTxModal.IsShown()
	currencyValue := pg.WL.MultiWallet.GetCurrencyConversionExchange()

	if !modalShown {
		isFeeRateEditFocused := pg.ratesEditor.Editor.Focused()
		isSendToWallet := pg.sendDestination.accountSwitch.SelectedIndex() == 2
		isInSaveMode := pg.ratesEditor.Editor.Text() == values.String(values.StrSave)
		isDestinationEditorFocused := pg.sendDestination.destinationAddressEditor.Editor.Focused()

		switch {
		// If the hidden fee rate editor is in focus.
		case isFeeRateEditFocused && isInSaveMode:
			pg.ratesEditor.Editor.Focus()

		// If destination address is invalid and destination editor is in focus.
		case !pg.sendDestination.validate() && isDestinationEditorFocused:
			pg.sendDestination.destinationAddressEditor.Editor.Focus()

		// If destination address is valid and destination editor is in focus.
		case pg.sendDestination.validate() && isDestinationEditorFocused:
			pg.amount.amountEditor.Editor.Focus()

		// If accounts switch selects the wallet option.
		case isSendToWallet && !isFeeRateEditFocused && !isDestinationEditorFocused:
			pg.amount.amountEditor.Editor.Focus()
		}
	}

	// if destination switch is equal to Address
	if pg.sendDestination.sendToAddress {
		if pg.sendDestination.validate() {
			if currencyValue == values.DefaultExchangeValue {
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
		if currencyValue == values.DefaultExchangeValue {
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

func (pg *Page) addRatesUnits(rates int64) string {
	return pg.Load.Printer.Sprintf("%d Sat/kvB", rates)
}

func (pg *Page) editsOrDisplayRatesHandler() {
	if pg.editRates.Clicked() {
		// reset fields
		pg.feeEstimationError("")
		// Enable after saving is complete successfully
		pg.fetchRates.SetEnabled(false)

		if pg.editRates.Text == values.String(values.StrSave) {
			text := pg.ratesEditor.Editor.Text()
			ratesInt, err := pg.selectedWallet.SetAPIFeeRate(text)
			if err != nil {
				pg.feeEstimationError(err.Error())
				text = " - "
			} else {
				text = pg.addRatesUnits(ratesInt)
				pg.amount.amountChanged()
			}

			pg.editOrDisplay = text
			pg.ratesEditor.Editor.SetText("")
			pg.editRates.Text = values.String(values.StrEdit)
			pg.fetchRates.SetEnabled(true)
			return
		}

		pg.editOrDisplay = pg.ratesEditor
		pg.editRates.Text = values.String(values.StrSave)
		pg.priority = "Unknown" // Only known when fee rate is set from the API.
	}
}

func (pg *Page) feeRateAPIHandler() {
	if pg.fetchRates.Clicked() {
		// reset fields
		pg.feeEstimationError("")
		// Enable after the fee rate selection is complete successfully.
		pg.editRates.SetEnabled(false)

		feeRates, err := pg.selectedWallet.GetAPIFeeRate()
		if err != nil {
			pg.feeEstimationError(err.Error())
			return
		}

		blocksStr := func(b int32) string {
			val := strconv.Itoa(int(b)) + " block"
			if b == 1 {
				return val
			}
			return val + "s"
		}

		radiogroupbtns := new(widget.Enum)
		items := make([]layout.FlexChild, 0)
		for index, feerate := range feeRates {
			key := strconv.Itoa(index)
			value := pg.addRatesUnits(feerate.Feerate.ToInt()) + " - " + blocksStr(feerate.ConfirmedBlocks)
			radioBtn := pg.Load.Theme.RadioButton(radiogroupbtns, key, value,
				pg.Load.Theme.Color.DeepBlue, pg.Load.Theme.Color.Primary)
			items = append(items, layout.Rigid(radioBtn.Layout))
		}

		info := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrFeeRates)).
			UseCustomWidget(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
			}).
			SetCancelable(true).
			SetNegativeButtonText(values.String(values.StrCancel)).
			SetPositiveButtonText(values.String(values.StrSave)).
			SetPositiveButtonCallback(func(isChecked bool, im *modal.InfoModal) bool {
				fields := strings.Fields(radiogroupbtns.Value)
				index, _ := strconv.Atoi(fields[0])
				rate := strconv.Itoa(int(feeRates[index].Feerate.ToInt()))
				rateInt, err := pg.selectedWallet.SetAPIFeeRate(rate)
				if err != nil {
					pg.feeEstimationError(err.Error())
					return false
				}

				pg.editOrDisplay = pg.addRatesUnits(rateInt)
				blocks := feeRates[index].ConfirmedBlocks
				timeBefore := time.Now().Add(time.Duration(-10*blocks) * time.Minute)
				pg.priority = fmt.Sprintf("%v (~%v)", blocksStr(blocks), components.TimeAgo(timeBefore.Unix()))
				im.Dismiss()
				return true
			})

		pg.ParentWindow().ShowModal((info))
		// fee rate selection is complete successfully.
		pg.editRates.SetEnabled(true)
		pg.amount.amountChanged()
	}
}
