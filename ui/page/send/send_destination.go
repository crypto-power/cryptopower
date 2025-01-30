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
	values.StrAddress,
	values.StrWallets,
}

type destination struct {
	*load.Load

	addressChanged           func()
	destinationAddressEditor cryptomaterial.Editor
	sourceAccount            *sharedW.Account

	walletDropdown  *components.WalletDropdown
	accountDropdown *components.AccountDropdown

	accountSwitch *cryptomaterial.SegmentedControl
}

func newSendDestination(l *load.Load, assetType libUtil.AssetType) *destination {
	dst := &destination{
		Load:          l,
		accountSwitch: l.Theme.SegmentedControl(tabOptions, cryptomaterial.SegmentTypeGroupMax),
	}

	dst.accountSwitch.SetEnableSwipe(false)
	dst.accountSwitch.DisableUniform(true)

	dst.destinationAddressEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrDestAddr))
	dst.destinationAddressEditor.TextSize = values.TextSizeTransform(l.IsMobileView(), values.TextSize16)
	dst.destinationAddressEditor.Editor.SingleLine = true
	dst.destinationAddressEditor.Editor.SetText("")
	dst.destinationAddressEditor.IsTitleLabel = false

	dst.initDestinationWalletSelector(assetType)
	return dst
}

func (dst *destination) initDestinationWalletSelector(assetType libUtil.AssetType) {
	dst.walletDropdown = components.NewWalletDropdown(dst.Load, assetType).
		SetChangedCallback(func(wallet sharedW.Asset) {
			if dst.accountDropdown != nil {
				_ = dst.accountDropdown.Setup(wallet)
			}
		}).
		WalletValidator(func(wallet sharedW.Asset) bool {
			if dst.sourceAccount == nil {
				return true
			}
			if wallet.GetWalletID() == dst.sourceAccount.WalletID {
				account, err := wallet.GetAccountsRaw()
				if err != nil || len(account.Accounts) < 2 {
					return false
				}
			}
			return true
		}).
		EnableWatchOnlyWallets(true).
		Setup()
	dst.accountDropdown = components.NewAccountDropdown(dst.Load).
		SetChangedCallback(func(_ *sharedW.Account) {
			dst.addressChanged()
		}).
		AccountValidator(func(account *sharedW.Account) bool {
			if dst.sourceAccount == nil {
				return false
			}
			accountIsValid := account.Number != load.MaxInt32
			// Filter mixed wallet
			destinationWallet := dst.walletDropdown.SelectedWallet()
			isMixedAccount := load.MixedAccountNumber(destinationWallet) == account.Number

			// Filter the sending account.
			sourceWalletID := dst.sourceAccount.WalletID
			isSameAccount := sourceWalletID == account.WalletID && account.Number == dst.sourceAccount.Number
			if !accountIsValid || isSameAccount || isMixedAccount {
				return false
			}
			return true
		}).
		Setup(dst.walletDropdown.SelectedWallet())
}

// destinationAddress validates the destination address obtained from the provided
// raw address or the selected account address.
func (dst *destination) destinationAddress() (string, error) {
	if dst.isSendToAddress() {
		return dst.validateDestinationAddress()
	}

	destinationAccount := dst.accountDropdown.SelectedAccount()
	if destinationAccount == nil {
		return "", fmt.Errorf(values.String(values.StrInvalidAddress))
	}

	wal := dst.AssetsManager.WalletWithID(destinationAccount.WalletID)
	return wal.CurrentAddress(destinationAccount.Number)
}

func (dst *destination) destinationAccount() *sharedW.Account {
	if dst.isSendToAddress() {
		return nil
	}

	return dst.accountDropdown.SelectedAccount()
}

// validateDestinationAddress checks if raw address provided as destination is
// valid.
func (dst *destination) validateDestinationAddress() (string, error) {
	address := dst.destinationAddressEditor.Editor.Text()
	address = strings.TrimSpace(address)

	if address == "" {
		return address, fmt.Errorf(values.String(values.StrDestinationMissing))
	}

	if dst.walletDropdown != nil && dst.walletDropdown.SelectedWallet() != nil && dst.walletDropdown.SelectedWallet().IsAddressValid(address) {
		dst.destinationAddressEditor.SetError("")
		return address, nil
	}

	return address, fmt.Errorf(values.String(values.StrInvalidAddress))
}

func (dst *destination) validate() bool {
	if dst.isSendToAddress() {
		_, err := dst.validateDestinationAddress()
		// if err equals to nil then the address is valid.
		return err == nil
	}

	if dst.destinationAccount() == nil {
		dst.setError(values.String(values.StrNoValidAccountFound))
		return false
	}

	return true
}

func (dst *destination) setError(errMsg string) {
	if dst.isSendToAddress() {
		dst.destinationAddressEditor.SetError(errMsg)
	}
}

func (dst *destination) clearAddressInput() {
	dst.destinationAddressEditor.SetError("")
	dst.destinationAddressEditor.Editor.SetText("")
}

// isSendToAddress returns the current tab selection status without depending
// on a buffered state.
func (dst *destination) isSendToAddress() bool {
	return dst.accountSwitch.SelectedSegment() == values.StrAddress
}

func (dst *destination) HandleDropdownInteraction(gtx C) {
	dst.accountDropdown.Handle(gtx)
	dst.walletDropdown.Handle(gtx)
}

func (dst *destination) handle(gtx C) {
	if dst.accountSwitch.Changed() {
		dst.addressChanged()
	}

	for {
		event, ok := dst.destinationAddressEditor.Editor.Update(gtx)
		if !ok {
			break
		}

		if gtx.Source.Focused(dst.destinationAddressEditor.Editor) {
			switch event.(type) {
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
