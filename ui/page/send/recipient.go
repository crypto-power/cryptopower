package send

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type recipient struct {
	*load.Load
	id int

	deleteBtn   *cryptomaterial.Clickable
	description cryptomaterial.Editor

	selectedWallet        sharedW.Asset
	selectedSourceAccount *sharedW.Account
	sourceAccount         *sharedW.Account

	sendDestination *destination
	amount          *sendAmount
	pageParam       getPageFields
}

func newRecipient(l *load.Load, selectedWallet sharedW.Asset, pageParam getPageFields, id int) *recipient {
	rp := &recipient{
		Load:           l,
		selectedWallet: selectedWallet,
		pageParam:      pageParam,
		id:             id,
	}

	assetType := rp.selectedWallet.GetAssetType()

	rp.amount = newSendAmount(l.Theme, assetType)
	rp.amount.amountEditor.TextSize = values.TextSizeTransform(l.IsMobileView(), values.TextSize16)
	rp.sendDestination = newSendDestination(l, assetType)

	rp.description = rp.Theme.Editor(new(widget.Editor), values.String(values.StrNote))
	rp.description.Editor.SingleLine = false
	rp.description.Editor.SetText("")
	rp.description.IsTitleLabel = false
	// Set the maximum characters the editor can accept.
	rp.description.Editor.MaxLen = MaxTxLabelSize
	rp.description.TextSize = values.TextSizeTransform(l.IsMobileView(), values.TextSize16)

	return rp
}

func (rp *recipient) onAddressChanged(addressChanged func()) {
	rp.sendDestination.addressChanged = addressChanged
}

func (rp *recipient) onAmountChanged(amountChanged func()) {
	rp.amount.amountChanged = amountChanged
}

func (rp *recipient) cleanAllErrors() {
	rp.amount.setError("")
	rp.sendDestination.setError("")
}

func (rp *recipient) setDestinationAssetType(assetType libUtil.AssetType) {
	rp.amount.setAssetType(assetType)
	rp.sendDestination.initDestinationWalletSelector(assetType)
}

func (rp *recipient) setSourceAccount(sourceAccount *sharedW.Account) {
	rp.sourceAccount = sourceAccount
}

func (rp *recipient) isAccountValid(sourceAccount, account *sharedW.Account) bool {
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
}

func (rp *recipient) initializeAccountSelectors(sourceAccount *sharedW.Account) {
	rp.selectedSourceAccount = sourceAccount
	rp.sendDestination.destinationAccountSelector = rp.sendDestination.destinationAccountSelector.AccountValidator(func(account *sharedW.Account) bool {
		return rp.isAccountValid(sourceAccount, account)
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

func (rp *recipient) isShowSendToWallet() bool {
	sourceWalletSelected := rp.sendDestination.destinationWalletSelector.SelectedWallet()
	var wallets []sharedW.Asset
	switch sourceWalletSelected.GetAssetType() {
	case utils.BTCWalletAsset:
		wallets = append(wallets, rp.AssetsManager.AllBTCWallets()...)
	case utils.DCRWalletAsset:
		wallets = append(wallets, rp.AssetsManager.AllDCRWallets()...)
	case utils.LTCWalletAsset:
		wallets = append(wallets, rp.AssetsManager.AllLTCWallets()...)
	}

	if len(wallets) == 1 {
		account, err := wallets[0].GetAccountsRaw()
		if err != nil {
			log.Errorf("Error getting accounts:", err)
			return false
		}
		accountValids := make([]sharedW.Account, 0)
		for _, acc := range account.Accounts {
			sourceAccountSelected := rp.sendDestination.destinationAccountSelector.SelectedAccount()
			if rp.isAccountValid(sourceAccountSelected, acc) {
				accountValids = append(accountValids, *acc)
			}
		}
		return len(accountValids) > 1
	}

	if len(wallets) > 1 {
		return true
	}

	return false
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

				if !rp.isShowSendToWallet() {
					return layoutBody(gtx)
				}

				if !rp.sendDestination.sendToAddress {
					layoutBody = rp.walletAccountlayout(window)
				}

				return rp.sendDestination.accountSwitch.Layout(gtx, layoutBody, rp.IsMobileView())
			}),
			layout.Rigid(rp.addressAndAmountlayout),
			layout.Rigid(rp.txLabelSection),
		)
	}
}

func (rp *recipient) topLayout(gtx C, index int) D {
	txt := fmt.Sprintf("%s: %s %v", values.String(values.StrTo), values.String(values.StrRecipient), index)
	titleTxt := rp.Theme.Label(values.TextSizeTransform(rp.IsMobileView(), values.TextSize16), txt)
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
				txt := fmt.Sprintf("%s %s", values.String(values.StrDestination), values.String(values.StrAccount))
				return rp.contentWrapper(gtx, txt, func(gtx C) D {
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
				lbl := rp.Theme.Label(values.TextSizeTransform(rp.IsMobileView(), values.TextSize16), title)
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
	widget := func(gtx C) D { return rp.amount.amountEditor.Layout(gtx) }
	if rp.pageParam().exchangeRate != -1 && rp.pageParam().usdExchangeSet {
		widget = func(gtx C) D {
			icon := cryptomaterial.NewIcon(rp.Theme.Icons.ActionSwapHoriz)
			axis := layout.Horizontal
			var flexChilds []layout.FlexChild
			flexChilds = []layout.FlexChild{
				layout.Flexed(0.45, rp.amount.amountEditor.Layout),
				layout.Flexed(0.1, func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D {
						return icon.Layout(gtx, values.MarginPadding16)
					})
				}),
				layout.Flexed(0.45, rp.amount.usdAmountEditor.Layout),
			}
			if rp.IsMobileView() {
				axis = layout.Vertical
				icon = cryptomaterial.NewIcon(rp.Theme.Icons.ActionSwapVertical)
				flexChilds = []layout.FlexChild{
					layout.Rigid(rp.amount.amountEditor.Layout),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding10}.Layout),
					layout.Rigid(func(gtx C) D {
						return icon.Layout(gtx, values.MarginPadding16)
					}),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding10}.Layout),
					layout.Rigid(rp.amount.usdAmountEditor.Layout),
				}
			}
			return layout.Flex{
				Axis:      axis,
				Alignment: layout.Middle,
			}.Layout(gtx, flexChilds...)
		}

	}
	return rp.contentWrapper(gtx, values.String(values.StrAmount), widget)
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
