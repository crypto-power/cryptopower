package receive

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
	qrcode "github.com/yeqown/go-qrcode"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

const ReceivePageID = "Receive"

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
	modalLayout *cryptomaterial.Modal

	pageContainer         layout.List
	scrollContainer       *widget.List
	isNewAddr             bool
	currentAddress        string
	qrImage               *image.Image
	newAddr, copy         *cryptomaterial.Clickable
	info                  cryptomaterial.IconButton
	card                  cryptomaterial.Card
	sourceAccountselector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	isCopying         bool
	backdrop          *widget.Clickable
	infoButton        cryptomaterial.IconButton
	selectedWallet    sharedW.Asset
	navigateToSyncBtn cryptomaterial.Button
}

func NewReceivePage(l *load.Load, wallet sharedW.Asset) *Page {
	pg := &Page{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ReceivePageID),
		pageContainer: layout.List{
			Axis: layout.Vertical,
		},
		scrollContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		info:              l.Theme.IconButton(cryptomaterial.MustIcon(widget.NewIcon(icons.ActionInfo))),
		copy:              l.Theme.NewClickable(false),
		newAddr:           l.Theme.NewClickable(false),
		card:              l.Theme.Card(),
		backdrop:          new(widget.Clickable),
		navigateToSyncBtn: l.Theme.Button(values.String(values.StrStartSync)),
		selectedWallet:    wallet,
	}

	pg.info.Inset, pg.info.Size = layout.UniformInset(values.MarginPadding5), values.MarginPadding20

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	if wallet == nil {
		pg.modalLayout = l.Theme.ModalFloatTitle(values.String(values.StrReceive), pg.IsMobileView(), nil)
		pg.GenericPageModal = pg.modalLayout.GenericPageModal
		pg.initWalletSelectors() // will auto select the first wallet in the dropdown as pg.selectedWallet
	} else {
		pg.sourceAccountselector = components.NewWalletAndAccountSelector(pg.Load).
			Title(values.String(values.StrTo)).
			AccountSelected(func(selectedAccount *sharedW.Account) {
				currentAddress, err := pg.selectedWallet.CurrentAddress(selectedAccount.Number)
				if err != nil {
					log.Errorf("Error getting current address: %v", err)
				} else {
					pg.currentAddress = currentAddress
				}

				pg.generateQRForAddress()
			}).
			AccountValidator(func(account *sharedW.Account) bool {
				// Filter out imported account and mixed.
				if account.Number == load.MaxInt32 {
					return false
				}
				if account.Number != load.MixedAccountNumber(pg.selectedWallet) {
					return true
				}
				return false
			})
		_ = pg.sourceAccountselector.SelectFirstValidAccount(pg.selectedWallet)
	}

	return pg
}

func (pg *Page) initWalletSelectors() {
	// Source wallet picker
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrSelectWallet))
	pg.selectedWallet = pg.sourceWalletSelector.SelectedWallet()

	// Source account picker
	pg.sourceAccountselector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrSelectAcc)).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32

			return accountIsValid
		})
	_ = pg.sourceAccountselector.SelectFirstValidAccount(pg.sourceWalletSelector.SelectedWallet())

	pg.sourceWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
		pg.selectedWallet = selectedWallet
		_ = pg.sourceAccountselector.SelectFirstValidAccount(selectedWallet)
		pg.generateQRForAddress()
	})

	pg.sourceAccountselector.AccountSelected(func(_ *sharedW.Account) {
		pg.generateQRForAddress()
	})

	pg.generateQRForAddress()
}

// OnResume is called to initialize data and get UI elements ready to be
// displayed. This is called just before Handle() and Layout() are called (in
// that order).

