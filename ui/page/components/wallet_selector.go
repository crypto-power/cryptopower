package components

import (
	"context"
	"sync"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const WalletSelectorID = "WalletSelector"

type WalletSelector struct {
	*load.Load
	*listeners.TxAndBlockNotificationListener

	selectedWallet *libwallet.Wallet
	callback       func(*libwallet.Wallet)

	openSelectorDialog *cryptomaterial.Clickable
	selectorModal      *WalletSelectorModal

	dialogTitle  string
	totalBalance string
	changed      bool
}

// NewWalletSelector opens up a modal to select the desired wallet. If a
// nil value is passed for selectedWallet, then wallets for all wallets
// are shown, otherwise only wallets for the selectedWallet is shown.
func NewWalletSelector(l *load.Load) *WalletSelector {
	return &WalletSelector{
		Load:               l,
		openSelectorDialog: l.Theme.NewClickable(true),
	}
}

func (as *WalletSelector) Title(title string) *WalletSelector {
	as.dialogTitle = title
	return as
}

func (as *WalletSelector) WalletSelected(callback func(*libwallet.Wallet)) *WalletSelector {
	as.callback = callback
	return as
}

func (as *WalletSelector) Changed() bool {
	changed := as.changed
	as.changed = false
	return changed
}

func (as *WalletSelector) Handle(window app.WindowNavigator) {
	for as.openSelectorDialog.Clicked() {
		as.selectorModal = newWalletSelectorModal(as.Load, as.selectedWallet).
			title(as.dialogTitle).
			walletSelected(func(wallet *libwallet.Wallet) {
				as.changed = true
				as.SetSelectedWallet(wallet)
				as.callback(wallet)
			}).
			onModalExit(func() {
				as.selectorModal = nil
			})
		window.ShowModal(as.selectorModal)
	}
}

func (as *WalletSelector) SetSelectedWallet(wallet *libwallet.Wallet) {
	as.selectedWallet = wallet
}

func (as *WalletSelector) SelectedWallet() *libwallet.Wallet {
	return as.selectedWallet
}

func (as *WalletSelector) Layout(window app.WindowNavigator, gtx C) D {
	as.Handle(window)

	return cryptomaterial.LinearLayout{
		Width:   cryptomaterial.MatchParent,
		Height:  cryptomaterial.WrapContent,
		Padding: layout.UniformInset(values.MarginPadding12),
		Border: cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  as.Theme.Color.Gray2,
			Radius: cryptomaterial.Radius(8),
		},
		Clickable: as.openSelectorDialog,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			walletIcon := as.Theme.Icons.WalletIcon
			inset := layout.Inset{
				Right: values.MarginPadding8,
			}
			return inset.Layout(gtx, func(gtx C) D {
				return walletIcon.Layout24dp(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return as.Theme.Body1("Name").Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return as.Theme.Body1(as.totalBalance).Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						inset := layout.Inset{
							Left: values.MarginPadding15,
						}
						return inset.Layout(gtx, func(gtx C) D {
							ic := cryptomaterial.NewIcon(as.Theme.Icons.DropDownIcon)
							ic.Color = as.Theme.Color.Gray1
							return ic.Layout(gtx, values.MarginPadding20)
						})
					}),
				)
			})
		}),
	)
}

func (as *WalletSelector) ListenForTxNotifications(ctx context.Context, window app.WindowNavigator) {
	if as.TxAndBlockNotificationListener != nil {
		return
	}
	as.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err := as.WL.MultiWallet.AddTxAndBlockNotificationListener(as.TxAndBlockNotificationListener, true, AccoutSelectorID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-as.TxAndBlockNotifChan:
				switch n.Type {
				case listeners.BlockAttached:
					// refresh wallet wallet and balance on every new block
					// only if sync is completed.
					if as.WL.MultiWallet.IsSynced() {
						if as.selectorModal != nil {
							as.selectorModal.setupWallet()
						}
						window.Reload()
					}
				case listeners.NewTransaction:
					// refresh wallets list when new transaction is received
					if as.selectorModal != nil {
						as.selectorModal.setupWallet()
					}
					window.Reload()
				}
			case <-ctx.Done():
				as.WL.MultiWallet.RemoveTxAndBlockNotificationListener(AccoutSelectorID)
				close(as.TxAndBlockNotifChan)
				as.TxAndBlockNotificationListener = nil
				return
			}
		}
	}()
}

type WalletSelectorModal struct {
	*load.Load
	*cryptomaterial.Modal

	walletIsValid func(*libwallet.Wallet) bool
	callback      func(*libwallet.Wallet)
	onExit        func()

	walletInfoButton cryptomaterial.IconButton
	walletsList      layout.List

	currentSelectedWallet *libwallet.Wallet
	wallets               []*selectorWallet // key = wallet id
	eventQueue            event.Queue
	walletMu              sync.Mutex

	dialogTitle string

	isCancelable bool
}

