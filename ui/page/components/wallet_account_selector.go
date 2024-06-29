package components

import (
	"errors"

	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/renderers"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletAndAccountSelectorID = "WalletAndAccountSelector"

type WalletAndAccountSelector struct {
	openSelectorDialog *cryptomaterial.Clickable
	*selectorModal

	totalBalance string
	changed      bool

	errorLabel cryptomaterial.Label
	HideLogo   bool

	hideBalance     bool
	isLeftAlignment bool
	isHaveBorder    bool
}

type selectorModal struct {
	*load.Load
	*cryptomaterial.Modal

	selectedWallet     sharedW.Asset
	selectedAccount    *sharedW.Account
	accountCallback    func(*sharedW.Account)
	walletCallback     func(sharedW.Asset)
	accountIsValid     func(*sharedW.Account) bool
	walletIsValid      func(sharedW.Asset) bool
	accountSelector    bool
	infoActionText     string
	dialogTitle        string
	onWalletClicked    func(sharedW.Asset)
	onAccountClicked   func(*sharedW.Account)
	walletsList        layout.List
	selectorItems      []*SelectorItem // A SelectorItem can either be a wallet or account
	assetType          []utils.AssetType
	eventSource        input.Source
	isCancelable       bool
	infoButton         cryptomaterial.IconButton
	infoModalOpen      bool
	infoBackdrop       *widget.Clickable
	isWatchOnlyEnabled bool
}

// NewWalletAndAccountSelector creates a wallet selector component.
// It opens a modal to select a desired wallet or a desired account.
func NewWalletAndAccountSelector(l *load.Load, assetType ...utils.AssetType) *WalletAndAccountSelector {
	ws := &WalletAndAccountSelector{
		openSelectorDialog: l.Theme.NewClickable(true),
		errorLabel:         l.Theme.ErrorLabel(""),
		hideBalance:        false,
		isLeftAlignment:    false,
		isHaveBorder:       true,
	}

	ws.selectorModal = newSelectorModal(l, assetType...).
		walletClicked(func(wallet sharedW.Asset) {
			if ws.selectedWallet.GetWalletID() != wallet.GetWalletID() {
				ws.changed = true
			}
			ws.SetSelectedWallet(wallet)
			if ws.walletCallback != nil {
				ws.walletCallback(wallet)
			}
		}).
		accountClicked(func(account *sharedW.Account) {
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

// EnableWatchOnlyWallets enables selection of watchOnly wallets and their accounts.
func (ws *WalletAndAccountSelector) EnableWatchOnlyWallets(isEnable bool) *WalletAndAccountSelector {
	ws.isWatchOnlyEnabled = isEnable
	return ws
}

// AccountValidator validates an account according to the rules defined to determine a valid a account.
func (ws *WalletAndAccountSelector) AccountValidator(accountIsValid func(*sharedW.Account) bool) *WalletAndAccountSelector {
	ws.accountIsValid = accountIsValid
	return ws
}

// WalletValidator validates a wallet according to the rules defined to
// determine a valid a wallet.
func (ws *WalletAndAccountSelector) WalletValidator(walletIsValid func(sharedW.Asset) bool) *WalletAndAccountSelector {
	ws.walletIsValid = walletIsValid
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
func (ws *WalletAndAccountSelector) SelectFirstValidAccount(wallet sharedW.Asset) error {
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

	ws.ResetAccount()
	return errors.New(values.String(values.StrNoValidAccountFound))
}

// SetSelectedAsset sets the specified assetType and returns true if the asset
// has at least one wallet.
func (ws *WalletAndAccountSelector) SetSelectedAsset(assetType ...utils.AssetType) bool {
	ws.accountSelector = false
	ws.selectorModal.setupWallet(assetType[0])
	if len(ws.selectorItems) == 0 {
		return false
	}

	ws.assetType = assetType
	ws.selectedWallet = ws.selectorItems[0].item.(sharedW.Asset)
	return true
}

func (ws *WalletAndAccountSelector) SelectedAsset() utils.AssetType {
	return ws.assetType[0]
}

func (ws *WalletAndAccountSelector) SelectAccount(wallet sharedW.Asset, accountNumber int32) error {
	ws.accountSelector = true
	ws.SetSelectedWallet(wallet)

	account, err := wallet.GetAccount(accountNumber)
	if err != nil {
		return err
	}

	if !ws.accountIsValid(account) {
		ws.ResetAccount()
		return errors.New(values.String(values.StrNoValidAccountFound))
	}

	ws.SetSelectedAccount(account)
	if ws.accountCallback != nil {
		ws.accountCallback(account)
	}

	return nil
}

func (ws *WalletAndAccountSelector) ResetAccount() {
	ws.selectedAccount = nil
	ws.totalBalance = ""
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

func (ws *WalletAndAccountSelector) WalletSelected(callback func(sharedW.Asset)) *WalletAndAccountSelector {
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

func (ws *WalletAndAccountSelector) Handle(gtx C, window app.WindowNavigator) {
	for ws.openSelectorDialog.Clicked(gtx) {
		ws.title(ws.dialogTitle).accountValidator(ws.accountIsValid)
		window.ShowModal(ws.selectorModal)
	}
}

func (ws *WalletAndAccountSelector) SetSelectedWallet(wallet sharedW.Asset) {
	ws.selectedWallet = wallet
}

func (ws *WalletAndAccountSelector) SelectedWallet() sharedW.Asset {
	return ws.selectedWallet
}

func (ws *WalletAndAccountSelector) SetError(errMsg string) {
	ws.errorLabel.Text = errMsg
}

func (ws *WalletAndAccountSelector) SetHideBalance(isHide bool) {
	ws.hideBalance = isHide
}

func (ws *WalletAndAccountSelector) SetLeftAlignment(isLeft bool) {
	ws.isLeftAlignment = isLeft
}

func (ws *WalletAndAccountSelector) SetBorder(isHaveBorder bool) {
	ws.isHaveBorder = isHaveBorder
}

func (ws *WalletAndAccountSelector) Layout(window app.WindowNavigator, gtx C) D {
	ws.Handle(gtx, window)

	borderColor := ws.Theme.Color.Gray2
	if ws.errorLabel.Text != "" {
		borderColor = ws.errorLabel.Color
	}

	border := cryptomaterial.Border{}
	if ws.isHaveBorder {
		border = cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  borderColor,
			Radius: cryptomaterial.Radius(8),
		}
	}

	textSize16 := values.TextSizeTransform(ws.IsMobileView(), values.TextSize16)
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:     cryptomaterial.MatchParent,
						Height:    cryptomaterial.WrapContent,
						Padding:   layout.UniformInset(values.MarginPadding12),
						Border:    border,
						Clickable: ws.Clickable(),
					}.Layout(gtx,
						layout.Rigid(ws.setWalletLogo),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Max.X = gtx.Constraints.Max.X / 2
							if ws.accountSelector {
								if ws.selectedAccount == nil {
									return ws.Theme.Body1("").Layout(gtx)
								}
								lbl := ws.Theme.Body1(ws.SelectedAccount().Name)
								lbl.MaxLines = 1
								lbl.TextSize = textSize16
								return lbl.Layout(gtx)
							}

							lbl := ws.Theme.SemiBoldLabel(ws.SelectedWallet().GetWalletName())
							lbl.MaxLines = 1
							lbl.TextSize = textSize16
							return lbl.Layout(gtx)
						}),
						layout.Flexed(1, func(gtx C) D {
							dim := func(gtx C) D {
								return layout.Flex{}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										if ws.hideBalance {
											return D{}
										}
										if ws.accountSelector {
											if ws.selectedAccount == nil {
												return ws.Theme.Label(textSize16, string(ws.selectedWallet.GetAssetType())).Layout(gtx)
											}
											return ws.Theme.Label(textSize16, ws.totalBalance).Layout(gtx)
										}
										selectWallet := ws.SelectedWallet()
										totalBal, _ := walletBalance(selectWallet)
										return ws.Theme.Label(textSize16, selectWallet.ToAmount(totalBal).String()).Layout(gtx)
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
							}
							if ws.isLeftAlignment {
								return dim(gtx)
							}
							return layout.E.Layout(gtx, dim)
						}),
					)
				}), layout.Rigid(func(gtx C) D {
					if ws.errorLabel.Text != "" {
						inset := layout.Inset{
							Top:  unit.Dp(2),
							Left: unit.Dp(5),
						}
						return inset.Layout(gtx, func(gtx C) D {
							return ws.errorLabel.Layout(gtx)
						})
					}
					return D{}
				}),
			)
		}),
	)
}

