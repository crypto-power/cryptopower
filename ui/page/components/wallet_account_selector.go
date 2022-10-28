package components

import (
	"context"
	"errors"

	"gioui.org/io/event"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/app"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/renderers"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const WalletAndAccountSelectorID = "WalletAndAccountSelector"

type WalletAndAccountSelector struct {
	*listeners.TxAndBlockNotificationListener
	openSelectorDialog *cryptomaterial.Clickable
	*selectorModal

	totalBalance string
	changed      bool
}

type selectorModal struct {
	*load.Load
	*cryptomaterial.Modal

	selectedWallet   *load.WalletMapping
	selectedAccount  *sharedW.Account
	accountCallback  func(*sharedW.Account)
	walletCallback   func(*load.WalletMapping)
	accountIsValid   func(*sharedW.Account) bool
	accountSelector  bool
	infoActionText   string
	dialogTitle      string
	onWalletClicked  func(*load.WalletMapping)
	onAccountClicked func(*sharedW.Account)
	walletsList      layout.List
	selectorItems    []*SelectorItem // A SelectorItem can either be a wallet or account
	eventQueue       event.Queue
	isCancelable     bool
	infoButton       cryptomaterial.IconButton
	infoModalOpen    bool
	infoBackdrop     *widget.Clickable
}

// NewWalletAndAccountSelector creates a wallet selector component.
// It opens a modal to select a desired wallet or a desired account.
func NewWalletAndAccountSelector(l *load.Load) *WalletAndAccountSelector {
	ws := &WalletAndAccountSelector{
		openSelectorDialog: l.Theme.NewClickable(true),
	}

	ws.selectorModal = newSelectorModal(l).
		walletClicked(func(wallet *load.WalletMapping) {
			if ws.selectedWallet.GetWalletID() == wallet.GetWalletID() {
				ws.changed = true
			}
			ws.SetSelectedWallet(wallet)
			if ws.walletCallback != nil {
				ws.walletCallback(wallet)
			}
		}).
		accountCliked(func(account *sharedW.Account) {
			if ws.selectedAccount.Number != account.Number {
				ws.changed = true
			}
			ws.SetSelectedAccount(account)
			if ws.accountCallback != nil {
				ws.accountCallback(account)
			}
		})

	return ws
}

// SelectedAccount returns the currently selected account.
func (ws *WalletAndAccountSelector) SelectedAccount() *sharedW.Account {
	return ws.selectedAccount
}

// AccountValidator validates an account according to the rules defined to determine a valid a account.
func (ws *WalletAndAccountSelector) AccountValidator(accountIsValid func(*sharedW.Account) bool) *WalletAndAccountSelector {
	ws.accountIsValid = accountIsValid
	return ws
}

// SetActionInfoText sets the text that is shown when the info action icon of the selector
// modal is is clicked. The {text} is rendered using a html renderer. So HTML text can be passed in.
func (ws *WalletAndAccountSelector) SetActionInfoText(text string) *WalletAndAccountSelector {
	ws.infoActionText = text
	return ws
}

// SelectFirstValidAccount transforms this widget into an Account selector and selects the first valid account from the
// the wallet passed to this method.
func (ws *WalletAndAccountSelector) SelectFirstValidAccount(wallet *load.WalletMapping) error {
	if !ws.accountSelector {
		ws.accountSelector = true
	}
	ws.SetSelectedWallet(wallet)

	accounts, err := wallet.GetAccountsRaw()
	if err != nil {
		return err
	}

	for _, account := range accounts.Accounts {
		if ws.accountIsValid(account) {
			ws.SetSelectedAccount(account)
			if ws.accountCallback != nil {
				ws.accountCallback(account)
			}
			return nil
		}
	}

	return errors.New(values.String(values.StrNoValidAccountFound))
}

func (ws *WalletAndAccountSelector) SetSelectedAccount(account *sharedW.Account) {
	ws.selectedAccount = account
	ws.totalBalance = account.Balance.Total.String()
}

func (ws *WalletAndAccountSelector) Clickable() *cryptomaterial.Clickable {
	return ws.openSelectorDialog
}

func (ws *WalletAndAccountSelector) Title(title string) *WalletAndAccountSelector {
	ws.dialogTitle = title
	return ws
}

func (ws *WalletAndAccountSelector) WalletSelected(callback func(*load.WalletMapping)) *WalletAndAccountSelector {
	ws.walletCallback = callback
	return ws
}

func (ws *WalletAndAccountSelector) AccountSelected(callback func(*sharedW.Account)) *WalletAndAccountSelector {
	ws.accountCallback = callback
	return ws
}

func (ws *WalletAndAccountSelector) Changed() bool {
	changed := ws.changed
	ws.changed = false
	return changed
}

func (ws *WalletAndAccountSelector) Handle(window app.WindowNavigator) {
	for ws.openSelectorDialog.Clicked() {
		ws.title(ws.dialogTitle).accountValidator(ws.accountIsValid)
		window.ShowModal(ws.selectorModal)
	}
}

func (ws *WalletAndAccountSelector) SetSelectedWallet(wallet *load.WalletMapping) {
	ws.selectedWallet = wallet
}

func (ws *WalletAndAccountSelector) SelectedWallet() *load.WalletMapping {
	return ws.selectedWallet
}

func (ws *WalletAndAccountSelector) Layout(window app.WindowNavigator, gtx C) D {
	ws.Handle(window)

	return cryptomaterial.LinearLayout{
		Width:   cryptomaterial.MatchParent,
		Height:  cryptomaterial.WrapContent,
		Padding: layout.UniformInset(values.MarginPadding12),
		Border: cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  ws.Theme.Color.Gray2,
			Radius: cryptomaterial.Radius(8),
		},
		Clickable: ws.Clickable(),
	}.Layout(gtx,
		layout.Rigid(ws.logoWallet),
		layout.Rigid(func(gtx C) D {
			if ws.accountSelector {
				if ws.selectedAccount == nil {
					return ws.Theme.Body1("").Layout(gtx)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(ws.Theme.Body1(ws.SelectedAccount().Name).Layout),
				)
			}
			return ws.Theme.Body1(ws.SelectedWallet().GetWalletName()).Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if ws.accountSelector {
							if ws.selectedAccount == nil {
								return ws.Theme.Body1(string(ws.selectedWallet.GetAssetType())).Layout(gtx)
							}
							return ws.Theme.Body1(ws.totalBalance).Layout(gtx)
						}
						selectWallet := ws.SelectedWallet()
						totalBal, _ := walletBalance(selectWallet)
						return ws.Theme.Body1(selectWallet.ToAmount(totalBal).String()).Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						inset := layout.Inset{
							Left: values.MarginPadding15,
						}
						return inset.Layout(gtx, func(gtx C) D {
							ic := cryptomaterial.NewIcon(ws.Theme.Icons.DropDownIcon)
							ic.Color = ws.Theme.Color.Gray1
							return ic.Layout(gtx, values.MarginPadding20)
						})
					}),
				)
			})
		}),
	)
}

