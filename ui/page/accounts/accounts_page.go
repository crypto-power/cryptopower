package accounts

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const AccountsPageID = "Accounts"

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet sharedW.Asset

	container     *widget.List
	addAccountBtn *cryptomaterial.Clickable
	accountsList  *cryptomaterial.ClickableList
	accounts      []*sharedW.Account

	exchangeRate   float64
	usdExchangeSet bool
}

func NewAccountPage(l *load.Load, wallet sharedW.Asset) *Page {
	pg := &Page{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(AccountsPageID),
		container: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		addAccountBtn: l.Theme.NewClickable(false),
		accountsList:  l.Theme.NewClickableList(layout.Vertical),
		wallet:        wallet,
	}

	return pg
}

func (pg *Page) loadWalletAccount() {
	walletAccounts := make([]*sharedW.Account, 0)
	accounts, err := pg.wallet.GetAccountsRaw()
	if err != nil {
		log.Errorf("error retrieving wallet accounts: %v", err)
		return
	}

	for _, acct := range accounts.Accounts {
		if acct.Number == dcr.ImportedAccountNumber {
			continue
		}
		walletAccounts = append(walletAccounts, acct)
	}

	pg.accounts = walletAccounts
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	pg.loadWalletAccount()
	pg.usdExchangeSet = false
	if pg.AssetsManager.ExchangeRateFetchingEnabled() {
		pg.usdExchangeSet = pg.AssetsManager.RateSource.Ready()
		go pg.fetchExchangeRate()
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {}

func (pg *Page) fetchExchangeRate() {
	var market string
	switch pg.wallet.GetAssetType() {
	case libutils.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libutils.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libutils.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", pg.wallet.GetAssetType())
		return
	}

	rate := pg.AssetsManager.RateSource.GetTicker(market)
	if rate == nil || rate.LastTradePrice <= 0 {
		return
	}

	pg.exchangeRate = rate.LastTradePrice
	pg.ParentWindow().Reload()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx C) D {
	return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return components.UniformHorizontalInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(pg.headerLayout),
					layout.Rigid(pg.bodyLayout),
				)
			})
		})
	})
}

func (pg *Page) bodyLayout(gtx C) D {
	inset := layout.Inset{Top: values.MarginPadding24, Bottom: values.MarginPadding24}
	if pg.IsMobileView() {
		inset = layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}
		pg.accountsList.Radius = cryptomaterial.Radius(8)
		pg.accountsList.CompleteRadius = true
		pg.accountsList.ClickableInset = cryptomaterial.ClickableInset{Bottom: values.MarginPadding20}
	}
	return inset.Layout(gtx, func(gtx C) D {
		return pg.accountsList.Layout(gtx, len(pg.accounts), func(gtx C, i int) D {
			if pg.IsMobileView() {
				return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, func(gtx C) D {
					return pg.mobileAccountItemLayout(gtx, pg.accounts[i])
				})
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return pg.accountItemLayout(gtx, pg.accounts[i])
				}),
				layout.Rigid(func(gtx C) D {
					isLastItem := i == len(pg.accounts)-1
					if isLastItem {
						return D{}
					}
					return pg.itemLine(gtx)
				}),
			)
		})
	})
}

func (pg *Page) headerLayout(gtx C) D {
	return layout.Inset{
		Top: values.MarginPadding24,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				txt := pg.Theme.Label(values.TextSize20, values.String(values.StrAccounts))
				txt.Font.Weight = font.SemiBold
				return txt.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, pg.addAccountBtnLayout)
			}),
		)
	})
}

func (pg *Page) addAccountBtnLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Background: pg.Theme.Color.DefaultThemeColors().SurfaceHighlight,
		Clickable:  pg.addAccountBtn,
		Alignment:  layout.Middle,
	}.Layout(gtx,
		layout.Rigid(pg.Theme.Icons.AddIcon.Layout16dp),
		layout.Rigid(func(gtx C) D {
			txt := pg.Theme.Label(values.TextSize16, values.String(values.StrAddNewAccount))
			txt.Color = pg.Theme.Color.DefaultThemeColors().Primary
			txt.Font.Weight = font.SemiBold
			return layout.Inset{
				Left: values.MarginPadding8,
			}.Layout(gtx, txt.Layout)
		}),
	)
}

func (pg *Page) itemLine(gtx C) D {
	return layout.Inset{Top: values.MarginPadding12, Bottom: values.MarginPadding12}.Layout(gtx, func(gtx C) D {
		line := pg.Theme.Line(1, 0)
		line.Color = pg.Theme.Color.Gray9
		return line.Layout(gtx)
	})
}

func (pg *Page) mobileAccountItemLayout(gtx C, account *sharedW.Account) D {
	dp10 := values.MarginPadding10
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Padding:     layout.UniformInset(dp10),
		Orientation: layout.Vertical,
		Border: cryptomaterial.Border{
			Width:  values.MarginPadding1,
			Color:  pg.Theme.Color.Gray3,
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.mobileAccountBalanceLayout(gtx, false, account)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp10, Bottom: dp10}.Layout(gtx, pg.Theme.Separator().Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.mobileAccountBalanceLayout(gtx, true, account)
		}),
	)
}

