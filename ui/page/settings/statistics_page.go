package settings

import (
	"fmt"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const StatisticsPageID = "Statistics"

type StatPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet   sharedW.Asset
	txs      []*sharedW.Transaction
	accounts *sharedW.Accounts

	l             layout.List
	scrollbarList *widget.List
	startupTime   string

	backButton cryptomaterial.IconButton
}

func NewStatPage(l *load.Load, wallet sharedW.Asset) *StatPage {
	pg := &StatPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(StatisticsPageID),
		wallet:           wallet,
		l:                layout.List{Axis: layout.Vertical},
		scrollbarList: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}

	pg.backButton = components.GetBackButton(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *StatPage) OnNavigatedTo() {
	txs, err := pg.wallet.GetTransactionsRaw(0, 0, dcr.TxFilterAll, true, "")
	if err != nil {
		log.Errorf("Error getting txs: %s", err.Error())
	} else {
		pg.txs = txs
	}

	acc, err := pg.wallet.GetAccountsRaw()
	if err != nil {
		log.Errorf("Error getting wallet accounts: %s", err.Error())
	} else {
		// Filter imported account.
		accounts := make([]*sharedW.Account, 0)
		for _, v := range acc.Accounts {
			if pg.wallet.GetAssetType() == libutils.BTCWalletAsset && v.AccountNumber != btc.ImportedAccountNumber {
				accounts = append(accounts, v)
			}

			if pg.wallet.GetAssetType() == libutils.DCRWalletAsset && v.Number != dcr.ImportedAccountNumber {
				accounts = append(accounts, v)
			}

		}
		acc.Accounts = accounts
		pg.accounts = acc
	}

	pg.appStartTime()
}

func (pg *StatPage) layoutStats(gtx C) D {
	background := pg.Theme.Color.Surface
	card := pg.Theme.Card()
	card.Color = background
	inset := layout.Inset{
		Top:    values.MarginPadding12,
		Bottom: values.MarginPadding12,
		Right:  values.MarginPadding16,
	}

	item := func(t, v string) layout.Widget {
		return func(gtx C) D {
			l := pg.Theme.Body2(t)
			r := pg.Theme.Body2(v)
			r.Color = pg.Theme.Color.GrayText2
			return inset.Layout(gtx, func(gtx C) D {
				return components.EndToEndRow(gtx, l.Layout, r.Layout)
			})
		}
	}

	bestBlock := pg.wallet.GetBestBlockHeight()
	bestBlockTime := time.Unix(pg.wallet.GetBestBlockTimeStamp(), 0)
	secondsSinceBestBlock := int64(time.Since(bestBlockTime).Seconds())

	walletDataSize := "Unknown"
	v, err := pg.AssetsManager.RootDirFileSizeInBytes(pg.wallet.DataDir())
	if err != nil {
		walletDataSize = fmt.Sprintf("%f GB", float64(v)*1e-9)
	}

	line := pg.Theme.Separator()
	line.Color = pg.Theme.Color.Gray2

	netType := pg.AssetsManager.NetType().Display()
	items := []layout.Widget{
		item(values.String(values.StrBuild), netType+", "+time.Now().Format("2006-01-02")),
		line.Layout,
		item(values.String(values.StrPeersConnected), strconv.Itoa(int(pg.wallet.ConnectedPeers()))),
		line.Layout,
		item(values.String(values.StrUptime), pg.startupTime),
		line.Layout,
		item(values.String(values.StrNetwork), netType),
		line.Layout,
		item(values.String(values.StrBestBlocks), fmt.Sprintf("%d", bestBlock)),
		line.Layout,
		item(values.String(values.StrBestBlockTimestamp), bestBlockTime.Format("2006-01-02 03:04:05 -0700")),
		line.Layout,
		item(values.String(values.StrBestBlockAge), components.SecondsToDays(secondsSinceBestBlock)),
		line.Layout,
		item(values.String(values.StrWalletDirectory), pg.wallet.DataDir()),
		line.Layout,
		item(values.String(values.StrDateSize), walletDataSize),
		line.Layout,
		item(values.String(values.StrTransactions), fmt.Sprintf("%d", len(pg.txs))),
		line.Layout,
		item(values.String(values.StrAccount)+"s", fmt.Sprintf("%d", len(pg.accounts.Accounts))),
	}

	return pg.Theme.List(pg.scrollbarList).Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
			return card.Layout(gtx, func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.l.Layout(gtx, len(items), func(gtx C, i int) D {
						return items[i](gtx)
					})
				})
			})
		})
	})
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *StatPage) Layout(gtx C) D {
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *StatPage) layoutDesktop(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrStatistics),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: pg.layoutStats,
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}

	// Refresh frames every 1 second
	op.InvalidateOp{At: time.Now().Add(time.Second * 1)}.Add(gtx.Ops)
	return container(gtx)
}

func (pg *StatPage) layoutMobile(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrStatistics),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: pg.layoutStats,
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}

	// Refresh frames every 1 second
	op.InvalidateOp{At: time.Now().Add(time.Second * 1)}.Add(gtx.Ops)
	return components.UniformMobile(gtx, false, true, container)
}

func (pg *StatPage) appStartTime() {
	pg.startupTime = func(t time.Time) string {
		v := int(time.Since(t).Seconds())
		h := v / 3600
		m := (v - h*3600) / 60
		s := v - h*3600 - m*60
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}(pg.AppInfo.StartupTime())
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *StatPage) HandleUserInteractions() {
	pg.appStartTime()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *StatPage) OnNavigatedFrom() {}
