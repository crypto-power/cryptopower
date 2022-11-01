package send

import (
	"fmt"
	"strconv"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	libUtil "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/utils"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/decred/dcrd/dcrutil/v4"
)

type sendAmount struct {
	*load.Load

	amountEditor    cryptomaterial.Editor
	usdAmountEditor cryptomaterial.Editor

	SendMax               bool
	sendMaxChangeEvent    bool
	usdSendMaxChangeEvent bool
	amountChanged         func()

	amountErrorText string

	exchangeRate float64
}

func newSendAmount(l *load.Load) *sendAmount {

	sa := &sendAmount{
		Load:         l,
		exchangeRate: -1,
	}

	hit := fmt.Sprintf("%s (%s)", values.String(values.StrAmount), string(l.WL.SelectedWallet.Wallet.GetAssetType()))
	sa.amountEditor = l.Theme.Editor(new(widget.Editor), hit)
	sa.amountEditor.Editor.SetText("")
	sa.amountEditor.HasCustomButton = true
	sa.amountEditor.Editor.SingleLine = true

	sa.amountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	sa.amountEditor.CustomButton.Text = values.String(values.StrMax)
	sa.amountEditor.CustomButton.CornerRadius = values.MarginPadding0

	sa.usdAmountEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrAmount)+" (USD)")
	sa.usdAmountEditor.Editor.SetText("")
	sa.usdAmountEditor.HasCustomButton = true
	sa.usdAmountEditor.Editor.SingleLine = true

	sa.usdAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	sa.usdAmountEditor.CustomButton.Text = values.String(values.StrMax)
	sa.usdAmountEditor.CustomButton.CornerRadius = values.MarginPadding0

	sa.styleWidgets()

	return sa
}

// styleWidgets sets the appropriate colors for the amount widgets.
func (sa *sendAmount) styleWidgets() {
	sa.amountEditor.CustomButton.Background = sa.Theme.Color.Gray1
	sa.amountEditor.CustomButton.Color = sa.Theme.Color.Surface
	sa.amountEditor.EditorStyle.Color = sa.Theme.Color.Text

	sa.usdAmountEditor.CustomButton.Background = sa.Theme.Color.Gray1
	sa.usdAmountEditor.CustomButton.Color = sa.Theme.Color.Surface
	sa.usdAmountEditor.EditorStyle.Color = sa.Theme.Color.Text
}

func (sa *sendAmount) setExchangeRate(exchangeRate float64) {
	sa.exchangeRate = exchangeRate
	sa.validateAmount() // convert dcr input to usd
}

func (sa *sendAmount) setAmount(amount int64) {
	// TODO: this workaround ignores the change events from the
	// amount input to avoid construct tx cycle.
	sa.sendMaxChangeEvent = sa.SendMax
	amountSet := dcrutil.Amount(amount).ToCoin()
	if sa.Load.WL.SelectedWallet.Wallet.GetAssetType() == libUtil.BTCWalletAsset {
		amountSet = btcutil.Amount(amount).ToBTC()
	}
	sa.amountEditor.Editor.SetText(fmt.Sprintf("%.8f", amountSet))

	if sa.exchangeRate != -1 {
		usdAmount := utils.CryptoToUSD(sa.exchangeRate, amountSet)
		sa.usdSendMaxChangeEvent = true
		sa.usdAmountEditor.Editor.SetText(fmt.Sprintf("%.2f", usdAmount))
	}
}

func (sa *sendAmount) amountIsValid() bool {
	_, err := strconv.ParseFloat(sa.amountEditor.Editor.Text(), 64)
	amountEditorErrors := sa.amountErrorText == ""
	return err == nil && amountEditorErrors || sa.SendMax
}

func (sa *sendAmount) validAmount() (int64, bool, error) {
	if sa.SendMax {
		return 0, sa.SendMax, nil
	}

	amount, err := strconv.ParseFloat(sa.amountEditor.Editor.Text(), 64)
	if err != nil {
		return -1, sa.SendMax, err
	}

	if sa.Load.WL.SelectedWallet.Wallet.GetAssetType() == libUtil.BTCWalletAsset {
		return btc.AmountSatoshi(amount), sa.SendMax, nil
	}
	return dcr.AmountAtom(amount), sa.SendMax, nil
}