func (ws *WalletAndAccountSelector) setWalletLogo(gtx C) D {
	if ws.HideLogo {
		return D{}
	}
	walletIcon := CoinImageBySymbol(ws.Load, ws.selectedWallet.GetAssetType(),
		ws.selectedWallet.IsWatchingOnlyWallet())
	if walletIcon == nil {
		return D{}
	}
	if ws.accountSelector {
		walletIcon = ws.Theme.Icons.AccountIcon
	}
	inset := layout.Inset{
		Right: values.MarginPadding8,
	}
	return inset.Layout(gtx, walletIcon.Layout24dp)
}

// ListenForTxNotifications listens for transaction and block updates and
// updates the selector modal, if the modal is open at the time of the update.
// The tx update listener MUST be unregistered using ws.StopTxNtfnListener()
// when the page using this WalletAndAccountSelector widget is exited.
func (ws *WalletAndAccountSelector) ListenForTxNotifications(window app.WindowNavigator) {
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(_ int, _ *sharedW.Transaction) {
			// refresh wallets/Accounts list when new transaction is received
			if ws.selectorModal == nil {
				return
			}
			if ws.accountSelector {
				ws.selectorModal.setupAccounts(ws.selectedWallet)
			} else {
				ws.selectorModal.setupWallet()
			}
			window.Reload()
		},
		OnBlockAttached: func(_ int, _ int32) {
			// refresh wallet and account balance on every new block
			// only if sync is completed.
			if !ws.selectedWallet.IsSynced() || ws.selectorModal == nil {
				return
			}
			if ws.accountSelector {
				ws.selectorModal.setupAccounts(ws.selectedWallet)
			} else {
				ws.selectorModal.setupWallet()
			}
			window.Reload()
		},
	}
	err := ws.selectedWallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, WalletAndAccountSelectorID)
	if err != nil {
		log.Errorf("WalletAndAccountSelector.ListenForTxNotifications error: %v", err)
	}
}

