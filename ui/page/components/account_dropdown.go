package components

import (
	"fmt"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type AccountDropdown struct {
	*load.Load
	selectedAccount        *sharedW.Account
	selectedWallet         sharedW.Asset
	dropdown               *cryptomaterial.DropDown
	allAccounts            []*sharedW.Account
	accountChangedCallback func(*sharedW.Account)
	accountIsValid         func(*sharedW.Account) bool
}

func NewAccountDropdown(l *load.Load) *AccountDropdown {
	d := &AccountDropdown{
		Load:            l,
		dropdown:        l.Theme.NewCommonDropDown([]cryptomaterial.DropDownItem{}, nil, cryptomaterial.MatchParent, values.AccountsDropdownGroup, false),
		allAccounts:     make([]*sharedW.Account, 0),
		selectedAccount: nil,
	}
	d.dropdown.BorderColor = &l.Theme.Color.Gray2
	return d
}

func (d *AccountDropdown) Setup(wallet sharedW.Asset) *AccountDropdown {
	if wallet == nil {
		return d
	}
	d.selectedWallet = wallet
	items := []cryptomaterial.DropDownItem{}
	d.allAccounts = make([]*sharedW.Account, 0)
	accounts, err := wallet.GetAccountsRaw()
	if err != nil {
		d.selectedAccount = nil
		d.dropdown.SetItems(items)
		return d
	}

	for _, account := range accounts.Accounts {
		if d.accountIsValid == nil || d.accountIsValid(account) {
			item := cryptomaterial.DropDownItem{
				Text:      fmt.Sprint(account.Number),
				Icon:      d.Theme.Icons.AccountIcon,
				DisplayFn: d.getAccountItemLayout(account),
			}
			items = append(items, item)
			d.allAccounts = append(d.allAccounts, account)
		}
	}
	if len(items) > 0 && !d.selectedIsValid() { // if selected account is not valid, select the first valid account
		id := items[0].Text
		accountNum, err := strconv.Atoi(id)
		if err == nil {
			d.selectedAccount = d.getAccountByNumber(int32(accountNum))
			d.dropdown.SetSelectedValue(id)
			if d.accountChangedCallback != nil {
				d.accountChangedCallback(d.selectedAccount)
			}
		}
	}
	d.dropdown.SetItems(items)
	return d
}

func (d *AccountDropdown) selectedIsValid() bool {
	if d.selectedWallet == nil {
		return false
	}

	if d.selectedAccount == nil {
		return false
	}
	if d.accountIsValid != nil {
		if !d.accountIsValid(d.selectedAccount) {
			return false
		}
	}

	for _, a := range d.allAccounts {
		if a.AccountNumber == d.selectedAccount.AccountNumber {
			return true
		}
	}
	return false
}

func (d *AccountDropdown) ResetAccount() {
	d.selectedAccount = nil
}

func (d *AccountDropdown) AccountValidator(accountIsValid func(*sharedW.Account) bool) *AccountDropdown {
	d.accountIsValid = accountIsValid
	return d
}

func (d *AccountDropdown) getAccountItemLayout(account *sharedW.Account) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						lbl := d.Theme.SemiBoldLabel(account.AccountName)
						lbl.MaxLines = 1
						lbl.TextSize = values.TextSizeTransform(d.IsMobileView(), values.TextSize16)
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return d.Theme.Label(values.TextSizeTransform(d.IsMobileView(), values.TextSize16), account.Balance.Total.String()).Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						spendableText := d.Theme.Label(values.TextSize14, values.String(values.StrLabelSpendable))
						spendableText.Color = d.Theme.Color.GrayText2
						return spendableText.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if d.selectedWallet != nil && d.selectedWallet.IsWatchingOnlyWallet() {
							account.Balance.Spendable = d.selectedWallet.ToAmount(0)
						}
						return d.Theme.Label(values.TextSizeTransform(d.IsMobileView(), values.TextSize14), account.Balance.Spendable.String()).Layout(gtx)
					}),
				)
			}),
		)
	}
}

func (d *AccountDropdown) getAccountByNumber(accountNumber int32) *sharedW.Account {
	for _, account := range d.allAccounts {
		if account.Number == accountNumber {
			return account
		}
	}
	return nil
}

func (d *AccountDropdown) SelectedAccount() *sharedW.Account {
	return d.selectedAccount
}

func (d *AccountDropdown) SetSelectedAccount(account *sharedW.Account) {
	d.selectedAccount = account
	d.dropdown.SetSelectedValue(fmt.Sprint(account.Number))
}

func (d *AccountDropdown) onChanged() {
	accountNumber, err := strconv.Atoi(d.dropdown.Selected())
	if err == nil {
		account := d.getAccountByNumber(int32(accountNumber))
		if account != nil {
			d.selectedAccount = account
			if d.accountChangedCallback != nil {
				d.accountChangedCallback(account)
			}
		}
	}
}

func (d *AccountDropdown) SetChangedCallback(callback func(*sharedW.Account)) *AccountDropdown {
	d.accountChangedCallback = callback
	return d
}

func (d *AccountDropdown) Handle(gtx C) {
	if d.dropdown.Changed(gtx) {
		d.onChanged()
	}
}

func (d *AccountDropdown) Layout(gtx C, title string) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := d.Theme.H6(title)
			lbl.TextSize = values.TextSizeTransform(d.IsMobileView(), values.TextSize16)
			lbl.Font.Weight = font.SemiBold
			return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return d.dropdown.Layout(gtx)
		}),
	)
}

// ListenForTxNotifications listens for transaction and block updates and
// updates the selector modal, if the modal is open at the time of the update.
// The tx update listener MUST be unregistered using ws.StopTxNtfnListener()
// when the page using this WalletAndAccountSelector widget is exited.
func (d *AccountDropdown) ListenForTxNotifications(window app.WindowNavigator) {
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(_ int, _ *sharedW.Transaction) {
			// refresh wallets/Accounts list when new transaction is received

			if d.selectedWallet == nil {
				return
			}
			d.Setup(d.selectedWallet)
			window.Reload()
		},
		OnBlockAttached: func(_ int, _ int32) {
			if d.selectedWallet == nil {
				return
			}
			// refresh wallet and account balance on every new block
			// only if sync is completed.
			if !d.selectedWallet.IsSynced() {
				return
			}
			d.Setup(d.selectedWallet)
			window.Reload()
		},
	}
	if d.selectedWallet == nil {
		return
	}
	err := d.selectedWallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, WalletAndAccountSelectorID)
	if err != nil {
		log.Errorf("WalletAndAccountSelector.ListenForTxNotifications error: %v", err)
	}
}

func (d *AccountDropdown) StopTxNtfnListener() {
	if d.selectedWallet != nil {
		d.selectedWallet.RemoveTxAndBlockNotificationListener(WalletAndAccountSelectorID)
	}
}
