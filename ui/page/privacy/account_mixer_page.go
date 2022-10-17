package privacy

import (
	"context"
	"fmt"

	"gioui.org/layout"

	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/preference"
	"gitlab.com/raedah/cryptopower/ui/renderers"
	"gitlab.com/raedah/cryptopower/ui/values"
	"gitlab.com/raedah/cryptopower/wallet"
)

const AccountMixerPageID = "AccountMixer"

type AccountMixerPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	*listeners.AccountMixerNotificationListener
	*listeners.TxAndBlockNotificationListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	pageContainer layout.List
	wallet        *dcr.DCRAsset

	settingsCollapsible *cryptomaterial.Collapsible
	unmixedAccount      *cryptomaterial.Clickable
	mixedAccount        *cryptomaterial.Clickable
	coordinationServer  *cryptomaterial.Clickable
	toggleMixer         *cryptomaterial.Switch
	mixerProgress       cryptomaterial.ProgressBarStyle

	mixedBalance       dcrutil.Amount
	unmixedBalance     dcrutil.Amount
	totalWalletBalance dcrutil.Amount

	ArrMixerAccounts map[string]string

	mixerCompleted bool
}

func NewAccountMixerPage(l *load.Load) *AccountMixerPage {
	pg := &AccountMixerPage{
		Load:                l,
		GenericPageModal:    app.NewGenericPageModal(AccountMixerPageID),
		wallet:              l.WL.SelectedWallet.Wallet,
		toggleMixer:         l.Theme.Switch(),
		mixerProgress:       l.Theme.ProgressBar(0),
		settingsCollapsible: l.Theme.Collapsible(),
		unmixedAccount:      l.Theme.NewClickable(false),
		mixedAccount:        l.Theme.NewClickable(false),
		coordinationServer:  l.Theme.NewClickable(false),
		pageContainer:       layout.List{Axis: layout.Vertical},
	}

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *AccountMixerPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.listenForMixerNotifications()
	pg.toggleMixer.SetChecked(pg.WL.SelectedWallet.Wallet.IsAccountMixerActive())
	pg.mixerProgress.Height = values.MarginPadding18
	pg.mixerProgress.Radius = cryptomaterial.Radius(2)
	totalBalance, _ := components.CalculateTotalWalletsBalance(pg.Load) // TODO - handle error
	pg.totalWalletBalance = totalBalance.Total
	// get balance information
	pg.getMixerBalance()
}

func (pg *AccountMixerPage) getMixerBalance() {
	accounts, err := pg.wallet.GetAccountsRaw()
	if err != nil {
		log.Error("could not load mixer account information. Please try again.")
	}

	vm := make(map[string]string)
	for _, acct := range accounts.Acc {
		// add data for change accounts selection
		if acct.Name != "imported" {
			vm[acct.Name] = acct.Name
		}

		if acct.Number == pg.wallet.MixedAccountNumber() {
			pg.mixedBalance = dcrutil.Amount(acct.TotalBalance)
		} else if acct.Number == pg.wallet.UnmixedAccountNumber() {
			pg.unmixedBalance = dcrutil.Amount(acct.TotalBalance)
		}
	}

	pg.ArrMixerAccounts = vm
}

func (pg *AccountMixerPage) bottomSectionLabel(clickable *cryptomaterial.Clickable, title string) layout.Widget {
	return func(gtx C) D {
		return clickable.Layout(gtx, func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding15,
				Bottom: values.MarginPadding4,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(pg.Theme.Body1(title).Layout),
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, pg.Theme.Icons.ChevronRight.Layout24dp)
					}),
				)
			})
		})
	}
}

func (pg *AccountMixerPage) mixerProgressBarLayout(gtx C) D {
	totalAmount := (pg.mixedBalance + pg.unmixedBalance).ToCoin()
	pacentage := (pg.mixedBalance.ToCoin() / totalAmount) * 100

	items := []cryptomaterial.ProgressBarItem{
		{
			Value: pg.mixedBalance.ToCoin(),
			Color: pg.Theme.Color.Success,
			Label: pg.Theme.Label(values.TextSize14, fmt.Sprintf("%v%% Mixed", int(pacentage))),
		},
		{
			Value: pg.unmixedBalance.ToCoin(),
			Color: pg.Theme.Color.Gray7,
			Label: pg.Theme.Label(values.TextSize14, ""),
		},
	}

	labelWdg := func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return components.LayoutIconAndText(pg.Load, gtx, "", pg.mixedBalance.String(), items[0].Color)
				}),
				layout.Rigid(func(gtx C) D {
					return components.LayoutIconAndText(pg.Load, gtx, "", pg.unmixedBalance.String(), items[1].Color)
				}),
			)
		})
	}

	pb := pg.Theme.MultiLayerProgressBar(totalAmount, items)
	pb.ShowOverLayValue = true
	pb.Height = values.MarginPadding18
	return pb.Layout(gtx, labelWdg)
}