func (ws *WalletAndAccountSelector) StopTxNtfnListener() {
	ws.selectedWallet.RemoveTxAndBlockNotificationListener(WalletAndAccountSelectorID)
}

// SelectorItem models a wallet or an account along with it's clickable.
type SelectorItem struct {
	item      interface{} // Item can either be a wallet or an account.
	clickable *cryptomaterial.Clickable
}

func newSelectorModal(l *load.Load, assetType ...utils.AssetType) *selectorModal {
	sm := &selectorModal{
		Load:         l,
		Modal:        l.Theme.ModalFloatTitle("SelectorModal", l.IsMobileView()),
		walletsList:  layout.List{Axis: layout.Vertical},
		isCancelable: true,
		infoBackdrop: new(widget.Clickable),
		assetType:    assetType,
	}

	sm.infoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	sm.infoButton.Size = values.MarginPadding14
	sm.infoButton.Inset = layout.UniformInset(values.MarginPadding4)

	sm.accountIsValid = func(*sharedW.Account) bool { return false }

	var wallets []sharedW.Asset
	for _, asset := range assetType {
		switch asset {
		case utils.BTCWalletAsset:
			wallets = append(wallets, sm.AssetsManager.AllBTCWallets()...)
		case utils.DCRWalletAsset:
			wallets = append(wallets, sm.AssetsManager.AllDCRWallets()...)
		case utils.LTCWalletAsset:
			wallets = append(wallets, sm.AssetsManager.AllLTCWallets()...)
		}
	}

	if len(assetType) == 0 {
		wallets = sm.AssetsManager.AllWallets()
	}

	if len(wallets) > 0 {
		for _, w := range wallets {
			if sm.walletIsValid == nil || sm.walletIsValid(w) {
				sm.selectedWallet = w // Set the default wallet to wallet loaded by cryptopower.
				break
			}
		}
	}

	sm.accountSelector = false

	sm.Modal.ShowScrollbar(true)
	return sm
}

func (sm *selectorModal) OnResume() {
	if sm.accountSelector {
		sm.setupAccounts(sm.selectedWallet)
		return
	}
	sm.setupWallet(sm.assetType...)
}

func (sm *selectorModal) setupWallet(assetType ...utils.AssetType) {
	sm.selectorItems = make([]*SelectorItem, 0)

	wallets := sm.AssetsManager.AssetWallets(assetType...)
	for _, wal := range wallets {
		if wal.IsWatchingOnlyWallet() && !sm.isWatchOnlyEnabled || sm.walletIsValid != nil && !sm.walletIsValid(wal) {
			continue
		}
		sm.selectorItems = append(sm.selectorItems, &SelectorItem{
			item:      wal,
			clickable: sm.Theme.NewClickable(true),
		})
	}
}