// OnResume is like OnNavigatedTo but OnResume is called if this page is
// displayed as a modal while OnNavigatedTo is called if this page is displayed
// as a full page. Either OnResume or OnNavigatedTo is called to initialize
// data and get UI elements ready to be displayed. This is called just before
// Handle() and Layout() are called (in that order).
func (pg *Page) OnResume() {
	pg.OnNavigatedTo()
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedTo() {
	if !pg.selectedWallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.sourceAccountselector.ListenForTxNotifications(pg.ParentWindow())    // listener is stopped in OnNavigatedFrom()
	_ = pg.sourceAccountselector.SelectFirstValidAccount(pg.selectedWallet) // Want to reset the user's selection everytime this page appears?
	// might be better to track the last selection in a variable and reselect it.
	currentAddress, err := pg.selectedWallet.CurrentAddress(pg.sourceAccountselector.SelectedAccount().Number)
	if err != nil {
		errStr := fmt.Sprintf("Error getting current address: %v", err)
		errModal := modal.NewErrorModal(pg.Load, errStr, modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
	} else {
		pg.currentAddress = currentAddress
		pg.generateQRForAddress()
	}
}

func (pg *Page) generateQRForAddress() {
	qrCode, err := qrcode.New(pg.currentAddress, qrcode.WithLogoImage(pg.getSelectedWalletLogo()))
	if err != nil {
		log.Error("Error generating address qrCode: " + err.Error())
		return
	}

	var buff bytes.Buffer
	err = qrCode.SaveTo(&buff)
	if err != nil {
		log.Error(err.Error())
		return
	}

	imgdec, _, err := image.Decode(bytes.NewReader(buff.Bytes()))
	if err != nil {
		log.Error(err.Error())
		return
	}

	pg.qrImage = &imgdec
}

func (pg *Page) getSelectedWalletLogo() *cryptomaterial.Image {
	pg.selectedWallet.GetAssetType()
	switch pg.selectedWallet.GetAssetType() {
	case utils.BTCWalletAsset:
		return pg.Theme.Icons.CircleBTC
	case utils.DCRWalletAsset:
		return pg.Theme.Icons.CircleDCR
	case utils.LTCWalletAsset:
		return pg.Theme.Icons.CircleLTC
	default:
		return nil
	}
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx C) D {
	if pg.modalLayout == nil {
		return pg.contentLayout(gtx)
	}
	var modalWidth float32 = 450
	if pg.IsMobileView() {
		modalWidth = 0
	}
	modalContent := []layout.Widget{pg.contentLayout}
	return pg.modalLayout.Layout(gtx, modalContent, modalWidth)
}

func (pg *Page) contentLayout(gtx C) D {
	pg.handleCopyEvent(gtx)
	pg.pageBackdropLayout(gtx)
	return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, _ int) D {
		textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
		uniformSize := values.MarginPadding16
		if pg.modalLayout != nil {
			uniformSize = values.MarginPadding0
		}
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return layout.UniformInset(values.MarginPaddingTransform(pg.IsMobileView(), uniformSize)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(pg.headerLayout),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout),
					layout.Rigid(func(gtx C) D {
						if pg.modalLayout == nil {
							return D{}
						}
						lbl := pg.Theme.Label(textSize16, values.String(values.StrDestinationWallet))
						lbl.Font.Weight = font.Bold
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if pg.modalLayout == nil {
							return D{}
						}
						return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return pg.sourceWalletSelector.Layout(pg.ParentWindow(), gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						lbl := pg.Theme.Label(textSize16, values.String(values.StrAccount))
						lbl.Font.Weight = font.Bold
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return pg.sourceAccountselector.Layout(pg.ParentWindow(), gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return components.VerticalInset(values.MarginPadding24).Layout(gtx, pg.Theme.Separator().Layout)
					}),
					layout.Rigid(func(gtx C) D {
						if !pg.selectedWallet.IsWatchingOnlyWallet() {
							return D{}
						}
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						warning := pg.Theme.Label(textSize16, values.String(values.StrWarningWatchWallet))
						warning.Color = pg.Theme.Color.Danger
						return layout.Center.Layout(gtx, warning.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						if !pg.selectedWallet.IsSynced() && pg.modalLayout != nil {
							// If wallet is not synced, display a message and don't display the sections
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.Center.Layout(gtx, func(gtx C) D {
								widgets := []func(gtx C) D{
									func(gtx C) D {
										warning := pg.Theme.Label(textSize16, values.String(values.StrFunctionUnavailable))
										warning.Color = pg.Theme.Color.Danger
										warning.Alignment = text.Middle
										return warning.Layout(gtx)

									},
									func(gtx C) D {
										if pg.selectedWallet.IsSyncing() {
											syncInfo := components.NewWalletSyncInfo(pg.Load, pg.selectedWallet, func() {}, func(_ sharedW.Asset) {})
											blockHeightFetched := values.StringF(values.StrBlockHeaderFetchedCount, pg.selectedWallet.GetBestBlock().Height, syncInfo.FetchSyncProgress().HeadersToFetchOrScan)
											text := fmt.Sprintf("%s "+blockHeightFetched, values.String(values.StrBlockHeaderFetched))
											blockInfo := pg.Theme.Label(textSize16, text)
											return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, blockInfo.Layout)
										}

										return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, pg.navigateToSyncBtn.Layout)
									},
								}
								options := components.FlexOptions{
									Axis:      layout.Vertical,
									Alignment: layout.Middle,
								}
								return components.FlexLayout(gtx, options, widgets)
							})
						}
						// If wallet is synced, display the original sections
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								txt := pg.Theme.Body2(values.String(values.StrMyAddress))
								txt.TextSize = values.TextSize16
								txt.Color = pg.Theme.Color.GrayText2
								return txt.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout),
							layout.Rigid(func(gtx C) D {
								if pg.qrImage == nil {
									return D{}
								}
								return pg.Theme.ImageIcon(gtx, *pg.qrImage, 150)
							}),
							layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout),
							layout.Rigid(pg.addressLayout),
							layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout),
							layout.Rigid(pg.copyAndNewAddressLayout),
						)
					}),
				)
			})
		})
	})
}

