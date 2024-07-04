package staking

import (
	"context"
	"fmt"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	tpage "github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/values"
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

	// The ticket height limit helps separate the scrolling of the ticket list and the page
	ticketHeight = 500
)

type Page struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scroll          *components.Scroll[*transactionItem]
	scrollContainer *widget.List

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

	dcrWallet *dcr.Asset

	// ticketContext is a managed context instance that is shut once a shutdown
	// request is made. It helps avoid the use of context.TODO() that isn't
	// responsive to the shutdown request.
	ticketContext context.Context
}

func NewStakingPage(l *load.Load, dcrWallet *dcr.Asset) *Page {
	pg := &Page{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OverviewPageID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		dcrWallet: dcrWallet,
	}

	// context will list for a shutdown request.
	pg.ticketContext, _ = dcrWallet.ShutdownContextWithCancel()

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
	// If staking is disabled no startup func should be called
	// Layout will draw an overlay to show that stacking is disabled.

	isSyncingOrRescanning := !pg.dcrWallet.IsSynced() || pg.dcrWallet.IsRescanning()
	if pg.isTicketsPurchaseAllowed() && !isSyncingOrRescanning {
		pg.fetchTicketPrice()

		pg.loadPageData() // starts go routines to refresh the display which is just about to be displayed, ok?

		pg.stake.SetChecked(pg.dcrWallet.IsAutoTicketsPurchaseActive())

		pg.setStakingButtonsState()

		pg.listenForTxNotifications() // tx ntfn listener is stopped in OnNavigatedFrom().

		go func() {
			pg.showMaterialLoader = true
			pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
			pg.showMaterialLoader = false
		}()
	}
}

// fetch ticket price only when the wallet is synced
func (pg *Page) fetchTicketPrice() {
	ticketPrice, err := pg.dcrWallet.TicketPrice()
	if err != nil && !pg.dcrWallet.IsSynced() {
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
	pg.stake.SetEnabled(pg.dcrWallet.IsSynced() || !pg.dcrWallet.IsWatchingOnlyWallet())
}

func (pg *Page) loadPageData() {
	go func() {
		if len(pg.dcrWallet.KnownVSPs()) == 0 {
			// TODO: Does this page need this list?
			pg.dcrWallet.ReloadVSPList(context.TODO())
		}

		totalRewards, err := pg.dcrWallet.TotalStakingRewards()
		if err != nil {
			errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(errModal)
		} else {
			pg.totalRewards = dcrutil.Amount(totalRewards).String()
		}

		overview, err := pg.dcrWallet.StakingOverview()
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
	return pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.VspAPI)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx C) D {
	// If Tickets Purchase API is not allowed, display the overlay with the message.
	isSyncingOrRescanning := !pg.dcrWallet.IsSynced() || pg.dcrWallet.IsRescanning()
	overlay := layout.Stacked(func(_ C) D { return D{} })
	if !pg.isTicketsPurchaseAllowed() && !isSyncingOrRescanning {
		gtxCopy := gtx
		overlay = layout.Stacked(func(_ C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrVsp))
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, str, "", &pg.navToSettingsBtn)
		})
		// Disable main page from receiving events
		gtx = gtx.Disabled()
	}

	mainChild := layout.Expanded(func(gtx C) D {
		pg.scroll.OnScrollChangeListener(pg.ParentWindow())
		return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, _ int) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.stakePriceSection),
				layout.Rigid(pg.stakeStatisticsSection),
				layout.Rigid(pg.ticketListLayout),
			)
		})
	})

	return layout.Stack{}.Layout(gtx, mainChild, overlay)
}

