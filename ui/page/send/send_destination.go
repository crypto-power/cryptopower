package send

import (
	"fmt"
	"strings"

	"gioui.org/widget"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

var tabOptions = []string{
	values.String(values.StrAddress),
	values.String(values.StrWallets),
}

type destination struct {
	*load.Load

	addressChanged             func()
	destinationAddressEditor   cryptomaterial.Editor
	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	sendToAddress bool
	accountSwitch *cryptomaterial.SegmentedControl

	selectedIndex int
}

func newSendDestination(l *load.Load, assetType libUtil.AssetType) *destination {
	dst := &destination{
		Load:          l,
		accountSwitch: l.Theme.SegmentedControl(tabOptions, cryptomaterial.SegmentTypeGroupMax),
	}

	dst.destinationAddressEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrDestAddr))
	dst.destinationAddressEditor.Editor.SingleLine = true
	dst.destinationAddressEditor.Editor.SetText("")

	dst.initDestinationWalletSelector(assetType)
	return dst
}

func (dst *destination) initDestinationWalletSelector(assetType libUtil.AssetType) {
	// Destination wallet picker
	dst.destinationWalletSelector = components.NewWalletAndAccountSelector(dst.Load, assetType).
		EnableWatchOnlyWallets(true).
		Title(values.String(values.StrTo))

	// Destination account picker
	dst.destinationAccountSelector = components.NewWalletAndAccountSelector(dst.Load).
		EnableWatchOnlyWallets(true).
		Title(values.String(values.StrAccount))
	dst.destinationAccountSelector.SelectFirstValidAccount(dst.destinationWalletSelector.SelectedWallet())
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

	wal := dst.AssetsManager.WalletWithID(destinationAccount.WalletID)
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

	if dst.destinationWalletSelector.SelectedWallet().IsAddressValid(address) {
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
	if dst.sendToAddress {
		dst.destinationAddressEditor.SetError(errMsg)
	} else {
		dst.destinationAccountSelector.SetError(errMsg)
	}
}

func (dst *destination) clearAddressInput() {
	dst.destinationAddressEditor.SetError("")
	dst.destinationAddressEditor.Editor.SetText("")
}

func (dst *destination) handle() {
	if dst.accountSwitch.SelectedSegment() == values.String(values.StrAddress) {
		dst.sendToAddress = true
	}

	if dst.accountSwitch.Changed() {
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
	// dst.accountSwitch.Active, dst.accountSwitch.Inactive = dst.Theme.Color.Surface, color.NRGBA{}
	// dst.accountSwitch.ActiveTextColor, dst.accountSwitch.InactiveTextColor = dst.Theme.Color.GrayText1, dst.Theme.Color.Surface
	dst.destinationAddressEditor.EditorStyle.Color = dst.Theme.Color.Text
}