func (sm *selectorModal) setupAccounts(wal sharedW.Asset) {
	selectorItems := make([]*SelectorItem, 0)
	// if isWatchOnlyEnabled is true the watch account of the watch only wallet will be added to the account selector list
	if !wal.IsWatchingOnlyWallet() || sm.isWatchOnlyEnabled {
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

func (sm *selectorModal) Handle(gtx C) {
	// if sm.eventQueue != nil {
	for _, selectorItem := range sm.selectorItems {
		for selectorItem.clickable.Clicked(gtx) {
			switch item := selectorItem.item.(type) {
			case *sharedW.Account:
				if sm.onAccountClicked != nil {
					sm.onAccountClicked(item)
				}
			case sharedW.Asset:
				if sm.onWalletClicked != nil {
					sm.onWalletClicked(item)
				}
			}
			sm.Dismiss()
		}
	}

	if sm.infoBackdrop.Clicked(gtx) {
		sm.infoModalOpen = false
	}

	if sm.infoButton.IconButtonStyle.Button.Clicked(gtx) {
		sm.infoModalOpen = !sm.infoModalOpen
	}
	// }

	if sm.Modal.BackdropClicked(gtx, sm.isCancelable) {
		sm.Dismiss()
	}
}

func (sm *selectorModal) title(title string) *selectorModal {
	sm.dialogTitle = title
	return sm
}

func (sm *selectorModal) walletClicked(callback func(sharedW.Asset)) *selectorModal {
	sm.onWalletClicked = callback
	return sm
}

func (sm *selectorModal) accountClicked(callback func(*sharedW.Account)) *selectorModal {
	sm.onAccountClicked = callback
	return sm
}

func (sm *selectorModal) Layout(gtx C) D {
	sm.eventSource = gtx.Source
	sm.infoBackdropLayout(gtx)

	w := []layout.Widget{
		func(gtx C) D {
			title := sm.Theme.H6(sm.dialogTitle)
			title.Color = sm.Theme.Color.Text
			title.Font.Weight = font.SemiBold
			return layout.Inset{
				Top: values.MarginPaddingMinus15,
			}.Layout(gtx, title.Layout)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if sm.accountSelector {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if !sm.infoModalOpen {
									return D{}
								}

								m := op.Record(gtx.Ops)
								dims := layout.Inset{Top: values.MarginPadding30}.Layout(gtx, func(gtx C) D {
									card := sm.Theme.Card()
									card.Color = sm.Theme.Color.Surface
									return card.Layout(gtx, func(gtx C) D {
										return layout.UniformInset(values.MarginPadding12).Layout(gtx, renderers.RenderHTML(sm.infoActionText, sm.Theme).Layout)
									})
								})
								op.Defer(gtx.Ops, m.Stop())
								return dims
							}),
							layout.Rigid(func(gtx C) D {
								if sm.infoActionText == "" {
									return D{}
								}
								return sm.infoButton.Layout(gtx)
							}),
						)
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

func (sm *selectorModal) modalListItemLayout(gtx C, selectorItem *SelectorItem) D {
	accountIcon := sm.Theme.Icons.AccountIcon
	switch n := selectorItem.item.(type) {
	case *sharedW.Account:
		accountIcon = sm.Theme.Icons.AccountIcon
	case sharedW.Asset:
		accountIcon = CoinImageBySymbol(sm.Load, n.GetAssetType(), n.IsWatchingOnlyWallet())
	}

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Margin:    layout.Inset{Bottom: values.MarginPadding4},
		Padding:   VerticalInset(values.MarginPadding8),
		Clickable: selectorItem.clickable,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Flexed(0.1, accountIcon.Layout20dp),
		layout.Flexed(0.8, func(gtx C) D {
			var name, totalBal, spendableBal string
			switch t := selectorItem.item.(type) {
			case *sharedW.Account:
				totalBal = t.Balance.Total.String()
				spendableBal = t.Balance.Spendable.String()
				name = t.Name
			case sharedW.Asset:
				tb, sb := walletBalance(t)
				totalBal = t.ToAmount(tb).String()
				spendableBal = t.ToAmount(sb).String()
				name = t.GetWalletName()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					acct := sm.Theme.Label(values.TextSize18, name)
					acct.MaxLines = 1
					acct.Color = sm.Theme.Color.Text
					acct.Font.Weight = font.Normal
					return EndToEndRow(gtx, func(gtx C) D {
						gtx.Constraints.Max.X = gtx.Constraints.Max.X / 2
						return acct.Layout(gtx)
					}, func(gtx C) D {
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

// return the count of wallets and accounts
func (sm *selectorModal) Count() int {
	return len(sm.selectorItems)
}

func (sm *selectorModal) OnDismiss() {}
