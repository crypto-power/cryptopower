package governance

import (
	"errors"
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type WalletSelector struct {
	*load.Load
	assetsManager *libwallet.AssetsManager
	dialogTitle   string

	walletIsValid func(sharedW.Asset) bool
	callback      func(sharedW.Asset)

	openSelectorDialog *cryptomaterial.Clickable

	wallets        []sharedW.Asset
	selectedWallet sharedW.Asset
	totalBalance   string
}

// TODO: merge this into the account selector modal.
func NewDCRWalletSelector(l *load.Load) *WalletSelector {
	return &WalletSelector{
		Load:               l,
		assetsManager:      l.AssetsManager,
		walletIsValid:      func(sharedW.Asset) bool { return true },
		openSelectorDialog: l.Theme.NewClickable(true),

		wallets: l.AssetsManager.AssetWallets(utils.DCRWalletAsset),
	}
}

func (as *WalletSelector) Title(title string) *WalletSelector {
	as.dialogTitle = title
	return as
}

func (as *WalletSelector) WalletValidator(walletIsValid func(sharedW.Asset) bool) *WalletSelector {
	as.walletIsValid = walletIsValid
	return as
}

func (as *WalletSelector) WalletSelected(callback func(sharedW.Asset)) *WalletSelector {
	as.callback = callback
	return as
}

func (as *WalletSelector) Handle(gtx C, window app.WindowNavigator) {
	if as.openSelectorDialog.Clicked(gtx) {
		walletSelectorModal := newWalletSelectorModal(as.Load, as.selectedWallet).
			title(as.dialogTitle).
			accountValidator(as.walletIsValid).
			accountSelected(func(wallet sharedW.Asset) {
				as.selectedWallet = wallet
				as.setupSelectedWallet(wallet)
				as.callback(wallet)
			})
		window.ShowModal(walletSelectorModal)
	}
}

func (as *WalletSelector) SelectFirstValidWallet() error {
	if as.selectedWallet != nil && as.walletIsValid(as.selectedWallet) {
		// no need to select account
		return nil
	}

	for _, wal := range as.wallets {
		if as.walletIsValid(wal) {
			as.selectedWallet = wal
			as.setupSelectedWallet(wal)
			as.callback(wal)
			return nil
		}
	}

	return errors.New(values.String(values.StrnoValidWalletFound))
}

func (as *WalletSelector) setupSelectedWallet(wallet sharedW.Asset) {
	_, walletTotalBalance, err := sharedW.Balances(wallet)
	if err != nil {
		fmt.Println(err)
		return
	}

	as.totalBalance = walletTotalBalance.String()
}

func (as *WalletSelector) SelectedWallet() sharedW.Asset {
	return as.selectedWallet
}

func (as *WalletSelector) Layout(gtx layout.Context, window app.WindowNavigator) layout.Dimensions {
	as.Handle(gtx, window)

	border := widget.Border{
		Color:        as.Theme.Color.Gray2,
		CornerRadius: values.MarginPadding8,
		Width:        values.MarginPadding2,
	}

	return border.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(values.MarginPadding12).Layout(gtx, func(gtx C) D {
			return as.openSelectorDialog.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						accountIcon := as.Theme.Icons.AccountIcon
						return layout.Inset{
							Right: values.MarginPadding8,
						}.Layout(gtx, accountIcon.Layout24dp)
					}),
					layout.Rigid(as.Theme.Body1(as.selectedWallet.GetWalletName()).Layout),
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(as.Theme.Body1(as.totalBalance).Layout),
								layout.Rigid(func(gtx C) D {
									inset := layout.Inset{
										Left: values.MarginPadding15,
									}
									return inset.Layout(gtx, func(gtx C) D {
										ic := cryptomaterial.NewIcon(as.Theme.Icons.DropDownIcon)
										return ic.Layout(gtx, values.MarginPadding20)
									})
								}),
							)
						})
					}),
				)
			})
		})
	})
}

type WalletSelectorModal struct {
	*load.Load
	*cryptomaterial.Modal

	dialogTitle string

	isCancelable bool

	walletIsValid func(sharedW.Asset) bool
	callback      func(sharedW.Asset)

	walletsList *cryptomaterial.ClickableList

	currentSelectedWallet sharedW.Asset
	filteredWallets       []sharedW.Asset
}

func newWalletSelectorModal(l *load.Load, currentSelectedWallet sharedW.Asset) *WalletSelectorModal {
	asm := &WalletSelectorModal{
		Load:        l,
		Modal:       l.Theme.ModalFloatTitle("WalletSelectorModal", l.IsMobileView()),
		walletsList: l.Theme.NewClickableList(layout.Vertical),

		currentSelectedWallet: currentSelectedWallet,
		isCancelable:          true,
	}

	return asm
}

