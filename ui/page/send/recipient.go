package send

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type recipient struct {
	*load.Load

	deleteBtn   *cryptomaterial.Clickable
	description cryptomaterial.Editor

	selectedWallet        sharedW.Asset
	selectedSourceAccount *sharedW.Account
	sourceAccount         *sharedW.Account

	sendDestination *destination
	amount          *sendAmount

	exchangeRate           float64
	isFetchingExchangeRate bool
	usdExchangeSet         bool
}

func newRecipient(l *load.Load, selectedWallet sharedW.Asset) *recipient {
	rp := &recipient{
		Load:           l,
		selectedWallet: selectedWallet,
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

	return rp
}

func (rp *recipient) onAddressChanged(addressChanged func()) {
	rp.sendDestination.addressChanged = addressChanged
}

func (rp *recipient) onAmountChanged(amountChanged func()) {
	rp.amount.amountChanged = amountChanged
}

func (rp *recipient) setDestinationAssetType(assetType libUtil.AssetType) {
	rp.amount.setAssetType(assetType)
	rp.sendDestination.initDestinationWalletSelector(assetType)
}

func (rp *recipient) setSourceAccount(sourceAccount *sharedW.Account) {
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
		rp.sendDestination.addressChanged()
	})

	rp.sendDestination.destinationWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
		rp.sendDestination.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		//TODO this should not be here.
		// if rp.selectedWallet.GetAssetType() == libUtil.DCRWalletAsset {
		// 	rp.sourceAccountSelector.SelectFirstValidAccount(rp.selectedWallet)
		// }
	})

	// destinationAccountSelector does not have a default value,
	// so assign it an initial value here
	rp.sendDestination.destinationAccountSelector.SelectFirstValidAccount(rp.sendDestination.destinationWalletSelector.SelectedWallet())
	rp.sendDestination.destinationAddressEditor.Editor.Focus()
}

func (rp *recipient) destinationWalletID() int {
	return rp.sendDestination.destinationWalletSelector.SelectedWallet().GetWalletID()
}

func (rp *recipient) isSendToAddress() bool {
	return rp.sendDestination.sendToAddress
}

func (rp *recipient) isValidated() bool {
	amountIsValid := rp.amount.amountIsValid()
	addressIsValid := rp.sendDestination.validate()

	// No need for checking the err message since it is as result of amount and
	// address validation.
	// validForSending
	return amountIsValid && addressIsValid
}

func (rp *recipient) resetFields() {
	rp.sendDestination.clearAddressInput()
	rp.description.Editor.SetText("")

	rp.amount.resetFields()
}

func (rp *recipient) destinationAddress() string {
	address, err := rp.sendDestination.destinationAddress()
	if err != nil {
		rp.addressValidationError(err.Error())
		return ""
	}
	return address
}

func (rp *recipient) destinationAccount() *sharedW.Account {
	return rp.sendDestination.destinationAccount()
}

func (rp *recipient) descriptionText() string {
	return rp.description.Editor.Text()
}

func (rp *recipient) addressValidated() bool {
	return rp.sendDestination.validate()
}

func (rp *recipient) validAmount() (int64, bool) {
	amountAtom, sendMax, err := rp.amount.validAmount()
	if err != nil {
		rp.amountValidationError(err.Error())
		return -1, false
	}

	return amountAtom, sendMax
}

func (rp *recipient) amountValidated() bool {
	return rp.amount.amountIsValid()
}

func (rp *recipient) setAmount(amount int64) {
	rp.amount.setAmount(amount)
}

func (rp *recipient) amountValidationError(err string) {
	rp.amount.setError(err)
}

func (rp *recipient) addressValidationError(err string) {
	rp.sendDestination.setError(err)
}

func (rp *recipient) resetDestinationAccountSelector() {
	rp.sendDestination.destinationAccountSelector.SelectFirstValidAccount(rp.selectedWallet)
}

func (rp *recipient) recipientLayout(index int, showIcon bool, window app.WindowNavigator) layout.Widget {
	rp.handle()
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if showIcon {
					return rp.topLayout(gtx, index)
				}
				return D{}
			}),
			layout.Rigid(func(gtx C) D {
				layoutBody := func(gtx C) D {
					txt := fmt.Sprintf("%s %s", values.String(values.StrDestination), values.String(values.StrAddress))
					return rp.contentWrapper(gtx, txt, rp.sendDestination.destinationAddressEditor.Layout)
				}
				if !rp.sendDestination.sendToAddress {
					layoutBody = rp.walletAccountlayout(window)
				}
				return rp.sendDestination.accountSwitch.Layout(gtx, layoutBody)
			}),
			layout.Rigid(rp.addressAndAmountlayout),
			layout.Rigid(rp.txLabelSection),
		)
	}
}

func (rp *recipient) topLayout(gtx C, index int) D {
	txt := fmt.Sprintf("%s: %s %v", values.String(values.StrTo), values.String(values.StrRecipient), index)
	titleTxt := rp.Theme.Label(values.TextSize16, txt)
	titleTxt.Color = rp.Theme.Color.GrayText2

	return layout.Flex{}.Layout(gtx,
		layout.Rigid(titleTxt.Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, rp.Theme.Icons.DeleteIcon.Layout20dp)
		}),
	)
}

func (rp *recipient) walletAccountlayout(window app.WindowNavigator) layout.Widget {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				txt := fmt.Sprintf("%s %s", values.String(values.StrDestination), values.String(values.StrWallet))
				return rp.contentWrapper(gtx, txt, func(gtx C) D {
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

func (rp *recipient) addressAndAmountlayout(gtx C) D {
	return rp.contentWrapper(gtx, values.String(values.StrAmount), rp.amount.amountEditor.Layout)
}

func (rp *recipient) txLabelSection(gtx C) D {
	count := len(rp.description.Editor.Text())
	txt := fmt.Sprintf("%s (%d/%d)", values.String(values.StrDescriptionNote), count, rp.description.Editor.MaxLen)
	return rp.contentWrapper(gtx, txt, rp.description.Layout)
}

func (rp *recipient) validateAmount() {
	if len(rp.amount.amountEditor.Editor.Text()) > 0 {
		rp.amount.validateAmount()
	}
}

func (rp *recipient) restyleWidgets() {
	rp.amount.styleWidgets()
	rp.sendDestination.styleWidgets()
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
