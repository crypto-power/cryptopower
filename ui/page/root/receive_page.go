package root

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	qrcode "github.com/yeqown/go-qrcode"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/values"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

const ReceivePageID = "Receive"

type ReceivePage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	multiWallet       *libwallet.MultiWallet
	pageContainer     layout.List
	scrollContainer   *widget.List
	isNewAddr, isInfo bool
	currentAddress    string
	qrImage           *image.Image
	newAddr, copy     cryptomaterial.Button
	info, more        cryptomaterial.IconButton
	card              cryptomaterial.Card
	receiveAddress    cryptomaterial.Label
	ops               *op.Ops
	selector          *components.AccountSelector
	copyAddressButton cryptomaterial.Button

	isCopying  bool
	backdrop   *widget.Clickable
	infoButton cryptomaterial.IconButton
}

func NewReceivePage(l *load.Load) *ReceivePage {
	pg := &ReceivePage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ReceivePageID),
		multiWallet:      l.WL.MultiWallet,
		pageContainer: layout.List{
			Axis: layout.Vertical,
		},
		scrollContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		info:           l.Theme.IconButton(cryptomaterial.MustIcon(widget.NewIcon(icons.ActionInfo))),
		copy:           l.Theme.Button(values.String(values.StrCopy)),
		more:           l.Theme.IconButton(l.Theme.Icons.NavigationMore),
		newAddr:        l.Theme.Button(values.String(values.StrGenerateAddress)),
		receiveAddress: l.Theme.Label(values.TextSize20, ""),
		card:           l.Theme.Card(),
		backdrop:       new(widget.Clickable),
	}

	pg.info.Inset, pg.info.Size = layout.UniformInset(values.MarginPadding5), values.MarginPadding20
	pg.copy.Background = pg.Theme.Color.Primary
	pg.copy.HighlightColor = pg.Theme.Color.SurfaceHighlight
	pg.copy.Color = pg.Theme.Color.Surface
	pg.copy.Inset = layout.UniformInset(values.MarginPadding10)
	pg.more.Inset = layout.UniformInset(values.MarginPadding0)
	pg.newAddr.Inset = layout.UniformInset(values.MarginPadding10)
	pg.newAddr.Color = pg.Theme.Color.Text
	pg.newAddr.Background = pg.Theme.Color.Surface
	pg.newAddr.HighlightColor = pg.Theme.Color.SurfaceHighlight
	pg.newAddr.ButtonStyle.TextSize = values.TextSize14
	pg.newAddr.ButtonStyle.Font.Weight = text.SemiBold

	pg.receiveAddress.MaxLines = 1

	_, pg.infoButton = components.SubpageHeaderButtons(l)

	pg.copyAddressButton = l.Theme.OutlineButton("")
	pg.copyAddressButton.TextSize = values.TextSize14
	pg.copyAddressButton.Inset = layout.UniformInset(values.MarginPadding0)

	pg.selector = components.NewAccountSelector(pg.Load).
		Title(values.String(values.StrFrom)).
		AccountSelected(func(selectedAccount *libwallet.Account) {
			selectedWallet := pg.multiWallet.WalletWithID(selectedAccount.WalletID)
			currentAddress, err := selectedWallet.CurrentAddress(selectedAccount.Number)
			if err != nil {
				log.Errorf("Error getting current address: %v", err)
			} else {
				pg.currentAddress = currentAddress
			}

			pg.generateQRForAddress()
		}).
		AccountValidator(func(account *libwallet.Account) bool {

			// Filter out imported account and mixed.
			wal := pg.multiWallet.WalletWithID(account.WalletID)
			if account.Number == load.MaxInt32 ||
				account.Number == wal.MixedAccountNumber() {
				return false
			}
			return true
		})

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ReceivePage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	pg.selector.ListenForTxNotifications(pg.ctx, pg.ParentWindow())
	pg.selector.SelectFirstWalletValidAccount() // Want to reset the user's selection everytime this page appears?
	// might be better to track the last selection in a variable and reselect it.
	currentAddress, err := pg.WL.SelectedWallet.Wallet.CurrentAddress(pg.selector.SelectedAccount().Number)
	if err != nil {
		errStr := fmt.Sprintf("Error getting current address: %v", err)
		errModal := modal.NewErrorModal(pg.Load, errStr, modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
	} else {
		pg.currentAddress = currentAddress
		pg.generateQRForAddress()
	}
}