func (asm *WalletSelectorModal) OnResume() {
	wallets := asm.AssetsManager.AssetWallets(asm.currentSelectedWallet.GetAssetType())

	validWallets := make([]sharedW.Asset, 0)
	for _, wal := range wallets {
		if asm.walletIsValid(wal) {
			validWallets = append(validWallets, wal)
		}
	}

	asm.filteredWallets = validWallets
}

func (asm *WalletSelectorModal) Handle(gtx C) {
	if clicked, index := asm.walletsList.ItemClicked(); clicked {
		asm.callback(asm.filteredWallets[index])
		asm.Dismiss()
	}

	if asm.Modal.BackdropClicked(gtx, asm.isCancelable) {
		asm.Dismiss()
	}
}

func (asm *WalletSelectorModal) title(title string) *WalletSelectorModal {
	asm.dialogTitle = title
	return asm
}

func (asm *WalletSelectorModal) accountValidator(walletIsValid func(sharedW.Asset) bool) *WalletSelectorModal {
	asm.walletIsValid = walletIsValid
	return asm
}

func (asm *WalletSelectorModal) accountSelected(callback func(sharedW.Asset)) *WalletSelectorModal {
	asm.callback = callback
	return asm
}

func (asm *WalletSelectorModal) OnDismiss() {
}

func (asm *WalletSelectorModal) Layout(gtx layout.Context) layout.Dimensions {
	w := []layout.Widget{
		func(gtx C) D {
			title := asm.Theme.H6(asm.dialogTitle)
			title.Color = asm.Theme.Color.Text
			title.Font.Weight = font.SemiBold
			return title.Layout(gtx)
		},
		func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return asm.walletsList.Layout(gtx, len(asm.filteredWallets), func(gtx C, windex int) D {
						wal := asm.filteredWallets[windex]
						return asm.walletAccountLayout(gtx, wal)
					})
				}),
				layout.Stacked(func(gtx C) D {
					if false { // TODO
						inset := layout.Inset{
							Top:  values.MarginPadding20,
							Left: values.MarginPaddingMinus75,
						}
						return inset.Layout(gtx, func(_ C) D {
							// return page.walletInfoPopup(gtx)
							return layout.Dimensions{}
						})
					}
					return layout.Dimensions{}
				}),
			)
		},
	}

	return asm.Modal.Layout(gtx, w)
}

func (asm *WalletSelectorModal) walletAccountLayout(gtx layout.Context, wallet sharedW.Asset) layout.Dimensions {
	walletSpendableBalance, walletTotalBalance, _ := sharedW.Balances(wallet)

	return layout.Inset{
		Bottom: values.MarginPadding20,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(0.1, func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding18,
						}.Layout(gtx, func(gtx C) D {
							accountIcon := asm.Theme.Icons.AccountIcon
							return accountIcon.Layout24dp(gtx)
						})
					}),
					layout.Flexed(0.8, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								acct := asm.Theme.Label(values.TextSize18, wallet.GetWalletName())
								acct.Color = asm.Theme.Color.Text
								return components.EndToEndRow(gtx, acct.Layout, func(gtx C) D {
									return components.LayoutBalanceWithUnit(gtx, asm.Load, walletTotalBalance.String())
								})
							}),
							layout.Rigid(func(gtx C) D {
								spendable := asm.Theme.Label(values.TextSize14, values.String(values.StrLabelSpendable))
								spendable.Color = asm.Theme.Color.GrayText2
								// TODO
								spendableBal := asm.Theme.Label(values.TextSize14, walletSpendableBalance.String())
								spendableBal.Color = asm.Theme.Color.GrayText2
								return components.EndToEndRow(gtx, spendable.Layout, spendableBal.Layout)
							}),
						)
					}),

					layout.Flexed(0.1, func(gtx C) D {
						inset := layout.Inset{
							Right: values.MarginPadding10,
							Top:   values.MarginPadding10,
						}
						sections := func(gtx layout.Context) layout.Dimensions {
							return layout.E.Layout(gtx, func(gtx C) D {
								return inset.Layout(gtx, func(gtx C) D {
									ic := cryptomaterial.NewIcon(asm.Theme.Icons.NavigationCheck)
									return ic.Layout(gtx, values.MarginPadding20)
								})
							})
						}

						if wallet.GetWalletID() == asm.currentSelectedWallet.GetWalletID() {
							return sections(gtx)
						}
						return layout.Dimensions{}
					}),
				)
			}),
		)
	})
}