func (pg *AccountMixerPage) mixerHeaderContent() layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(pg.Theme.Label(values.TextSize18, values.String(values.StrBalance)).Layout),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
								return components.LayoutBalanceWithUnitSize(gtx, pg.Load, pg.totalWalletBalance.String(), values.TextSize18)
							})
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.Theme.Label(values.TextSize18, values.String(values.StrMix)).Layout)
									}),
									layout.Rigid(pg.toggleMixer.Layout),
								)
							})
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Left:  values.MarginPadding10,
					Right: values.MarginPadding10,
				}.Layout(gtx, pg.Theme.Separator().Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.UniformInset(values.MarginPadding22).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							txt := pg.Theme.Label(values.TextSize18, values.String(values.StrMixer))
							txt.Color = pg.Theme.Color.GrayText3
							return txt.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: values.MarginPadding16}.Layout(gtx, pg.mixerProgressBarLayout)
						}),
					)
				})
			}),
		)
	})
}

func (pg *AccountMixerPage) balanceInfo(balanceLabel, balanceValue string, balanceIcon *cryptomaterial.Image) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		leftWg := func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(balanceIcon.Layout12dp),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: values.MarginPadding8}.Layout(gtx, pg.Theme.Label(values.TextSize18, balanceLabel).Layout)
				}),
			)
		}

		return components.EndToEndRow(gtx, leftWg, func(gtx C) D {
			return components.LayoutBalanceWithUnitSize(gtx, pg.Load, balanceValue, values.TextSize18)
		})
	})
}

func (pg *AccountMixerPage) mixerImage() layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Flexed(4, pg.Theme.Separator().Layout),
			layout.Flexed(2, func(gtx C) D {
				return layout.Center.Layout(gtx, pg.Theme.Icons.MixerIcon.Layout36dp)
			}),
			layout.Flexed(4, pg.Theme.Separator().Layout),
		)
	})
}

func (pg *AccountMixerPage) mixerSettings(l *load.Load) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding10, Right: values.MarginPadding10, Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
					return l.Theme.Separator().Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
					return pg.settingsCollapsible.Layout(gtx,
						func(gtx C) D {
							txt := pg.Theme.Label(values.TextSize16, values.String(values.StrSettings))
							txt.Color = pg.Theme.Color.GrayText3
							return txt.Layout(gtx)
						},
						func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(pg.bottomSectionLabel(pg.mixedAccount, values.String(values.StrMixedAccount))),
									layout.Rigid(pg.bottomSectionLabel(pg.unmixedAccount, values.String(values.StrUnmixedAccount))),
									layout.Rigid(pg.bottomSectionLabel(pg.coordinationServer, values.String(values.StrCoordinationServer))),
								)
							})
						},
					)
				})
			}),
		)
	})
}

func (pg *AccountMixerPage) mixerPageLayout(gtx C) D {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		wdg := func(gtx C) D {
			return layout.UniformInset(values.MarginPadding25).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					pg.mixerHeaderContent(),
					pg.balanceInfo(values.String(values.StrMixed), pg.mixedBalance.String(), pg.Theme.Icons.MixedTxIcon),
					pg.mixerImage(),
					pg.balanceInfo(values.String(values.StrUnmixed), pg.unmixedBalance.String(), pg.Theme.Icons.UnmixedTxIcon),
					pg.mixerSettings(pg.Load),
				)
			})
		}

		return pg.pageContainer.Layout(gtx, 1, func(gtx C, i int) D {
			return wdg(gtx)
		})
	})
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *AccountMixerPage) Layout(gtx layout.Context) layout.Dimensions {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *AccountMixerPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return components.UniformPadding(gtx, func(gtx C) D {
		in := values.MarginPadding50
		return layout.Inset{
			Top:    values.MarginPadding25,
			Left:   in,
			Right:  in,
			Bottom: in,
		}.Layout(gtx, pg.mixerPageLayout)
	})
}

