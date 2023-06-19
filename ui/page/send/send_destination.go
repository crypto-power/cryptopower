package send

import (
	"fmt"
	"image/color"
	"strings"

	"gioui.org/widget"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	sendToAddress int = 1
	SendToWallet  int = 2
)

type destination struct {
	*load.Load

	addressChanged             func()
	destinationAddressEditor   cryptomaterial.Editor
	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	sendToAddress bool
	accountSwitch *cryptomaterial.SwitchButtonText

	selectedIndex int
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
		EnableWatchOnlyWallets(true).
		Title(values.String(values.StrTo))

	// Destination account picker
	dst.destinationAccountSelector = components.NewWalletAndAccountSelector(dst.Load).
		EnableWatchOnlyWallets(true).
		Title(values.String(values.StrAccount))
	dst.destinationAccountSelector.SelectFirstValidAccount(dst.destinationWalletSelector.SelectedWallet())

	return dst
}

// destinationAddress validates the destination address obtained from the provided
// raw address or the selected account address.
func (dst *destination) destinationAddress() (string, error) {
	if dst.sendToAddress {
		return dst.validateDestinationAddress()
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

// validateDestinationAddress checks if raw address provided as destination is
// valid.
func (dst *destination) validateDestinationAddress() (string, error) {
	address := dst.destinationAddressEditor.Editor.Text()
	address = strings.TrimSpace(address)

	if address == "" {
		return address, fmt.Errorf(values.String(values.StrDestinationMissing))
	}

	if dst.WL.SelectedWallet.Wallet.IsAddressValid(address) {
		dst.destinationAddressEditor.SetError("")
		return address, nil
	}

	return address, fmt.Errorf(values.String(values.StrInvalidAddress))
}

func (dst *destination) validate() bool {
	if dst.sendToAddress {
		_, err := dst.validateDestinationAddress()
		// if err equals to nil then the address is valid.
		return err == nil
	}

	return true
}

func (dst *destination) setError(errMsg string) {
	switch dst.accountSwitch.SelectedIndex() {
	case SendToWallet:
		dst.destinationAccountSelector.SetError(errMsg)
	default: // SendToAddress option
		dst.destinationAddressEditor.SetError(errMsg)
	}
}

func (dst *destination) clearAddressInput() {
	dst.destinationAddressEditor.SetError("")
	dst.destinationAddressEditor.Editor.SetText("")
}

func (dst *destination) handle() {
	dst.selectedIndex = dst.accountSwitch.SelectedIndex()
	if dst.selectedIndex == 0 {
		dst.selectedIndex = sendToAddress // default value is sendToAddress option
	}

	isSendToAddress := dst.accountSwitch.SelectedIndex() == sendToAddress
	if isSendToAddress != dst.sendToAddress { // switch changed
		dst.sendToAddress = isSendToAddress
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
