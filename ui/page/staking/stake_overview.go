package staking

import (
	"context"
	"fmt"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	tpage "github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	OverviewPageID = "staking"

	// pageSize define the maximum number of items fetched for the list scroll view.
	pageSize int32 = 20
)

type Page struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	*listeners.TxAndBlockNotificationListener

	scroll *components.Scroll

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	ticketOverview *dcr.StakingOverview

	ticketsList    *cryptomaterial.ClickableList
	stakeSettings  *cryptomaterial.Clickable
	stake          *cryptomaterial.Switch
	infoButton     cryptomaterial.IconButton
	materialLoader material.LoaderStyle

	ticketPrice        string
	totalRewards       string
	showMaterialLoader bool

	navToSettingsBtn cryptomaterial.Button
	processingTicket uint32

	dcrImpl *dcr.Asset
}

func NewStakingPage(l *load.Load) *Page {
	impl := l.WL.SelectedWallet.Wallet.(*dcr.Asset)
	if impl == nil {
		log.Error(values.ErrDCRSupportedOnly)
		return nil
	}

	pg := &Page{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OverviewPageID),
		dcrImpl:          impl,
	}

	pg.scroll = components.NewScroll(l, pageSize, pg.fetchTickets)
	pg.materialLoader = material.Loader(l.Theme.Base)
	pg.ticketOverview = new(dcr.StakingOverview)
	pg.initStakePriceWidget()
	pg.initTicketList()

	pg.navToSettingsBtn = l.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrVsp)))

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	// pg.ctx is used to load known vsps in background and
	// canceled in OnNavigatedFrom().

	// If staking is disabled no startup func should be called
	// Layout will draw an overlay to show that stacking is disabled.

	isSyncingOrRescanning := !pg.WL.SelectedWallet.Wallet.IsSynced() || pg.WL.SelectedWallet.Wallet.IsRescanning()
	if pg.isTicketsPurchaseAllowed() && !isSyncingOrRescanning {
		pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

		pg.fetchTicketPrice()

		pg.loadPageData() // starts go routines to refresh the display which is just about to be displayed, ok?

		pg.stake.SetChecked(pg.dcrImpl.IsAutoTicketsPurchaseActive())

		pg.setStakingButtonsState()

		pg.listenForTxNotifications()
		go func() {
			pg.showMaterialLoader = true
			pg.scroll.FetchScrollData(false, pg.ParentWindow())
			pg.showMaterialLoader = false
		}()
	}
}

// fetch ticket price only when the wallet is synced
func (pg *Page) fetchTicketPrice() {
	ticketPrice, err := pg.dcrImpl.TicketPrice()
	if err != nil && !pg.WL.SelectedWallet.Wallet.IsSynced() {
		log.Error(err)
		pg.ticketPrice = dcrutil.Amount(0).String()
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrWalletNotSynced), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
	} else {
		pg.ticketPrice = dcrutil.Amount(ticketPrice.TicketPrice).String()
	}
}

func (pg *Page) setStakingButtonsState() {
	// disable auto ticket purchase if wallet is not synced
	pg.stake.SetEnabled(pg.WL.SelectedWallet.Wallet.IsSynced() || !pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet())
}

func (pg *Page) loadPageData() {
	go func() {
		if len(pg.dcrImpl.KnownVSPs()) == 0 {
			// TODO: Does this page need this list?
			if pg.ctx != nil {
				pg.dcrImpl.ReloadVSPList(pg.ctx)
			}
		}

		totalRewards, err := pg.dcrImpl.TotalStakingRewards()
		if err != nil {
			errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(errModal)
		} else {
			pg.totalRewards = dcrutil.Amount(totalRewards).String()
		}

		overview, err := pg.dcrImpl.StakingOverview()
		if err != nil {
			errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(errModal)
		} else {
			pg.ticketOverview = overview
		}

		pg.ParentWindow().Reload()
	}()
}

func (pg *Page) isTicketsPurchaseAllowed() bool {
	return pg.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.VspAPI)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx C) D {
	// If Tickets Purcahse API is not allowed, display the overlay with the message.
	isSyncingOrRescanning := !pg.WL.SelectedWallet.Wallet.IsSynced() || pg.WL.SelectedWallet.Wallet.IsRescanning()
	overlay := layout.Stacked(func(gtx C) D { return D{} })
	if !pg.isTicketsPurchaseAllowed() && !isSyncingOrRescanning {
		gtxCopy := gtx
		overlay = layout.Stacked(func(gtx C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrVsp))
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, str, &pg.navToSettingsBtn)
		})
		// Disable main page from recieving events
		gtx = gtx.Disabled()
	}

	mainChild := layout.Expanded(func(gtx C) D {
		if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
			return pg.layoutMobile(gtx)
		}
		return pg.layoutDesktop(gtx)
	})

	return layout.Stack{}.Layout(gtx, mainChild, overlay)
}