func (sa *sendAmount) validateAmount() {
	sa.amountErrorText = ""
	if sa.inputsNotEmpty(sa.amountEditor.Editor) {
		amount, err := strconv.ParseFloat(sa.amountEditor.Editor.Text(), 64)
		if err != nil {
			// empty usd input
			sa.usdAmountEditor.Editor.SetText("")
			sa.amountErrorText = values.String(values.StrInvalidAmount)
			return
		}

		if sa.exchangeRate != -1 {
			usdAmount := utils.CryptoToUSD(sa.exchangeRate, amount)
			sa.usdAmountEditor.Editor.SetText(fmt.Sprintf("%.2f", usdAmount)) // 2 decimal places
		}

		return
	}

	// empty usd input since this is empty
	sa.usdAmountEditor.Editor.SetText("")
}

// validateUSDAmount is called when usd text changes
func (sa *sendAmount) validateUSDAmount() bool {

	sa.amountErrorText = ""
	if sa.inputsNotEmpty(sa.usdAmountEditor.Editor) {
		usdAmount, err := strconv.ParseFloat(sa.usdAmountEditor.Editor.Text(), 64)
		if err != nil {
			// empty dcr input
			sa.amountEditor.Editor.SetText("")
			sa.amountErrorText = values.String(values.StrInvalidAmount)
			return false
		}

		if sa.exchangeRate != -1 {
			dcrAmount := utils.USDToDCR(sa.exchangeRate, usdAmount)
			sa.amountEditor.Editor.SetText(fmt.Sprintf("%.8f", dcrAmount)) // 8 decimal places
		}

		return true
	}

	// empty dcr input since this is empty
	sa.amountEditor.Editor.SetText("")
	return false
}

func (sa *sendAmount) inputsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if e.Text() == "" {
			return false
		}
	}
	return true
}

func (sa *sendAmount) setError(err string) {
	sa.amountErrorText = values.TranslateErr(err)
}

func (sa *sendAmount) resetFields() {
	sa.SendMax = false

	sa.clearAmount()
}

func (sa *sendAmount) clearAmount() {
	sa.amountErrorText = ""
	sa.amountEditor.Editor.SetText("")
	sa.usdAmountEditor.Editor.SetText("")
}

func (sa *sendAmount) handle() {
	sa.amountEditor.SetError(sa.amountErrorText)

	if sa.amountErrorText != "" {
		sa.amountEditor.LineColor = sa.Theme.Color.Danger
		sa.usdAmountEditor.LineColor = sa.Theme.Color.Danger
	} else {
		sa.amountEditor.LineColor = sa.Theme.Color.Gray2
		sa.usdAmountEditor.LineColor = sa.Theme.Color.Gray2
	}

	if sa.SendMax {
		sa.amountEditor.CustomButton.Background = sa.Theme.Color.Primary
		sa.usdAmountEditor.CustomButton.Background = sa.Theme.Color.Primary
	} else if len(sa.amountEditor.Editor.Text()) < 1 || !sa.SendMax {
		sa.amountEditor.CustomButton.Background = sa.Theme.Color.Gray1
		sa.usdAmountEditor.CustomButton.Background = sa.Theme.Color.Gray1
	}

	for _, evt := range sa.amountEditor.Editor.Events() {
		if sa.amountEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if sa.sendMaxChangeEvent {
					sa.sendMaxChangeEvent = false
					continue
				}
				sa.SendMax = false
				sa.validateAmount()
				sa.amountChanged()

			}
		}
	}

	for _, evt := range sa.usdAmountEditor.Editor.Events() {
		if sa.usdAmountEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if sa.usdSendMaxChangeEvent {
					sa.usdSendMaxChangeEvent = false
					continue
				}
				sa.SendMax = false
				sa.validateUSDAmount()
				sa.amountChanged()
			}
		}
	}
}

func (sa *sendAmount) IsMaxClicked() bool {
	if sa.amountEditor.CustomButton.Clicked() || sa.usdAmountEditor.CustomButton.Clicked() {
		return true
	}
	return false
}