func (pg *Page) copyAndNewAddressLayout(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return pg.buttonIconLayout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.CopyIcon), values.String(values.StrCopy), pg.copy)
			}),
			layout.Rigid(layout.Spacer{Width: values.MarginPadding32}.Layout),
			layout.Rigid(func(gtx C) D {
				return pg.buttonIconLayout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.NavigationRefresh), values.String(values.StrRegenerate), pg.newAddr)
			}),
		)
	})
}

func (pg *Page) buttonIconLayout(gtx C, icon *cryptomaterial.Icon, text string, clickable *cryptomaterial.Clickable) D {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			dp40 := gtx.Dp(values.MarginPadding40)
			return cryptomaterial.LinearLayout{
				Width:       dp40,
				Height:      dp40,
				Background:  pg.Theme.Color.Gray2,
				Orientation: layout.Horizontal,
				Direction:   layout.Center,
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.Radius(20),
				},
				Clickable: clickable,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding10, Bottom: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
				}),
			)
		}),
		layout.Rigid(pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize14), text).Layout),
	)
}

// pageBackdropLayout layout of background overlay when the popup button generate new address is show,
// click outside of the generate new address button to hide the button
func (pg *Page) pageBackdropLayout(gtx C) {
	if pg.isNewAddr {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		m := op.Record(gtx.Ops)
		pg.backdrop.Layout(gtx, func(gtx C) D {
			semantic.Button.Add(gtx.Ops)
			return D{Size: gtx.Constraints.Min}
		})
		op.Defer(gtx.Ops, m.Stop())
	}
}

func (pg *Page) headerLayout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.H6(values.String(values.StrReceive))
			lbl.TextSize = values.TextSizeTransform(pg.IsMobileView(), values.TextSize20)
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding6}.Layout(gtx, pg.infoButton.Layout)
		}),
	)
}

func (pg *Page) addressLayout(gtx C) D {
	border := widget.Border{
		Color:        pg.Theme.Color.Gray4,
		CornerRadius: values.MarginPadding10,
		Width:        values.MarginPadding2,
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return border.Layout(gtx, func(gtx C) D {
		return components.VerticalInset(values.MarginPadding12).Layout(gtx, func(gtx C) D {
			lbl := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), "")
			if pg.currentAddress != "" && pg.selectedWallet.IsSynced() {
				lbl.Text = pg.currentAddress
			}
			return layout.Center.Layout(gtx, lbl.Layout)
		})
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Page) HandleUserInteractions(gtx C) {
	if pg.backdrop.Clicked(gtx) {
		pg.isNewAddr = false
	}

	if pg.newAddr.Clicked(gtx) {
		newAddr, err := pg.generateNewAddress()
		if err != nil {
			log.Debug("Error generating new address" + err.Error())
			return
		}

		pg.currentAddress = newAddr
		pg.generateQRForAddress()
		pg.isNewAddr = false
	}

	if pg.infoButton.Button.Clicked(gtx) {
		textWithUnit := values.String(values.StrReceive) + " " + string(pg.selectedWallet.GetAssetType())
		info := modal.NewCustomModal(pg.Load).
			Title(textWithUnit).
			Body(values.String(values.StrReceiveInfo)).
			SetContentAlignment(layout.NW, layout.W, layout.Center)
		pg.ParentWindow().ShowModal(info)
	}

	if pg.navigateToSyncBtn.Button.Clicked(gtx) {
		pg.ToggleSync(pg.selectedWallet, func(b bool) {
			pg.selectedWallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, b)
		})
	}
}

func (pg *Page) generateNewAddress() (string, error) {
	selectedAccount := pg.sourceAccountselector.SelectedAccount()
	selectedWallet := pg.AssetsManager.WalletWithID(selectedAccount.WalletID)

generateAddress:
	newAddr, err := selectedWallet.NextAddress(selectedAccount.Number)
	if err != nil {
		return "", err
	}

	if newAddr == pg.currentAddress {
		goto generateAddress
	}

	return newAddr, nil
}

func (pg *Page) handleCopyEvent(gtx C) {
	// Prevent copying again if the timer hasn't expired
	if pg.copy.Clicked(gtx) && !pg.isCopying {
		gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(pg.currentAddress))})
		pg.Toast.Notify(values.String(values.StrCopied))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Page) OnNavigatedFrom() {
	pg.sourceAccountselector.StopTxNtfnListener()
}

func (pg *Page) Handle(gtx C) {
	if pg.modalLayout.BackdropClicked(gtx, true) {
		pg.modalLayout.Dismiss()
	} else {
		pg.HandleUserInteractions(gtx)
	}
}

// OnDismiss is like OnNavigatedFrom but OnDismiss is called if this page is
// displayed as a modal while OnNavigatedFrom is called if this page is
// displayed as a full page. Either OnDismiss or OnNavigatedFrom is called
// after the modal is dismissed.
// NOTE: The modal may be re-displayed on the app's window, in which case
// OnResume() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnResume() method.
func (pg *Page) OnDismiss() {
	pg.OnNavigatedFrom()
}