func (ws *WalletAndAccountSelector) logoWallet(gtx C) D {
	walletIcon := ws.Theme.Icons.DecredLogo
	if ws.selectedWallet.GetAssetType() == utils.BTCWalletAsset {
		walletIcon = ws.Theme.Icons.BTC
	}
	if ws.accountSelector {
		walletIcon = ws.Theme.Icons.AccountIcon
	}
	inset := layout.Inset{
		Right: values.MarginPadding8,
	}
	return inset.Layout(gtx, walletIcon.Layout24dp)
}

func (ws *WalletAndAccountSelector) ListenForTxNotifications(ctx context.Context, window app.WindowNavigator) {
	if ws.TxAndBlockNotificationListener != nil {
		return
	}

	ws.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err := ws.selectedWallet.AddTxAndBlockNotificationListener(ws.TxAndBlockNotificationListener, true, WalletAndAccountSelectorID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-ws.TxAndBlockNotifChan:
				switch n.Type {
				case listeners.BlockAttached:
					// refresh wallet and account balance on every new block
					// only if sync is completed.
					if ws.selectedWallet.IsSynced() {
						if ws.selectorModal != nil {
							if ws.accountSelector {
								ws.selectorModal.setupAccounts(ws.selectedWallet)
							} else {
								ws.selectorModal.setupWallet()
							}
							window.Reload()
						}

					}
				case listeners.NewTransaction:
					// refresh wallets/Accounts list when new transaction is received
					if ws.selectorModal != nil {
						if ws.accountSelector {
							ws.selectorModal.setupAccounts(ws.selectedWallet)
						} else {
							ws.selectorModal.setupWallet()
						}
						window.Reload()
					}

				}
			case <-ctx.Done():
				ws.selectedWallet.RemoveTxAndBlockNotificationListener(WalletAndAccountSelectorID)
				close(ws.TxAndBlockNotifChan)
				ws.TxAndBlockNotificationListener = nil
				return
			}
		}
	}()
}

