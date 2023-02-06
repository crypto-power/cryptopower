package send

import (
	"fmt"
	"image/color"
	"strings"

	"gioui.org/widget"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type destination struct {
	*load.Load

	addressChanged             func()
	destinationAddressEditor   cryptomaterial.Editor
	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	sendToAddress bool
	accountSwitch *cryptomaterial.SwitchButtonText
}

func newSendDestination(l *load.Load) *destination {
	dst := &destination{
		Load: l,
	}

	dst.destinationAddressEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrDestAddr))
	dst.destinationAddressEditor.Editor.SingleLine = true
	dst.destinationAddressEditor.Editor.SetText("")

	dst.accountSwitch = l.Theme.SwitchButtonText([]cryptomaterial.SwitchItem{
		{Text: values.String(values.StrAddress)},
		{Text: values.String(values.StrWallets)},
	})

	// Destination wallet picker
	dst.destinationWalletSelector = components.NewWalletAndAccountSelector(dst.Load, l.WL.SelectedWallet.Wallet.GetAssetType()).
		Title(values.String(values.StrTo))

	// Destination account picker
	dst.destinationAccountSelector = components.NewWalletAndAccountSelector(dst.Load).
		Title(values.String(values.StrAccount))
	dst.destinationAccountSelector.SelectFirstValidAccount(dst.destinationWalletSelector.SelectedWallet())

	return dst
}

func (dst *destination) destinationAddress() (string, error) {
	if dst.sendToAddress {
		valid, address := dst.validateDestinationAddress()
		if valid {
			return address, nil
		}

		return "", fmt.Errorf(values.String(values.StrInvalidAddress))
	}
	destinationAccount := dst.destinationAccountSelector.SelectedAccount()
	if destinationAccount == nil {
		return "", fmt.Errorf(values.String(values.StrInvalidAddress))
	}
	wal := dst.WL.AssetsManager.WalletWithID(destinationAccount.WalletID)

	return wal.CurrentAddress(destinationAccount.Number)
}

func (dst *destination) destinationAccount() *sharedW.Account {
	if dst.sendToAddress {
		return nil
	}

	return dst.destinationAccountSelector.SelectedAccount()
}

func (dst *destination) isDestinationAccountExist() bool {
	return dst.destinationAccountSelector.SelectedAccount() != nil
}

func (dst *destination) validateDestinationAddress() (bool, string) {

	address := dst.destinationAddressEditor.Editor.Text()
	address = strings.TrimSpace(address)

	if len(address) == 0 {
		dst.destinationAddressEditor.SetError("")
		return false, address
	}

	if dst.WL.SelectedWallet.Wallet.IsAddressValid(address) {
		dst.destinationAddressEditor.SetError("")
		return true, address
	}

	dst.destinationAddressEditor.SetError(values.String(values.StrInvalidAddress))
	return false, address
}

func (dst *destination) validate() bool {
	if dst.sendToAddress {
		validAddress, _ := dst.validateDestinationAddress()
		return validAddress
	}

	return true
}

func (dst *destination) clearAddressInput() {
	dst.destinationAddressEditor.SetError("")
	dst.destinationAddressEditor.Editor.SetText("")
}

func (dst *destination) handle() {
	sendToAddress := dst.accountSwitch.SelectedIndex() == 1
	if sendToAddress != dst.sendToAddress { // switch changed
		dst.sendToAddress = sendToAddress
		dst.addressChanged()
	}

	for _, evt := range dst.destinationAddressEditor.Editor.Events() {
		if dst.destinationAddressEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				dst.addressChanged()
			}
		}
	}
}

// styleWidgets sets the appropriate colors for the destination widgets.
func (dst *destination) styleWidgets() {
	dst.accountSwitch.Active, dst.accountSwitch.Inactive = dst.Theme.Color.Surface, color.NRGBA{}
	dst.accountSwitch.ActiveTextColor, dst.accountSwitch.InactiveTextColor = dst.Theme.Color.GrayText1, dst.Theme.Color.Surface
	dst.destinationAddressEditor.EditorStyle.Color = dst.Theme.Color.Text
}