func (pg *ReceivePage) generateQRForAddress() {
	qrCode, err := qrcode.New(pg.currentAddress)
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

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ReceivePage) Layout(gtx C) D {
	pg.handleCopyEvent(gtx)
	pg.pageBackdropLayout(gtx)

	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *ReceivePage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return pg.pageSections(gtx, func(gtx C) D {
				return pg.selector.Layout(pg.ParentWindow(), gtx)
			})
		},
		func(gtx C) D {
			return pg.Theme.Separator().Layout(gtx)
		},
		func(gtx C) D {
			return pg.pageSections(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return pg.titleLayout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							return layout.Flex{
								Axis:      layout.Vertical,
								Alignment: layout.Middle,
							}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if pg.currentAddress != "" {
										return pg.addressLayout(gtx)
									}
									return D{}
								}),
								layout.Rigid(func(gtx C) D {
									if pg.qrImage == nil {
										return D{}
									}

									return pg.Theme.ImageIcon(gtx, *pg.qrImage, 180)
								}),
							)
						})
					}),
				)
			})
		},
	}

	dims := components.UniformPadding(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.topNav(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
					return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Card().Layout(gtx, func(gtx C) D {
							return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
								return pageContent[i](gtx)
							})
						})
					})
				})
			}),
		)
	})

	return dims
}

func (pg *ReceivePage) layoutMobile(gtx layout.Context) layout.Dimensions {
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return pg.pageSections(gtx, func(gtx C) D {
				return pg.selector.Layout(pg.ParentWindow(), gtx)
			})
		},
		func(gtx C) D {
			return pg.Theme.Separator().Layout(gtx)
		},
		func(gtx C) D {
			return pg.pageSections(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return pg.titleLayout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							return layout.Flex{
								Axis:      layout.Vertical,
								Alignment: layout.Middle,
							}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if pg.qrImage == nil {
										return layout.Dimensions{}
									}

									return pg.Theme.ImageIcon(gtx, *pg.qrImage, 500)
								}),
								layout.Rigid(func(gtx C) D {
									if pg.currentAddress != "" {
										pg.copyAddressButton.Text = pg.currentAddress
										return pg.copyAddressButton.Layout(gtx)
									}
									return layout.Dimensions{}
								}),
								layout.Rigid(func(gtx C) D {
									tapToCopy := pg.Theme.Label(values.TextSize10, "(Tap to copy)")
									tapToCopy.Color = pg.Theme.Color.Text
									return tapToCopy.Layout(gtx)
								}),
							)
						})
					}),
				)
			})
		},
	}

	dims := components.UniformMobile(gtx, false, true, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: values.MarginPadding16, Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
					return pg.topNav(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
					return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Card().Layout(gtx, func(gtx C) D {
							return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
								return pageContent[i](gtx)
							})
						})
					})
				})
			}),
		)
	})

	return dims
}

func (pg *ReceivePage) pageSections(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(values.MarginPadding16).Layout(gtx, body)
	})
}

// pageBackdropLayout layout of background overlay when the popup button generate new address is show,
// click outside of the generate new address button to hide the button
func (pg *ReceivePage) pageBackdropLayout(gtx C) {
	if pg.isNewAddr {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		m := op.Record(gtx.Ops)
		pg.backdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		})
		op.Defer(gtx.Ops, m.Stop())
	}
}

func (pg *ReceivePage) topNav(gtx C) D {
	m := values.MarginPadding0
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: m}.Layout(gtx, pg.Theme.H6(values.String(values.StrReceive)+" DCR").Layout)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, pg.infoButton.Layout)
		}),
	)
}

