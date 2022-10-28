package send

import (
	"context"
	"fmt"
	"log"

	"gioui.org/io/key"
	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/app"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/utils"
	"gitlab.com/raedah/cryptopower/ui/values"
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

	txFeeCollapsible *cryptomaterial.Collapsible
	shadowBox        *cryptomaterial.Shadow
	backdrop         *widget.Clickable

	isFetchingExchangeRate bool

	exchangeRate        float64
	usdExchangeSet      bool
	exchangeRateMessage string
	confirmTxModal      *sendConfirmModal
	coinSelectionLabel  *cryptomaterial.Clickable
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
	rate, err := pg.WL.MultiWallet.ExternalService.GetTicker(pg.currencyExchange, values.DCRUSDTMarket)
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
		pg.constructTx(false)
	} else {
		pg.clearEstimates()
		pg.showBalaceAfterSend()
	}
}

func (pg *Page) validateAndConstructTxAmountOnly() {
	defer pg.RefreshTheme(pg.ParentWindow())

	if !pg.sendDestination.validate() && pg.amount.amountIsValid() {
		pg.constructTx(true)
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

func (pg *Page) constructTx(useDefaultParams bool) {
	destinationAddress, err := pg.sendDestination.destinationAddress(useDefaultParams)
	if err != nil {
		pg.feeEstimationError(err.Error())
		return
	}
	destinationAccount := pg.sendDestination.destinationAccount(useDefaultParams)

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
	pg.estSignedSize = fmt.Sprintf("%d bytes", feeAndSize.EstimatedSignedSize)
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
		pg.txFeeUSD = fmt.Sprintf("$%.4f", utils.DCRToUSD(pg.exchangeRate, feeAndSize.Fee.CoinValue))
		pg.totalCostUSD = utils.FormatUSDBalance(pg.Printer, utils.DCRToUSD(pg.exchangeRate, totalSendingAmount.ToCoin()))
		pg.balanceAfterSendUSD = utils.FormatUSDBalance(pg.Printer, utils.DCRToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))

		usdAmount := utils.DCRToUSD(pg.exchangeRate, wal.ToAmount(amountAtom).ToCoin())
		pg.sendAmountUSD = utils.FormatUSDBalance(pg.Printer, usdAmount)
	}
}

func (pg *Page) showBalaceAfterSend() {
	if pg.sourceAccountSelector != nil {
		sourceAccount := pg.sourceAccountSelector.SelectedAccount()
		balanceAfterSend := sourceAccount.Balance.Spendable
		pg.balanceAfterSend = balanceAfterSend.String()
		pg.balanceAfterSendUSD = utils.FormatUSDBalance(pg.Printer, utils.DCRToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))
	}
}

func (pg *Page) feeEstimationError(err string) {
	pg.amount.setError(err)
	pg.clearEstimates()
}

