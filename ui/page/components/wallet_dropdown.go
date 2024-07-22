package components

import (
	"fmt"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletAndAccountSelectorID = "WalletAndAccountSelector"

type WalletDropdown struct {
	*load.Load
	selectedWallet        sharedW.Asset
	dropdown              *cryptomaterial.DropDown
	allWallets            []sharedW.Asset
	walletChangedCallback func(sharedW.Asset)
	walletIsValid         func(sharedW.Asset) bool
	isWatchOnlyEnabled    bool
	assetTypes            []utils.AssetType
}

func NewWalletDropdown(l *load.Load, assetType ...utils.AssetType) *WalletDropdown {
	wd := &WalletDropdown{
		Load:     l,
		dropdown: l.Theme.NewCommonDropDown([]cryptomaterial.DropDownItem{}, nil, cryptomaterial.MatchParent, values.WalletsDropdownGroup, false),
	}
	wd.dropdown.BorderColor = &l.Theme.Color.Gray2
	wd.assetTypes = assetType
	return wd
}

func (d *WalletDropdown) Setup() *WalletDropdown {
	d.allWallets = make([]sharedW.Asset, 0)
	wallets := d.AssetsManager.AssetWallets(d.assetTypes...)
	items := []cryptomaterial.DropDownItem{}
	if len(wallets) > 0 {
		for _, w := range wallets {
			if w.IsWatchingOnlyWallet() && !d.isWatchOnlyEnabled || d.walletIsValid != nil && !d.walletIsValid(w) {
				continue
			}
			item := cryptomaterial.DropDownItem{
				Text:      fmt.Sprint(w.GetWalletID()),
				Icon:      d.Theme.AssetIcon(w.GetAssetType()),
				DisplayFn: d.getWalletItemLayout(w),
			}
			items = append(items, item)
			d.allWallets = append(d.allWallets, w)
		}

		if len(items) > 0 && !d.selectedIsValid() {
			id := items[0].Text
			walletID, err := strconv.Atoi(id)
			if err == nil {
				d.selectedWallet = d.getWalletByID(walletID)
				d.dropdown.SetSelectedValue(id)
			}
		}
	}
	d.dropdown.SetItems(items)
	return d
}

func (d *WalletDropdown) selectedIsValid() bool {
	if d.selectedWallet == nil {
		return false
	}

	if d.walletIsValid != nil {
		if !d.walletIsValid(d.selectedWallet) {
			return false
		}
	}

	if d.isWatchOnlyEnabled {
		if !d.selectedWallet.IsWatchingOnlyWallet() {
			return false
		}
	}

	for _, w := range d.allWallets {
		if w.GetWalletID() == d.selectedWallet.GetWalletID() {
			return true
		}
	}
	return false
}

// EnableWatchOnlyWallets enables selection of watchOnly wallets and their accounts.
func (d *WalletDropdown) EnableWatchOnlyWallets(isEnable bool) *WalletDropdown {
	d.isWatchOnlyEnabled = isEnable
	return d
}

func (d *WalletDropdown) SetSelectedWallet(wallet sharedW.Asset) {
	if wallet == nil {
		return
	}
	d.dropdown.SetSelectedValue(fmt.Sprint(wallet.GetWalletID()))
}

func (d *WalletDropdown) walletBalance(wal sharedW.Asset) (totalBalance, spendableBalance int64) {
	accountsResult, err := wal.GetAccountsRaw()
	if err != nil {
		log.Errorf("Error getting accounts: %s", err)
		return 0, 0
	}
	var tBal, sBal int64
	for _, account := range accountsResult.Accounts {
		// If the wallet is watching-only, the spendable balance is zero.
		if wal.IsWatchingOnlyWallet() {
			account.Balance.Spendable = wal.ToAmount(0)
		}
		tBal += account.Balance.Total.ToInt()
		sBal += account.Balance.Spendable.ToInt()
	}
	return tBal, sBal
}

func (d *WalletDropdown) getWalletItemLayout(wallet sharedW.Asset) layout.Widget {
	return func(gtx C) D {
		totalBal, spendable := d.walletBalance(wallet)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						lbl := d.Theme.SemiBoldLabel(wallet.GetWalletName())
						lbl.MaxLines = 1
						lbl.TextSize = values.TextSizeTransform(d.IsMobileView(), values.TextSize16)
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return d.Theme.Label(values.TextSizeTransform(d.IsMobileView(), values.TextSize16), wallet.ToAmount(totalBal).String()).Layout(gtx)
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
						return d.Theme.Label(values.TextSizeTransform(d.IsMobileView(), values.TextSize14), wallet.ToAmount(spendable).String()).Layout(gtx)
					}),
				)
			}),
		)
	}
}

func (d *WalletDropdown) WalletValidator(walletIsValid func(sharedW.Asset) bool) *WalletDropdown {
	d.walletIsValid = walletIsValid
	return d
}

func (d *WalletDropdown) getWalletByID(walletID int) sharedW.Asset {
	for _, wallet := range d.allWallets {
		if wallet.GetWalletID() == walletID {
			return wallet
		}
	}
	return nil
}

func (d *WalletDropdown) onChanged() {
	walletID, err := strconv.Atoi(d.dropdown.Selected())
	if err == nil {
		wallet := d.getWalletByID(walletID)
		if wallet != nil {
			d.selectedWallet = wallet
			if d.walletChangedCallback != nil {
				d.walletChangedCallback(wallet)
			}
		}
	}
}

func (d *WalletDropdown) SelectedWallet() sharedW.Asset {
	return d.selectedWallet
}

func (d *WalletDropdown) SetChangedCallback(callback func(sharedW.Asset)) *WalletDropdown {
	d.walletChangedCallback = callback
	return d
}

func (d *WalletDropdown) Handle(gtx C) {
	if d.dropdown.Changed(gtx) {
		d.onChanged()
	}
}

func (d *WalletDropdown) Layout(gtx C, titleKey string) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := d.Theme.H6(values.String(titleKey))
			lbl.TextSize = values.TextSizeTransform(d.IsMobileView(), values.TextSize16)
			lbl.Font.Weight = font.SemiBold
			return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(d.dropdown.Layout),
	)
}

// ListenForTxNotifications listens for transaction and block updates and
// updates the selector modal, if the modal is open at the time of the update.
// The tx update listener MUST be unregistered using ws.StopTxNtfnListener()
// when the page using this WalletAndAccountSelector widget is exited.
func (d *WalletDropdown) ListenForTxNotifications(window app.WindowNavigator) {
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(_ int, _ *sharedW.Transaction) {
			// refresh wallets/Accounts list when new transaction is received

			if d.selectedWallet == nil {
				return
			}
			d.Setup()
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
			d.Setup()
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

func (d *WalletDropdown) StopTxNtfnListener() {
	if d.selectedWallet != nil {
		d.selectedWallet.RemoveTxAndBlockNotificationListener(WalletAndAccountSelectorID)
	}
}