func (pg *ReceivePage) titleLayout(gtx C) D {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := pg.Theme.Body2(values.String(values.StrYourAddress))
			txt.Color = pg.Theme.Color.GrayText2
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if pg.isNewAddr {
						m := op.Record(gtx.Ops)
						layout.Inset{Top: values.MarginPadding30, Left: unit.Dp(-152)}.Layout(gtx, func(gtx C) D {
							return pg.Theme.Shadow().Layout(gtx, pg.newAddr.Layout)
						})
						op.Defer(gtx.Ops, m.Stop())
					}
					return D{}
				}),
				layout.Rigid(pg.more.Layout),
			)
		}),
	)
}

func (pg *ReceivePage) addressLayout(gtx C) D {
	return layout.Inset{Top: values.MarginPadding14, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				card := cryptomaterial.Card{Color: pg.Theme.Color.Gray4}
				card.Radius = cryptomaterial.CornerRadius{TopRight: 0, TopLeft: 8, BottomRight: 0, BottomLeft: 8}
				return card.Layout(gtx, func(gtx C) D {
					return layout.Inset{
						Top:    values.MarginPadding8,
						Bottom: values.MarginPadding8,
						Left:   values.MarginPadding30,
						Right:  values.MarginPadding30,
					}.Layout(gtx, func(gtx C) D {
						pg.receiveAddress.Text = pg.currentAddress
						return pg.receiveAddress.Layout(gtx)
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, func(gtx C) D { return D{} })
			}),
			layout.Rigid(func(gtx C) D {
				card := cryptomaterial.Card{Color: pg.copy.Background}
				card.Radius = cryptomaterial.CornerRadius{TopRight: 8, TopLeft: 0, BottomRight: 8, BottomLeft: 0}
				return card.Layout(gtx, pg.copy.Layout)
			}),
		)
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ReceivePage) HandleUserInteractions() {
	if pg.backdrop.Clicked() {
		pg.isNewAddr = false
	}

	if pg.more.Button.Clicked() {
		pg.isNewAddr = !pg.isNewAddr
		if pg.isInfo {
			pg.isInfo = false
		}
	}

	if pg.newAddr.Clicked() {
		newAddr, err := pg.generateNewAddress()
		if err != nil {
			log.Debug("Error generating new address" + err.Error())
			return
		}

		pg.currentAddress = newAddr
		pg.generateQRForAddress()
		pg.isNewAddr = false
	}

	if pg.infoButton.Button.Clicked() {
		info := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrReceive)+" DCR").
			Body(values.String(values.StrReceiveInfo)).
			SetContentAlignment(layout.NW, layout.Center)
		pg.ParentWindow().ShowModal(info)
	}
}

func (pg *ReceivePage) generateNewAddress() (string, error) {
	selectedAccount := pg.selector.SelectedAccount()
	selectedWallet := pg.multiWallet.WalletWithID(selectedAccount.WalletID)

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

func (pg *ReceivePage) handleCopyEvent(gtx C) {
	// Prevent copying again if the timer hasn't expired
	if pg.copy.Clicked() && !pg.isCopying {
		clipboard.WriteOp{Text: pg.currentAddress}.Add(gtx.Ops)

		pg.copy.Text = values.String(values.StrCopied)
		pg.copy.Background = pg.Theme.Color.Success
		pg.isCopying = true
		time.AfterFunc(time.Second*4, func() {
			pg.copy.Text = values.String(values.StrCopy)
			pg.copy.Background = pg.Theme.Color.Primary
			pg.isCopying = false
		})
	}

	if pg.copyAddressButton.Clicked() {
		clipboard.WriteOp{Text: pg.copyAddressButton.Text}.Add(gtx.Ops)
		pg.Toast.Notify("Copied")
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ReceivePage) OnNavigatedFrom() {
	pg.ctxCancel()
}