func (pg *Page) clearEstimates() {

	// pg.txAuthor = nil
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
	pg.nextButton.SetEnabled(pg.validate())
	pg.sendDestination.handle()
	pg.amount.handle()

	if pg.infoButton.Button.Clicked() {
		textWithUnit := values.String(values.StrSend) + " " + string(pg.WL.SelectedWallet.Wallet.GetAssetType())
		info := modal.NewCustomModal(pg.Load).
			Title(textWithUnit).
			Body(values.String(values.StrSendInfo)).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(info)
	}

	for pg.retryExchange.Clicked() {
		go pg.fetchExchangeRate()
	}

	for pg.nextButton.Clicked() {
		if pg.selectedWallet.IsUnsignedTxExist() {
			pg.confirmTxModal = newSendConfirmModal(pg.Load, pg.authoredTxData, pg.WL.SelectedWallet.Wallet)
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
	if currencyValue == values.DefaultExchangeValue {
		switch {
		case !pg.sendDestination.sendToAddress:
			if !pg.amount.dcrAmountEditor.Editor.Focused() && !modalShown {
				pg.amount.dcrAmountEditor.Editor.Focus()
			}
		default:
			if pg.sendDestination.accountSwitch.Changed() {
				if !pg.sendDestination.validate() {
					pg.sendDestination.destinationAddressEditor.Editor.Focus()
				} else {
					pg.amount.dcrAmountEditor.Editor.Focus()
				}

			}
		}
	} else {
		switch {
		case !pg.sendDestination.sendToAddress && !(pg.amount.dcrAmountEditor.Editor.Focused() || pg.amount.usdAmountEditor.Editor.Focused()):
			if !modalShown {
				pg.amount.dcrAmountEditor.Editor.Focus()
			}
		case !pg.sendDestination.sendToAddress && (pg.amount.dcrAmountEditor.Editor.Focused() || pg.amount.usdAmountEditor.Editor.Focused()):
		default:
			if pg.sendDestination.accountSwitch.Changed() {
				if !pg.sendDestination.validate() {
					pg.sendDestination.destinationAddressEditor.Editor.Focus()
				} else {
					pg.amount.dcrAmountEditor.Editor.Focus()
				}
			}
		}
	}

	// if destination switch is equal to Address
	if pg.sendDestination.sendToAddress {
		if pg.sendDestination.validate() {
			if currencyValue == values.DefaultExchangeValue {
				if len(pg.amount.dcrAmountEditor.Editor.Text()) == 0 {
					pg.amount.SendMax = false
				}
			} else {
				if len(pg.amount.dcrAmountEditor.Editor.Text()) == 0 {
					pg.amount.usdAmountEditor.Editor.SetText("")
					pg.amount.SendMax = false
				}
			}
		}
	} else {
		if currencyValue == values.DefaultExchangeValue {
			if len(pg.amount.dcrAmountEditor.Editor.Text()) == 0 {
				pg.amount.SendMax = false
			}
		} else {
			if len(pg.amount.dcrAmountEditor.Editor.Text()) == 0 {
				pg.amount.usdAmountEditor.Editor.SetText("")
				pg.amount.SendMax = false
			}
		}
	}

	if len(pg.amount.dcrAmountEditor.Editor.Text()) > 0 && pg.sourceAccountSelector.Changed() {
		pg.amount.validateDCRAmount()
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
func (pg *Page) HandleKeyPress(evt *key.Event) {
	if pg.confirmTxModal != nil && pg.confirmTxModal.IsShown() {
		return
	}

	currencyValue := pg.WL.MultiWallet.GetCurrencyConversionExchange()
	if currencyValue == values.DefaultExchangeValue {
		switch {
		case !pg.sendDestination.sendToAddress:
			cryptomaterial.SwitchEditors(evt, pg.amount.dcrAmountEditor.Editor)
		default:
			cryptomaterial.SwitchEditors(evt, pg.sendDestination.destinationAddressEditor.Editor, pg.amount.dcrAmountEditor.Editor)
		}
	} else {
		switch {
		case !pg.sendDestination.sendToAddress && !(pg.amount.dcrAmountEditor.Editor.Focused() || pg.amount.usdAmountEditor.Editor.Focused()):
		case !pg.sendDestination.sendToAddress && (pg.amount.dcrAmountEditor.Editor.Focused() || pg.amount.usdAmountEditor.Editor.Focused()):
			cryptomaterial.SwitchEditors(evt, pg.amount.usdAmountEditor.Editor, pg.amount.dcrAmountEditor.Editor)
		default:
			cryptomaterial.SwitchEditors(evt, pg.sendDestination.destinationAddressEditor.Editor, pg.amount.dcrAmountEditor.Editor, pg.amount.usdAmountEditor.Editor)
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
func (pg *Page) OnNavigatedFrom() {
	pg.ctxCancel() // causes crash if nil, when the main page is closed if send page is created but never displayed (because sync in progress)
}