func (pg *Page) layoutDesktop(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())

	return layout.Inset{Top: values.MarginPadding24, Bottom: values.MarginPadding14}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return components.UniformHorizontalPadding(gtx, pg.stakePriceSection)
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
					if pg.showMaterialLoader {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Center.Layout(gtx, func(gtx C) D {
							return pg.materialLoader.Layout(gtx)
						})
					}
					return components.UniformHorizontalPadding(gtx, func(gtx C) D {
						return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
							return pg.ticketListLayout(gtx)
						})
					})
				})
			}),
		)
	})
}

func (pg *Page) layoutMobile(gtx layout.Context) layout.Dimensions {
	widgets := []layout.Widget{
		pg.stakePriceSection,
		pg.ticketListLayout,
	}

	return components.UniformMobile(gtx, true, true, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
			return pg.scroll.List().Layout(gtx, len(widgets), func(gtx C, i int) D {
				return widgets[i](gtx)
			})
		})
	})
}

func (pg *Page) pageSections(gtx C, body layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding8,
	}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(values.MarginPadding26).Layout(gtx, body)
		})
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions() {
	pg.setStakingButtonsState()

	if pg.navToSettingsBtn.Clicked() {
		pg.ParentWindow().Display(settings.NewSettingsPage(pg.Load))
	}

	if pg.stake.Changed() {
		if pg.stake.IsChecked() {
			if pg.dcrImpl.TicketBuyerConfigIsSet() {
				// get ticket buyer config to check if the saved wallet account is mixed
				// check if mixer is set, if yes check if allow spend from unmixed account
				// if not set, check if the saved account is mixed before opening modal
				// if it is not, open stake config modal
				tbConfig := pg.dcrImpl.AutoTicketsBuyerConfig()
				if pg.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
					!pg.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) &&
					(tbConfig.PurchaseAccount == pg.dcrImpl.MixedAccountNumber()) {
					pg.startTicketBuyerPasswordModal()
				} else {
					pg.ticketBuyerSettingsModal()
				}
			} else {
				pg.ticketBuyerSettingsModal()
			}
		} else {
			pg.dcrImpl.StopAutoTicketsPurchase()
		}
	}

	if pg.stakeSettings.Clicked() && !pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
		if pg.dcrImpl.IsAutoTicketsPurchaseActive() {
			errModal := modal.NewErrorModal(pg.Load, values.String(values.StrAutoTicketWarn), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(errModal)
			return
		}

		ticketBuyerModal := newTicketBuyerModal(pg.Load).
			OnSettingsSaved(func() {
				infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrTicketSettingSaved), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(infoModal)
			}).
			OnCancel(func() {
				pg.stake.SetChecked(false)
			})
		pg.ParentWindow().ShowModal(ticketBuyerModal)
	}

	secs, _ := pg.dcrImpl.NextTicketPriceRemaining()
	if secs <= 0 {
		pg.fetchTicketPrice()
	}

	if pg.WL.SelectedWallet.Wallet.IsSynced() {
		pg.fetchTicketPrice()
	}

	if clicked, selectedItem := pg.ticketsList.ItemClicked(); clicked {
		tickets := pg.scroll.FetchedData().([]*transactionItem)
		ticketTx := tickets[selectedItem].transaction
		pg.ParentNavigator().Display(tpage.NewTransactionDetailsPage(pg.Load, ticketTx, true))

		// Check if this ticket is fully registered with a VSP
		// and log any discrepancies.
		// NOTE: Wallet needs to be unlocked to get the ticket status
		// from the vsp. Otherwise, only the wallet-stored info will
		// be retrieved. This is fine because we're only just logging
		// but where it is necessary to display vsp-stored info, the
		// wallet passphrase should be requested and used to unlock
		// the wallet before calling this method.
		ticketInfo, err := pg.dcrImpl.VSPTicketInfo(ticketTx.Hash)
		if err != nil {
			log.Errorf("VSPTicketInfo error: %v", err)
		} else {
			if ticketInfo.FeeTxStatus != dcr.VSPFeeProcessConfirmed || !ticketInfo.ConfirmedByVSP {
				log.Warnf("Ticket %s has unconfirmed fee tx with status %q, vsp %s",
					ticketTx.Hash, ticketInfo.FeeTxStatus.String(), ticketInfo.VSP)
			}

			// Confirm that fee hasn't been paid, sender account exists, the wallet
			// is unlocked and no previous ticket processing instance is running.
			if ticketInfo.FeeTxStatus != dcr.VSPFeeProcessPaid && len(ticketTx.Inputs) == 1 &&
				ticketInfo.Client != nil && atomic.CompareAndSwapUint32(&pg.processingTicket, 0, 1) {

				log.Infof("Attempting to process the unconfirmed VSP fee for tx: %v", ticketTx.Hash)

				txHash, err := chainhash.NewHashFromStr(ticketTx.Hash)
				if err != nil {
					log.Errorf("convert hex to hash failed: %v", ticketTx.Hash)
					return
				}

				account := ticketTx.Inputs[0].AccountNumber
				err = ticketInfo.Client.ProcessTicket(pg.ctx, txHash, pg.dcrImpl.GetvspPolicy(account))
				if err != nil {
					log.Errorf("processing the unconfirmed tx fee failed: %v", err)
				}

				// Reset the processing
				atomic.StoreUint32(&pg.processingTicket, 0)
			}
		}
	}

	if pg.infoButton.Button.Clicked() {
		backupNowOrLaterModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrStatistics)).
			SetCancelable(true).
			UseCustomWidget(func(gtx C) D {
				return pg.stakingRecordStatistics(gtx)
			}).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(backupNowOrLaterModal)
	}
}