type selectorWallet struct {
	*libwallet.Wallet
	clickable *cryptomaterial.Clickable
}

func newWalletSelectorModal(l *load.Load, currentSelectedWallet *libwallet.Wallet) *WalletSelectorModal {
	asm := &WalletSelectorModal{
		Load:        l,
		Modal:       l.Theme.ModalFloatTitle("WalletSelectorModal"),
		walletsList: layout.List{Axis: layout.Vertical},

		currentSelectedWallet: currentSelectedWallet,
		isCancelable:          true,
	}

	asm.walletInfoButton = l.Theme.IconButton(asm.Theme.Icons.ActionInfo)
	asm.walletInfoButton.Size = values.MarginPadding15
	asm.walletInfoButton.Inset = layout.UniformInset(values.MarginPadding0)

	asm.Modal.ShowScrollbar(true)
	return asm
}

func (asm *WalletSelectorModal) OnResume() {
	asm.setupWallet()
}

func (asm *WalletSelectorModal) setupWallet() {
	wallet := make([]*selectorWallet, 0)
	wallets := asm.WL.SortedWalletList()
	for _, wal := range wallets {
		if !asm.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
			wallet = append(wallet, &selectorWallet{
				Wallet:    wal,
				clickable: asm.Theme.NewClickable(true),
			})
		}
	}
	asm.wallets = wallet
}

func (asm *WalletSelectorModal) SetCancelable(min bool) *WalletSelectorModal {
	asm.isCancelable = min
	return asm
}

func (asm *WalletSelectorModal) Handle() {
	if asm.eventQueue != nil {
		for _, wallet := range asm.wallets {
			for wallet.clickable.Clicked() {
				asm.callback(wallet.Wallet)
				asm.onExit()
				asm.Dismiss()
			}
		}
	}

	if asm.Modal.BackdropClicked(asm.isCancelable) {
		asm.onExit()
		asm.Dismiss()
	}
}

func (asm *WalletSelectorModal) title(title string) *WalletSelectorModal {
	asm.dialogTitle = title
	return asm
}

func (asm *WalletSelectorModal) walletValidator(walletIsValid func(*libwallet.Wallet) bool) *WalletSelectorModal {
	asm.walletIsValid = walletIsValid
	return asm
}

func (asm *WalletSelectorModal) walletSelected(callback func(*libwallet.Wallet)) *WalletSelectorModal {
	asm.callback = callback
	return asm
}

func (asm *WalletSelectorModal) Layout(gtx C) D {
	asm.eventQueue = gtx

	w := []layout.Widget{
		func(gtx C) D {
			title := asm.Theme.H6(asm.dialogTitle)
			title.Color = asm.Theme.Color.Text
			title.Font.Weight = text.SemiBold
			return title.Layout(gtx)
		},
		func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					wallets := asm.wallets
					return asm.walletsList.Layout(gtx, len(wallets), func(gtx C, aindex int) D {
						return asm.walletWalletLayout(gtx, wallets[aindex])
					})
				}),
				layout.Stacked(func(gtx C) D {
					if false { //TODO
						inset := layout.Inset{
							Top:  values.MarginPadding20,
							Left: values.MarginPaddingMinus75,
						}
						return inset.Layout(gtx, func(gtx C) D {
							// return page.walletInfoPopup(gtx)
							return D{}
						})
					}
					return D{}
				}),
			)
		},
	}

	return asm.Modal.Layout(gtx, w)
}

func (asm *WalletSelectorModal) walletWalletLayout(gtx C, wallet *selectorWallet) D {
	walletIcon := asm.Theme.Icons.WalletIcon

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Margin:    layout.Inset{Bottom: values.MarginPadding4},
		Padding:   layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		Clickable: wallet.clickable,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Flexed(0.1, func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding18,
			}.Layout(gtx, func(gtx C) D {
				return walletIcon.Layout24dp(gtx)
			})
		}),
		layout.Flexed(0.8, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					acct := asm.Theme.Label(values.TextSize18, wallet.Name)
					acct.Color = asm.Theme.Color.Text
					return EndToEndRow(gtx, acct.Layout, func(gtx C) D {
						return layout.Dimensions{}
					})
				}),
				layout.Rigid(func(gtx C) D {
					spendable := asm.Theme.Label(values.TextSize14, values.String(values.StrLabelSpendable))
					spendable.Color = asm.Theme.Color.GrayText2
					spendableBal := asm.Theme.Label(values.TextSize14, dcrutil.Amount(1234567).String())
					spendableBal.Color = asm.Theme.Color.GrayText2
					return EndToEndRow(gtx, spendable.Layout, spendableBal.Layout)
				}),
			)
		}),

		layout.Flexed(0.1, func(gtx C) D {
			return D{}
		}),
	)
}

func (asm *WalletSelectorModal) onModalExit(exit func()) *WalletSelectorModal {
	asm.onExit = exit
	return asm
}

func (asm *WalletSelectorModal) OnDismiss() {}