func (pg *Page) mobileAccountBalanceLayout(gtx C, spendableLayout bool, account *sharedW.Account) D {
	var label, balanceTxt cryptomaterial.Label
	var balanceAmt float64
	if !spendableLayout {
		label = pg.Theme.Label(pg.ConvertTextSize(values.TextSize18), account.AccountName)
		label.Font.Weight = font.SemiBold
		balanceTxt = pg.Theme.Label(pg.ConvertTextSize(values.TextSize18), account.Balance.Total.String())
		balanceTxt.Font.Weight = font.SemiBold
		balanceAmt = pg.wallet.ToAmount(account.Balance.Total.ToInt()).ToCoin()
	} else {
		label = pg.Theme.Label(pg.ConvertTextSize(values.TextSize16), values.String(values.StrAmountSpendable))
		label.Font.Weight = font.SemiBold
		label.Color = pg.Theme.Color.GrayText3
		balanceTxt = pg.Theme.Label(pg.ConvertTextSize(values.TextSize16), account.Balance.Spendable.String())
		balanceTxt.Font.Weight = font.SemiBold
		balanceTxt.Color = pg.Theme.Color.GrayText3
		balanceAmt = pg.wallet.ToAmount(account.Balance.Spendable.ToInt()).ToCoin()
	}

	balanceUSD := "($ 0.00)"
	if pg.exchangeRate != -1 && pg.usdExchangeSet {
		balanceUSD = fmt.Sprintf("(%v)", utils.FormatAsUSDString(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, balanceAmt)))
	}
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(label.Layout), // Title
		layout.Flexed(1, func(gtx C) D { // Balances
			var usdAmtLabel cryptomaterial.Label
			if spendableLayout {
				usdAmtLabel = pg.Theme.Label(pg.ConvertTextSize(values.TextSize16), balanceUSD)
				usdAmtLabel.Font.Weight = font.SemiBold
				usdAmtLabel.Color = pg.Theme.Color.GrayText3
			} else {
				usdAmtLabel = pg.Theme.Label(pg.ConvertTextSize(values.TextSize16), balanceUSD)
				usdAmtLabel.Font.Weight = font.SemiBold
			}
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(balanceTxt.Layout),
					layout.Rigid(usdAmtLabel.Layout),
				)
			})
		}),
	)
}

func (pg *Page) accountItemLayout(gtx C, account *sharedW.Account) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Label(values.TextSize18, account.AccountName)
						txt.Font.Weight = font.SemiBold
						return txt.Layout(gtx)
					}),
					layout.Flexed(1, func(gtx C) D {
						balance := account.Balance.Total.String()
						totalBalance := pg.wallet.ToAmount(account.Balance.Total.ToInt()).ToCoin()
						if pg.exchangeRate != -1 && pg.usdExchangeSet {
							balanceUSD := utils.FormatAsUSDString(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, totalBalance))
							balance = fmt.Sprintf("%s (%s)", balance, balanceUSD)
						}
						txt := pg.Theme.Label(values.TextSize18, balance)
						txt.Font.Weight = font.SemiBold
						return layout.E.Layout(gtx, txt.Layout)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Label(values.TextSize16, values.String(values.StrAmountSpendable))
						txt.Font.Weight = font.SemiBold
						txt.Color = pg.Theme.Color.GrayText3
						return txt.Layout(gtx)
					}),

					layout.Flexed(1, func(gtx C) D {
						spendable := account.Balance.Spendable.String()
						spendableCoin := pg.wallet.ToAmount(account.Balance.Spendable.ToInt()).ToCoin()
						if pg.exchangeRate != -1 && pg.usdExchangeSet {
							balanceUSD := utils.FormatAsUSDString(pg.Printer, utils.CryptoToUSD(pg.exchangeRate, spendableCoin))
							spendable = fmt.Sprintf("%s (%s)", spendable, balanceUSD)
						}
						txt := pg.Theme.Label(values.TextSize16, spendable)
						txt.Color = pg.Theme.Color.GrayText3
						txt.Font.Weight = font.SemiBold
						return layout.E.Layout(gtx, txt.Layout)
					}),
				)
			})
		}),
	)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions() {
	if pg.addAccountBtn.Clicked() {
		createAccountModal := modal.NewCreatePasswordModal(pg.Load).
			Title(values.String(values.StrCreateNewAccount)).
			EnableName(true).
			NameHint(values.String(values.StrAcctName)).
			EnableConfirmPassword(false).
			PasswordHint(values.String(values.StrSpendingPassword)).
			SetPositiveButtonCallback(func(accountName, password string, m *modal.CreatePasswordModal) bool {
				_, err := pg.wallet.CreateNewAccount(accountName, password)
				if err != nil {
					m.SetError(err.Error())
					m.SetLoading(false)
					return false
				}
				pg.loadWalletAccount()
				m.Dismiss()

				info := modal.NewSuccessModal(pg.Load, values.StringF(values.StrAcctCreated),
					modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(info)
				return true
			})
		pg.ParentWindow().ShowModal(createAccountModal)
	}

	if clicked, selectedItem := pg.accountsList.ItemClicked(); clicked {
		switch pg.wallet.GetAssetType() {
		case libutils.BTCWalletAsset:
			pg.ParentNavigator().Display(NewBTCAcctDetailsPage(pg.Load, pg.wallet, pg.accounts[selectedItem]))
		case libutils.DCRWalletAsset:
			pg.ParentNavigator().Display(NewDCRAcctDetailsPage(pg.Load, pg.wallet, pg.accounts[selectedItem]))
		case libutils.LTCWalletAsset:
			pg.ParentNavigator().Display(NewLTCAcctDetailsPage(pg.Load, pg.wallet, pg.accounts[selectedItem]))
		}
	}
}