// SelectorItem models a wallet or an account a long with it's clickable.
type SelectorItem struct {
	item      interface{} // Item can either be a wallet or an account.
	clickable *cryptomaterial.Clickable
}

func newSelectorModal(l *load.Load) *selectorModal {
	sm := &selectorModal{
		Load:         l,
		Modal:        l.Theme.ModalFloatTitle("SelectorModal"),
		walletsList:  layout.List{Axis: layout.Vertical},
		isCancelable: true,
		infoBackdrop: new(widget.Clickable),
	}

	sm.infoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	sm.infoButton.Size = values.MarginPadding14
	sm.infoButton.Inset = layout.UniformInset(values.MarginPadding4)

	sm.accountIsValid = func(*sharedW.Account) bool { return false }
	sm.selectedWallet = &load.WalletMapping{
		Asset: l.WL.SelectedWallet.Wallet,
	} // Set the default wallet to wallet loaded by cryptopower.
	sm.accountSelector = false

	sm.Modal.ShowScrollbar(true)
	return sm
}

func (sm *selectorModal) OnResume() {
	if sm.accountSelector {
		sm.setupAccounts(sm.selectedWallet)
		return
	}
	sm.setupWallet()
}

func (sm *selectorModal) setupWallet() {
	selectorItems := make([]*SelectorItem, 0)
	wallets := sm.WL.SortedWalletList()
	for _, wal := range wallets {
		if !wal.IsWatchingOnlyWallet() {
			selectorItems = append(selectorItems, &SelectorItem{
				item: &load.WalletMapping{
					Asset: wal,
				},
				clickable: sm.Theme.NewClickable(true),
			})
		}
	}
	sm.selectorItems = selectorItems
}

func (sm *selectorModal) setupAccounts(wal sharedW.Asset) {
	selectorItems := make([]*SelectorItem, 0)
	if !wal.IsWatchingOnlyWallet() {
		accountsResult, err := wal.GetAccountsRaw()
		if err != nil {
			log.Errorf("Error getting accounts:", err)
			return
		}

		for _, account := range accountsResult.Accounts {
			if sm.accountIsValid(account) {
				selectorItems = append(selectorItems, &SelectorItem{
					item:      account,
					clickable: sm.Theme.NewClickable(true),
				})
			}
		}
	}
	sm.selectorItems = selectorItems
}

func (sm *selectorModal) accountValidator(accountIsValid func(*sharedW.Account) bool) *selectorModal {
	sm.accountIsValid = accountIsValid
	return sm
}

func (sm *selectorModal) Handle() {
	if sm.eventQueue != nil {
		for _, selectorItem := range sm.selectorItems {
			for selectorItem.clickable.Clicked() {
				switch item := selectorItem.item.(type) {
				case *sharedW.Account:
					if sm.onAccountClicked != nil {
						sm.onAccountClicked(item)
					}
				case *load.WalletMapping:
					if sm.onWalletClicked != nil {
						sm.onWalletClicked(item)
					}
				}
				sm.Dismiss()
			}
		}

		if sm.infoBackdrop.Clicked() {
			sm.infoModalOpen = false
		}

		if sm.infoButton.IconButtonStyle.Button.Clicked() {
			sm.infoModalOpen = !sm.infoModalOpen
		}
	}

	if sm.Modal.BackdropClicked(sm.isCancelable) {
		sm.Dismiss()
	}
}

func (sm *selectorModal) title(title string) *selectorModal {
	sm.dialogTitle = title
	return sm
}

func (sm *selectorModal) walletClicked(callback func(*load.WalletMapping)) *selectorModal {
	sm.onWalletClicked = callback
	return sm
}

func (sm *selectorModal) accountCliked(callback func(*sharedW.Account)) *selectorModal {
	sm.onAccountClicked = callback
	return sm
}

