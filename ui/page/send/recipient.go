package send

import (
	"fmt"
	// "image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

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

type recipient struct {
	*load.Load

	deleteBtn       *cryptomaterial.Clickable
	description     cryptomaterial.Editor
	feeRateSelector *components.FeeRateSelector

	selectedWallet        sharedW.Asset
	selectedSourceAccount *sharedW.Account

	sendDestination *destination
	amount          *sendAmount
	*authoredTxData

	exchangeRate           float64
	isFetchingExchangeRate bool
	usdExchangeSet         bool
}

func newRecipient(l *load.Load, selectedWallet sharedW.Asset) *recipient {
	rp := &recipient{
		Load:           l,
		selectedWallet: selectedWallet,
		authoredTxData: &authoredTxData{},
		exchangeRate:   -1,
	}

	assetType := rp.selectedWallet.GetAssetType()

	rp.amount = newSendAmount(l.Theme, assetType)
	rp.sendDestination = newSendDestination(l, assetType)

	rp.description = rp.Theme.Editor(new(widget.Editor), values.String(values.StrNote))
	rp.description.Editor.SingleLine = false
	rp.description.Editor.SetText("")
	rp.description.IsTitleLabel = false
	// Set the maximum characters the editor can accept.
	rp.description.Editor.MaxLen = MaxTxLabelSize

	callbackFunc := func() libUtil.AssetType {
		return assetType
	}
	rp.feeRateSelector = components.NewFeeRateSelector(l, callbackFunc).ShowSizeAndCost()
	rp.feeRateSelector.TitleInset = layout.Inset{Bottom: values.MarginPadding10}
	rp.feeRateSelector.ContainerInset = layout.Inset{Bottom: values.MarginPadding100}
	rp.feeRateSelector.WrapperInset = layout.UniformInset(values.MarginPadding15)

	rp.sendDestination.addressChanged = func() {
		rp.validateAndConstructTx()
	}

	rp.amount.amountChanged = func() {
		rp.validateAndConstructTxAmountOnly()
	}

	return rp
}
func (rp *recipient) setDestinationAssetType(assetType libUtil.AssetType) {
	rp.amount.setAssetType(assetType)
	rp.sendDestination.initDestinationWalletSelector(assetType)
}

func (rp *recipient) serSourceAccount(sourceAccount *sharedW.Account) {
	rp.sourceAccount = sourceAccount
}

func (rp *recipient) initializeAccountSelectors(sourceAccount *sharedW.Account) {
	rp.selectedSourceAccount = sourceAccount
	rp.sendDestination.destinationAccountSelector = rp.sendDestination.destinationAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
		accountIsValid := account.Number != load.MaxInt32
		// Filter mixed wallet
		destinationWallet := rp.sendDestination.destinationAccountSelector.SelectedWallet()
		isMixedAccount := load.MixedAccountNumber(destinationWallet) == account.Number
		// Filter the sending account.

		sourceWalletID := sourceAccount.WalletID
		isSameAccount := sourceWalletID == account.WalletID && account.Number == sourceAccount.Number
		if !accountIsValid || isSameAccount || isMixedAccount {
			return false
		}
		return true
	})

	rp.sendDestination.destinationAccountSelector.AccountSelected(func(selectedWallet *sharedW.Account) {
		rp.validateAndConstructTx()
	})

	rp.sendDestination.destinationWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
		rp.sendDestination.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		//TODO
		// if rp.selectedWallet.GetAssetType() == libUtil.DCRWalletAsset {
		// 	rp.sourceAccountSelector.SelectFirstValidAccount(rp.selectedWallet)
		// }
	})
}

func (rp *recipient) validateAndConstructTx() {
	// delete all the previous errors set earlier.
	rp.amountValidationError("")
	rp.addressValidationError("")

	if rp.isValidated() {
		rp.constructTx()
	} else {
		rp.clearEstimates()
		rp.showBalanceAfterSend()
	}
}