func (pg *Page) ticketBuyerSettingsModal() {
	ticketBuyerModal := newTicketBuyerModal(pg.Load).
		OnCancel(func() {
			pg.stake.SetChecked(false)
		}).
		OnSettingsSaved(func() {
			pg.startTicketBuyerPasswordModal()
			infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrTicketSettingSaved), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(infoModal)
		})
	pg.ParentWindow().ShowModal(ticketBuyerModal)
}

func (pg *Page) startTicketBuyerPasswordModal() {
	tbConfig := pg.dcrImpl.AutoTicketsBuyerConfig()
	balToMaintain := pg.WL.SelectedWallet.Wallet.ToAmount(tbConfig.BalanceToMaintain).ToCoin()
	name, err := pg.WL.SelectedWallet.Wallet.AccountNameRaw(uint32(tbConfig.PurchaseAccount))
	if err != nil {
		errModal := modal.NewErrorModal(pg.Load, values.StringF(values.StrTicketError, err), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrConfirmPurchase)).
		SetCancelable(false).
		UseCustomWidget(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.Theme.Label(values.TextSize14, values.StringF(values.StrWalletToPurchaseFrom, pg.WL.SelectedWallet.Wallet.GetWalletName())).Layout),
				layout.Rigid(pg.Theme.Label(values.TextSize14, values.StringF(values.StrSelectedAccount, name)).Layout),
				layout.Rigid(pg.Theme.Label(values.TextSize14, values.StringF(values.StrBalToMaintainValue, balToMaintain)).Layout), layout.Rigid(func(gtx C) D {
					label := pg.Theme.Label(values.TextSize14, fmt.Sprintf("VSP: %s", tbConfig.VspHost))
					return layout.Inset{Bottom: values.MarginPadding12}.Layout(gtx, label.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:      cryptomaterial.MatchParent,
						Height:     cryptomaterial.WrapContent,
						Background: pg.Theme.Color.LightBlue,
						Padding: layout.Inset{
							Top:    values.MarginPadding12,
							Bottom: values.MarginPadding12,
						},
						Border:    cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
						Direction: layout.Center,
						Alignment: layout.Middle,
					}.Layout2(gtx, func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
							msg := values.String(values.StrAutoTicketInfo)
							txt := pg.Theme.Label(values.TextSize14, msg)
							txt.Alignment = text.Middle
							txt.Color = pg.Theme.Color.GrayText3
							if pg.WL.AssetsManager.IsDarkModeOn() {
								txt.Color = pg.Theme.Color.Gray3
							}
							return txt.Layout(gtx)
						})
					})
				}),
			)
		}).
		SetNegativeButtonCallback(func() { pg.stake.SetChecked(false) }).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			if !pg.WL.SelectedWallet.Wallet.IsConnectedToNetwork() {
				pm.SetError(values.String(values.StrNotConnected))
				pm.SetLoading(false)
				pg.stake.SetChecked(false)
				return false
			}

			err := pg.dcrImpl.StartTicketBuyer(password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}

			pg.stake.SetChecked(pg.dcrImpl.IsAutoTicketsPurchaseActive())
			pg.ParentWindow().Reload()

			pm.Dismiss()

			return false
		})
	pg.ParentWindow().ShowModal(walletPasswordModal)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {
	// There are cases where context was never created in the first place
	// for instance if VSP is disabled will not be created, so context cancellation
	// should be ignored.
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}