func (sm *selectorModal) Layout(gtx C) D {
	sm.eventQueue = gtx
	sm.infoBackdropLayout(gtx)

	w := []layout.Widget{
		func(gtx C) D {
			title := sm.Theme.H6(sm.dialogTitle)
			title.Color = sm.Theme.Color.Text
			title.Font.Weight = text.SemiBold
			return layout.Inset{
				Top: values.MarginPaddingMinus15,
			}.Layout(gtx, title.Layout)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if sm.accountSelector {
						inset := layout.Inset{
							Top: values.MarginPadding0,
						}
						return inset.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											if sm.infoModalOpen {
												m := op.Record(gtx.Ops)
												layout.Inset{Top: values.MarginPadding30}.Layout(gtx, func(gtx C) D {
													card := sm.Theme.Card()
													card.Color = sm.Theme.Color.Surface
													return card.Layout(gtx, func(gtx C) D {
														return layout.UniformInset(values.MarginPadding12).Layout(gtx, renderers.RenderHTML(sm.infoActionText, sm.Theme).Layout)
													})
												})
												op.Defer(gtx.Ops, m.Stop())
											}

											return D{}
										}),
										layout.Rigid(func(gtx C) D {
											if sm.infoActionText != "" {
												return sm.infoButton.Layout(gtx)
											}
											return D{}
										}),
									)
								}),
							)
						})
					}
					return D{}
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Stack{Alignment: layout.NW}.Layout(gtx,
						layout.Expanded(func(gtx C) D {
							return sm.walletsList.Layout(gtx, len(sm.selectorItems), func(gtx C, aindex int) D {
								return sm.modalListItemLayout(gtx, sm.selectorItems[aindex])
							})
						}),
					)

				}),
			)
		},
	}

	return sm.Modal.Layout(gtx, w)
}

// infoBackdropLayout draws background overlay when the confirmation modal action button is clicked.
func (sm *selectorModal) infoBackdropLayout(gtx C) {
	if sm.infoModalOpen {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		m := op.Record(gtx.Ops)
		sm.infoBackdrop.Layout(gtx, func(gtx C) D {
			semantic.Button.Add(gtx.Ops)
			return D{Size: gtx.Constraints.Min}
		})
		op.Defer(gtx.Ops, m.Stop())
	}
}

func walletBalance(wal sharedW.Asset) (totalBalance, spendableBalance int64) {
	accountsResult, _ := wal.GetAccountsRaw()
	var tBal, sBal int64
	for _, account := range accountsResult.Accounts {
		tBal += account.Balance.Total.ToInt()
		sBal += account.Balance.Spendable.ToInt()
	}
	return tBal, sBal
}

func (sm *selectorModal) modalListItemLayout(gtx C, selectorItem *SelectorItem) D {
	accountIcon := sm.Theme.Icons.AccountIcon

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Margin:    layout.Inset{Bottom: values.MarginPadding4},
		Padding:   layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		Clickable: selectorItem.clickable,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Flexed(0.1, func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding18,
			}.Layout(gtx, accountIcon.Layout16dp)
		}),
		layout.Flexed(0.8, func(gtx C) D {
			var name, totalBal, spendableBal string
			switch t := selectorItem.item.(type) {
			case *sharedW.Account:
				totalBal = t.Balance.Total.String()
				spendableBal = t.Balance.Spendable.String()
				name = t.Name
			case *load.WalletMapping:
				tb, sb := walletBalance(t)
				totalBal = t.ToAmount(tb).String()
				spendableBal = t.ToAmount(sb).String()
				name = t.GetWalletName()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					acct := sm.Theme.Label(values.TextSize18, name)
					acct.Color = sm.Theme.Color.Text
					acct.Font.Weight = text.Normal
					return EndToEndRow(gtx, acct.Layout, func(gtx C) D {
						return LayoutBalance(gtx, sm.Load, totalBal)
					})
				}),
				layout.Rigid(func(gtx C) D {
					spendableText := sm.Theme.Label(values.TextSize14, values.String(values.StrLabelSpendable))
					spendableText.Color = sm.Theme.Color.GrayText2
					spendableLabel := sm.Theme.Label(values.TextSize14, spendableBal)
					spendableLabel.Color = sm.Theme.Color.GrayText2
					return EndToEndRow(gtx, spendableText.Layout, spendableLabel.Layout)
				}),
			)
		}),

		layout.Flexed(0.1, func(gtx C) D {
			inset := layout.Inset{
				Top:  values.MarginPadding10,
				Left: values.MarginPadding10,
			}
			sections := func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return inset.Layout(gtx, func(gtx C) D {
						ic := cryptomaterial.NewIcon(sm.Theme.Icons.NavigationCheck)
						ic.Color = sm.Theme.Color.Gray1
						return ic.Layout(gtx, values.MarginPadding20)
					})
				})
			}
			switch t := selectorItem.item.(type) {
			case *sharedW.Account:
				if t.Number == sm.selectedAccount.Number {
					return sections(gtx)
				}
			case sharedW.Asset:
				if t.GetWalletID() == sm.selectedWallet.GetWalletID() {
					return sections(gtx)
				}
			}
			return D{}
		}),
	)
}

func (sm *selectorModal) OnDismiss() {}