func (rp *recipient) fetchExchangeRate() {
	if rp.isFetchingExchangeRate {
		return
	}
	rp.isFetchingExchangeRate = true
	var market string
	switch rp.selectedWallet.GetAssetType() {
	case libUtil.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libUtil.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libUtil.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", rp.selectedWallet.GetAssetType())
		rp.isFetchingExchangeRate = false
		return
	}

	rate := rp.AssetsManager.RateSource.GetTicker(market)
	if rate == nil || rate.LastTradePrice <= 0 {
		rp.isFetchingExchangeRate = false
		return
	}

	rp.exchangeRate = rate.LastTradePrice
	rp.amount.setExchangeRate(rp.exchangeRate)
	rp.validateAndConstructTx() // convert estimates to usd

	rp.isFetchingExchangeRate = false
	// rp.ParentWindow().Reload()
}

func (rp *recipient) validateAndConstructTxAmountOnly() {
	// defer rp.RefreshTheme(rp.ParentWindow())

	if !rp.sendDestination.validate() && rp.amount.amountIsValid() {
		rp.constructTx()
	} else {
		rp.validateAndConstructTx()
	}
}

func (rp *recipient) isValidated() bool {
	amountIsValid := rp.amount.amountIsValid()
	addressIsValid := rp.sendDestination.validate()

	// No need for checking the err message since it is as result of amount and
	// address validation.
	// validForSending
	return amountIsValid && addressIsValid
}

func (rp *recipient) constructTx() {
	destinationAddress, err := rp.sendDestination.destinationAddress()
	if err != nil {
		rp.addressValidationError(err.Error())
		return
	}
	destinationAccount := rp.sendDestination.destinationAccount()

	amountAtom, SendMax, err := rp.amount.validAmount()
	if err != nil {
		rp.amountValidationError(err.Error())
		return
	}

	selectedUTXOs := make([]*sharedW.UnspentOutput, 0)
	// if rp.selectedSourceAccount == rp.selectedUTXOs.sourceAccount {
	// 	selectedUTXOs = rp.selectedUTXOs.selectedUTXOs
	// }

	err = rp.selectedWallet.NewUnsignedTx(rp.selectedSourceAccount.Number, selectedUTXOs)
	if err != nil {
		rp.amountValidationError(err.Error())
		return
	}

	err = rp.selectedWallet.AddSendDestination(destinationAddress, amountAtom, SendMax)
	if err != nil {
		if strings.Contains(err.Error(), "amount") {
			rp.amountValidationError(err.Error())
			return
		}
		rp.addressValidationError(err.Error())
		return
	}

	feeAndSize, err := rp.selectedWallet.EstimateFeeAndSize()
	if err != nil {
		rp.amountValidationError(err.Error())
		return
	}

	feeAtom := feeAndSize.Fee.UnitValue
	spendableAmount := rp.selectedSourceAccount.Balance.Spendable.ToInt()
	// if len(selectedUTXOs) > 0 {
	// 	spendableAmount = rp.selectedUTXOs.totalUTXOsAmount
	// }

	if SendMax {
		amountAtom = spendableAmount - feeAtom
	}

	wal := rp.selectedWallet
	totalSendingAmount := wal.ToAmount(amountAtom + feeAtom)
	balanceAfterSend := wal.ToAmount(spendableAmount - totalSendingAmount.ToInt())

	// populate display data
	rp.txFee = wal.ToAmount(feeAtom).String()

	rp.feeRateSelector.EstSignedSize = fmt.Sprintf("%d Bytes", feeAndSize.EstimatedSignedSize)
	rp.feeRateSelector.TxFee = rp.txFee
	rp.feeRateSelector.SetFeerate(feeAndSize.FeeRate)
	rp.totalCost = totalSendingAmount.String()
	rp.balanceAfterSend = balanceAfterSend.String()
	rp.sendAmount = wal.ToAmount(amountAtom).String()
	rp.destinationAddress = destinationAddress
	rp.destinationAccount = destinationAccount
	rp.sourceAccount = rp.selectedSourceAccount

	if SendMax {
		// TODO: this workaround ignores the change events from the
		// amount input to avoid construct tx cycle.
		rp.amount.setAmount(amountAtom)
	}

	if rp.exchangeRate != -1 && rp.usdExchangeSet {
		rp.feeRateSelector.USDExchangeSet = true
		rp.txFeeUSD = fmt.Sprintf("$%.4f", utils.CryptoToUSD(rp.exchangeRate, feeAndSize.Fee.CoinValue))
		rp.feeRateSelector.TxFeeUSD = rp.txFeeUSD
		rp.totalCostUSD = utils.FormatAsUSDString(rp.Printer, utils.CryptoToUSD(rp.exchangeRate, totalSendingAmount.ToCoin()))
		rp.balanceAfterSendUSD = utils.FormatAsUSDString(rp.Printer, utils.CryptoToUSD(rp.exchangeRate, balanceAfterSend.ToCoin()))

		usdAmount := utils.CryptoToUSD(rp.exchangeRate, wal.ToAmount(amountAtom).ToCoin())
		rp.sendAmountUSD = utils.FormatAsUSDString(rp.Printer, usdAmount)
	}
}