func (pg *AccountMixerPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	return D{}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *AccountMixerPage) HandleUserInteractions() {
	if pg.toggleMixer.Changed() {
		if pg.toggleMixer.IsChecked() {
			if pg.unmixedBalance.ToCoin() <= 0 {
				pg.Toast.NotifyError(values.String(values.StrNoMixable))
				pg.toggleMixer.SetChecked(false)
				return
			}
			pg.showModalPasswordStartAccountMixer()
		} else {
			pg.toggleMixer.SetChecked(true)
			info := modal.NewCustomModal(pg.Load).
				Title(values.String(values.StrCancelMixer)).
				Body(values.String(values.StrSureToCancelMixer)).
				SetNegativeButtonText(values.String(values.StrNo)).
				SetNegativeButtonCallback(func() {}).
				SetPositiveButtonText(values.String(values.StrYes)).
				SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
					pg.toggleMixer.SetChecked(false)
					go pg.WL.SelectedWallet.Wallet.StopAccountMixer()
					return true
				})
			pg.ParentWindow().ShowModal(info)
		}
	}

	if pg.mixerCompleted {
		pg.toggleMixer.SetChecked(false)
		pg.mixerCompleted = false
		pg.ParentWindow().Reload()
	}

	// get account number for the selected wallet name
	acctNum := func(val string) int32 {
		num, err := pg.wallet.AccountNumber(val)
		if err != nil {
			log.Error(err.Error())
			return -1
		}
		return num
	}

	for pg.mixedAccount.Clicked() {
		name, err := pg.wallet.AccountName(pg.wallet.MixedAccountNumber())
		if err != nil {
			log.Error(err.Error())
		}

		subtitle := func(gtx C) D {
			text := values.StringF(values.StrSelectMixedAcc, `<span style="text-color: text">`, `<span style="font-weight: bold">`, `</span><span style="text-color: danger">`, `</span></span>`)
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(renderers.RenderHTML(text, pg.Theme).Layout),
			)
		}

		selectMixedAccModal := preference.NewListPreference(pg.Load, "", name, pg.ArrMixerAccounts).
			UseCustomWidget(subtitle).
			IsWallet(true).
			UpdateValues(func(val string) {
				if acctNum(val) != -1 {
					pg.wallet.SetInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, acctNum(val))
				}
			})
		pg.ParentWindow().ShowModal(selectMixedAccModal)
	}

	for pg.unmixedAccount.Clicked() {
		name, err := pg.wallet.AccountName(pg.wallet.UnmixedAccountNumber())
		if err != nil {
			log.Error(err.Error())
		}

		subtitle := func(gtx C) D {
			text := values.StringF(values.StrSelectChangeAcc, `<span style="text-color: text">`, `<span style="font-weight: bold">`, `</span><span style="text-color: danger">`, `</span></span>`)
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(renderers.RenderHTML(text, pg.Theme).Layout),
			)
		}
		selectChangeAccModal := preference.NewListPreference(pg.Load, "", name, pg.ArrMixerAccounts).
			UseCustomWidget(subtitle).
			IsWallet(true).
			UpdateValues(func(val string) {
				if acctNum(val) != -1 {
					pg.wallet.SetInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, acctNum(val))
				}
			})
		pg.ParentWindow().ShowModal(selectChangeAccModal)
	}

	for pg.coordinationServer.Clicked() {
		textModal := modal.NewTextInputModal(pg.Load).
			Hint(values.String(values.StrCoordinationServer)).
			PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
			SetPositiveButtonCallback(func(newName string, tim *modal.TextInputModal) bool {
				// Todo - implement custom CSPP server
				return true
			})

		textModal.SetNegativeButtonCallback(func() {}).
			SetPositiveButtonText(values.String(values.StrSave))

		pg.ParentWindow().ShowModal(textModal)
	}
}

func (pg *AccountMixerPage) showModalPasswordStartAccountMixer() {
	passwordModal := modal.NewPasswordModal(pg.Load).
		Title(values.String(values.StrConfirmToMixAccount)).
		NegativeButton(values.String(values.StrCancel), func() {
			pg.toggleMixer.SetChecked(false)
		}).
		PositiveButton(values.String(values.StrConfirm), func(password string, pm *modal.PasswordModal) bool {
			go func() {
				err := pg.WL.SelectedWallet.Wallet.StartAccountMixer(password)
				if err != nil {
					pg.Toast.NotifyError(err.Error())
					pm.SetLoading(false)
					return
				}
				pm.Dismiss()
			}()

			return false
		})
	pg.ParentWindow().ShowModal(passwordModal)
}

func (pg *AccountMixerPage) listenForMixerNotifications() {
	if pg.AccountMixerNotificationListener != nil {
		return
	}

	if pg.TxAndBlockNotificationListener != nil {
		return
	}

	pg.AccountMixerNotificationListener = listeners.NewAccountMixerNotificationListener()
	err := pg.WL.SelectedWallet.Wallet.AddAccountMixerNotificationListener(pg, AccountMixerPageID)
	if err != nil {
		log.Errorf("Error adding account mixer notification listener: %+v", err)
		return
	}

	pg.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err = pg.WL.SelectedWallet.Wallet.AddTxAndBlockNotificationListener(pg.TxAndBlockNotificationListener, true, AccountMixerPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.MixerChan:
				if n.RunStatus == wallet.MixerStarted {
					pg.Toast.Notify(values.String(values.StrMixerStart))
					pg.getMixerBalance()
					pg.ParentWindow().Reload()
				}

				if n.RunStatus == wallet.MixerEnded {
					pg.mixerCompleted = true
					pg.getMixerBalance()
					pg.ParentWindow().Reload()
				}
			// this is needed to refresh the UI on every block
			case n := <-pg.TxAndBlockNotifChan:
				if n.Type == listeners.BlockAttached {
					pg.getMixerBalance()
					pg.ParentWindow().Reload()
				}

			case <-pg.ctx.Done():
				pg.WL.SelectedWallet.Wallet.RemoveTxAndBlockNotificationListener(AccountMixerPageID)
				pg.WL.SelectedWallet.Wallet.RemoveAccountMixerNotificationListener(AccountMixerPageID)
				close(pg.MixerChan)
				close(pg.TxAndBlockNotifChan)
				pg.AccountMixerNotificationListener = nil
				return
			}
		}
	}()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *AccountMixerPage) OnNavigatedFrom() {
	pg.ctxCancel()
}