func (pg *Page) pageSections(gtx C, body layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding24)).Layout(gtx, body)
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
		pg.ParentWindow().Display(settings.NewAppSettingsPage(pg.Load))
	}

	if pg.stake.Changed() {
		if pg.stake.IsChecked() {
			if pg.dcrWallet.TicketBuyerConfigIsSet() {
				// get ticket buyer config to check if the saved wallet account is mixed
				// check if mixer is set, if yes check if allow spend from unmixed account
				// if not set, check if the saved account is mixed before opening modal
				// if it is not, open stake config modal
				tbConfig := pg.dcrWallet.AutoTicketsBuyerConfig()
				if pg.dcrWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false) &&
					!pg.dcrWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false) &&
					(tbConfig.PurchaseAccount == pg.dcrWallet.MixedAccountNumber()) {
					pg.startTicketBuyerPasswordModal()
				} else {
					pg.ticketBuyerSettingsModal()
				}
			} else {
				pg.ticketBuyerSettingsModal()
			}
		} else {
			_ = pg.dcrWallet.StopAutoTicketsPurchase()
		}
	}

	if pg.stakeSettings.Clicked() && !pg.dcrWallet.IsWatchingOnlyWallet() {
		if pg.dcrWallet.IsAutoTicketsPurchaseActive() {
			errModal := modal.NewErrorModal(pg.Load, values.String(values.StrAutoTicketWarn), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(errModal)
			return
		}

		ticketBuyerModal := newTicketBuyerModal(pg.Load, pg.dcrWallet).
			OnSettingsSaved(func() {
				infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrTicketSettingSaved), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(infoModal)
			}).
			OnCancel(func() {
				pg.stake.SetChecked(false)
			})
		pg.ParentWindow().ShowModal(ticketBuyerModal)
	}

	secs, _ := pg.dcrWallet.NextTicketPriceRemaining()
	if secs <= 0 {
		pg.fetchTicketPrice()
	}

	if pg.dcrWallet.IsSynced() {
		pg.fetchTicketPrice()
	}

	if clicked, selectedItem := pg.ticketsList.ItemClicked(); clicked {
		tickets := pg.scroll.FetchedData()
		ticketTx := tickets[selectedItem].transaction
		pg.ParentNavigator().Display(tpage.NewTransactionDetailsPage(pg.Load, pg.dcrWallet, ticketTx))

		// Check if this ticket is fully registered with a VSP
		// and log any discrepancies.
		// NOTE: Wallet needs to be unlocked to get any ticket info
		// from the vsp. This is fine because we're only just logging
		// but where it is necessary to display vsp-stored info, the
		// wallet passphrase should be requested and used to unlock
		// the wallet before calling this method.
		ticketInfo, err := pg.dcrWallet.VSPTicketInfo(ticketTx.Hash)
		if err != nil {
			if err.Error() != libutils.ErrWalletLocked {
				// Ignore the wallet is locked error.
				log.Errorf("VSPTicketInfo error: %v", err)
			}
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

				err = ticketInfo.Client.Process(pg.ticketContext, ticketInfo.VSPTicket, nil)
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
	ticketBuyerModal := newTicketBuyerModal(pg.Load, pg.dcrWallet).
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
	tbConfig := pg.dcrWallet.AutoTicketsBuyerConfig()
	balToMaintain := pg.dcrWallet.ToAmount(tbConfig.BalanceToMaintain).ToCoin()
	name, err := pg.dcrWallet.AccountNameRaw(uint32(tbConfig.PurchaseAccount))
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
				layout.Rigid(pg.Theme.Label(values.TextSize14, values.StringF(values.StrWalletToPurchaseFrom, pg.dcrWallet.GetWalletName())).Layout),
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
							if pg.AssetsManager.IsDarkModeOn() {
								txt.Color = pg.Theme.Color.Gray3
							}
							return txt.Layout(gtx)
						})
					})
				}),
			)
		}).
		SetNegativeButtonCallback(func() {
			_ = pg.dcrWallet.StopAutoTicketsPurchase()
			pg.stake.SetChecked(false)
		}).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			pg.stake.SetChecked(false)

			if !pg.dcrWallet.IsConnectedToNetwork() {
				pm.SetError(values.String(values.StrNotConnected))
				_ = pg.dcrWallet.StopAutoTicketsPurchase() // Halt auto tickets purchase.
				return false
			}

			if err := pg.dcrWallet.StartTicketBuyer(password); err != nil {
				pm.SetError(err.Error())
				_ = pg.dcrWallet.StopAutoTicketsPurchase() // Halt auto tickets purchase.
				return false
			}

			pg.stake.SetChecked(pg.dcrWallet.IsAutoTicketsPurchaseActive())
			pg.ParentWindow().Reload()
			pm.Dismiss()

			return true
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
	pg.stopTxNotificationsListener()
}