func (rp *recipient) showBalanceAfterSend() {
	if rp.selectedSourceAccount != nil {
		if rp.selectedSourceAccount.Balance == nil {
			return
		}
		balanceAfterSend := rp.selectedSourceAccount.Balance.Spendable
		rp.balanceAfterSend = balanceAfterSend.String()
		rp.balanceAfterSendUSD = utils.FormatAsUSDString(rp.Printer, utils.CryptoToUSD(rp.exchangeRate, balanceAfterSend.ToCoin()))
	}
}

func (rp *recipient) clearEstimates() {
	rp.txFee = " - " + string(rp.selectedWallet.GetAssetType())
	rp.feeRateSelector.TxFee = rp.txFee
	rp.txFeeUSD = " - "
	rp.feeRateSelector.TxFeeUSD = rp.txFeeUSD
	rp.totalCost = " - " + string(rp.selectedWallet.GetAssetType())
	rp.totalCostUSD = " - "
	rp.balanceAfterSend = " - " + string(rp.selectedWallet.GetAssetType())
	rp.balanceAfterSendUSD = " - "
	rp.sendAmount = " - "
	rp.sendAmountUSD = " - "
	rp.feeRateSelector.SetFeerate(0)
}

func (rp *recipient) resetFields() {
	rp.sendDestination.clearAddressInput()
	rp.description.Editor.SetText("")

	rp.amount.resetFields()
}

func (rp *recipient) amountValidationError(err string) {
	rp.amount.setError(err)
	rp.clearEstimates()
}

func (rp *recipient) addressValidationError(err string) {
	rp.sendDestination.setError(err)
	rp.clearEstimates()
}

func (rp *recipient) resetDestinationAccountSelector() {
	rp.sendDestination.destinationAccountSelector.SelectFirstValidAccount(rp.selectedWallet)
	rp.validateAndConstructTx()
}

func (rp *recipient) recipientLayout(gtx C, index int, showIcon bool, window app.WindowNavigator) layout.Widget {
	rp.handle()
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return rp.topLayout(gtx, index, showIcon)
			}),
			layout.Rigid(func(gtx C) D {
				layoutBody := func(gtx C) D {
					return rp.contentWrapper(gtx, "Destination Address", rp.sendDestination.destinationAddressEditor.Layout)
				}
				// fmt.Println(rp.sendDestination.sendToAddress, "rp.sendDestination.sendToAddress")
				if !rp.sendDestination.sendToAddress {
					layoutBody = rp.walletAccountlayout(gtx, window)
				}
				return rp.sendDestination.accountSwitch.Layout(gtx, layoutBody)
			}),
			layout.Rigid(func(gtx C) D {
				return rp.addressAndAmountlayout(gtx, window)
			}),
			layout.Rigid(rp.txLabelSection),
		)
	}
}

func (rp *recipient) topLayout(gtx C, index int, showIcon bool) D {
	titleTxt := rp.Theme.Label(values.TextSize16, fmt.Sprintf("To: Recipient %v", index))
	titleTxt.Color = rp.Theme.Color.GrayText2
	if !showIcon {
		return titleTxt.Layout(gtx)
	}

	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return titleTxt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, rp.Theme.Icons.DeleteIcon.Layout20dp)
		}),
	)
}

func (rp *recipient) walletAccountlayout(gtx C, window app.WindowNavigator) layout.Widget {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return rp.contentWrapper(gtx, "Destination Wallet", func(gtx C) D {
					return rp.sendDestination.destinationWalletSelector.Layout(window, gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return rp.contentWrapper(gtx, values.String(values.StrAccount), func(gtx C) D {
					return rp.sendDestination.destinationAccountSelector.Layout(window, gtx)
				})
			}),
		)
	}
}

func (rp *recipient) contentWrapper(gtx C, title string, content layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := rp.Theme.Label(values.TextSize16, title)
				lbl.Font.Weight = font.SemiBold
				return layout.Inset{
					Bottom: values.MarginPadding4,
				}.Layout(gtx, lbl.Layout)
			}),
			layout.Rigid(content),
		)
	})
}

func (rp *recipient) addressAndAmountlayout(gtx C, window app.WindowNavigator) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// if rp.exchangeRate != -1 && rp.usdExchangeSet {
			// 	return layout.Flex{
			// 		Axis:      layout.Horizontal,
			// 		Alignment: layout.Middle,
			// 	}.Layout(gtx,
			// 		layout.Flexed(0.45, func(gtx C) D {
			// 			return rp.amount.amountEditor.Layout(gtx)
			// 		}),
			// 		layout.Flexed(0.1, func(gtx C) D {
			// 			return layout.Center.Layout(gtx, func(gtx C) D {
			// 				icon := rp.Theme.Icons.CurrencySwapIcon
			// 				return icon.Layout12dp(gtx)
			// 			})
			// 		}),
			// 		layout.Flexed(0.45, func(gtx C) D {
			// 			return rp.amount.usdAmountEditor.Layout(gtx)
			// 		}),
			// 	)
			// }
			return rp.contentWrapper(gtx, "Amount", rp.amount.amountEditor.Layout)
		}),
		// layout.Rigid(func(gtx C) D {
		// 	if rp.exchangeRateMessage == "" {
		// 		return layout.Dimensions{}
		// 	}
		// 	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// 		layout.Rigid(func(gtx C) D {
		// 			return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
		// 				gtx.Constraints.Min.X = gtx.Constraints.Max.X
		// 				gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding1)
		// 				return cryptomaterial.Fill(gtx, rp.Theme.Color.Gray1)
		// 			})
		// 		}),
		// 		layout.Rigid(func(gtx C) D {
		// 			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// 				layout.Rigid(func(gtx C) D {
		// 					label := rp.Theme.Body2(rp.exchangeRateMessage)
		// 					label.Color = rp.Theme.Color.Danger
		// 					if rp.isFetchingExchangeRate {
		// 						label.Color = rp.Theme.Color.Primary
		// 					}
		// 					return label.Layout(gtx)
		// 				}),
		// 				layout.Rigid(func(gtx C) D {
		// 					if rp.isFetchingExchangeRate {
		// 						return layout.Dimensions{}
		// 					}
		// 					gtx.Constraints.Min.X = gtx.Constraints.Max.X
		// 					return layout.E.Layout(gtx, rp.retryExchange.Layout)
		// 				}),
		// 			)
		// 		}),
		// 	)
		// }),
	)
}

func (rp *recipient) txLabelSection(gtx C) D {
	count := len(rp.description.Editor.Text())
	txt := fmt.Sprintf("%s (%d/%d)", values.String(values.StrDescriptionNote), count, rp.description.Editor.MaxLen)
	return rp.contentWrapper(gtx, txt, rp.description.Layout)
}

func (rp *recipient) validateAmount() {
	if len(rp.amount.amountEditor.Editor.Text()) > 0 {
		rp.amount.validateAmount()
		rp.validateAndConstructTxAmountOnly()
	}
}

func (rp *recipient) handle() {
	rp.sendDestination.handle()
	rp.amount.handle()

	if rp.amount.IsMaxClicked() {
		rp.amount.setError("")
		rp.amount.SendMax = true
		rp.amount.amountChanged()
	}

	// if destination switch is equal to Address
	if rp.sendDestination.sendToAddress {
		if rp.sendDestination.validate() {
			if !rp.AssetsManager.ExchangeRateFetchingEnabled() {
				if len(rp.amount.amountEditor.Editor.Text()) == 0 {
					rp.amount.SendMax = false
				}
			} else {
				if len(rp.amount.amountEditor.Editor.Text()) == 0 {
					rp.amount.usdAmountEditor.Editor.SetText("")
					rp.amount.SendMax = false
				}
			}
		}
	} else {
		if !rp.AssetsManager.ExchangeRateFetchingEnabled() {
			if len(rp.amount.amountEditor.Editor.Text()) == 0 {
				rp.amount.SendMax = false
			}
		} else {
			if len(rp.amount.amountEditor.Editor.Text()) == 0 {
				rp.amount.usdAmountEditor.Editor.SetText("")
				rp.amount.SendMax = false
			}
		}
	}
}
