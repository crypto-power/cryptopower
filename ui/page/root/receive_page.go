package root

import (
	"bytes"
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
	qrcode "github.com/yeqown/go-qrcode"
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

	pageContainer     layout.List
	scrollContainer   *widget.List
	isNewAddr, isInfo bool
	currentAddress    string
	qrImage           *image.Image
	newAddr, copy     cryptomaterial.Button
	newAddr1, copy1   *cryptomaterial.Clickable
	info, more        cryptomaterial.IconButton
	card              cryptomaterial.Card
	receiveAddress    cryptomaterial.Label
	copyLabel         cryptomaterial.Label
	selector          *components.WalletAndAccountSelector
	copyAddressButton cryptomaterial.Button

	isCopying      bool
	backdrop       *widget.Clickable
	infoButton     cryptomaterial.IconButton
	selectedWallet sharedW.Asset
}

func NewReceivePage(l *load.Load, wallet sharedW.Asset) *ReceivePage {
	pg := &ReceivePage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ReceivePageID),
		pageContainer: layout.List{
			Axis: layout.Vertical,
		},
		scrollContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		info:           l.Theme.IconButton(cryptomaterial.MustIcon(widget.NewIcon(icons.ActionInfo))),
		copy1:          l.Theme.NewClickable(false),
		newAddr1:       l.Theme.NewClickable(false),
		copy:           l.Theme.Button(values.String(values.StrCopy)),
		more:           l.Theme.IconButton(l.Theme.Icons.NavigationMore),
		newAddr:        l.Theme.Button(values.String(values.StrGenerateAddress)),
		receiveAddress: l.Theme.Label(values.TextSize16, ""),
		copyLabel:      l.Theme.Label(values.TextSize16, ""),
		card:           l.Theme.Card(),
		backdrop:       new(widget.Clickable),
		selectedWallet: wallet,
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
	pg.newAddr.ButtonStyle.Font.Weight = font.SemiBold

	pg.receiveAddress.MaxLines = 1

	_, pg.infoButton = components.SubpageHeaderButtons(l)

	pg.copyAddressButton = l.Theme.OutlineButton("")
	pg.copyAddressButton.TextSize = values.TextSize14
	pg.copyAddressButton.Inset = layout.UniformInset(values.MarginPadding0)

	pg.selector = components.NewWalletAndAccountSelector(pg.Load).
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
	pg.selector.SelectFirstValidAccount(pg.selectedWallet)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ReceivePage) OnNavigatedTo() {
	if !pg.selectedWallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.selector.ListenForTxNotifications(pg.ParentWindow()) // listener is stopped in OnNavigatedFrom()
	pg.selector.SelectFirstValidAccount(pg.selectedWallet)  // Want to reset the user's selection everytime this page appears?
	// might be better to track the last selection in a variable and reselect it.
	currentAddress, err := pg.selectedWallet.CurrentAddress(pg.selector.SelectedAccount().Number)
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

	// if pg.Load.IsMobileView() {
	// 	return pg.layoutMobile(gtx)
	// }
	return pg.layoutDesktop(gtx)
}

func (pg *ReceivePage) layoutDesktop(gtx C) D {
	// pageContent := []func(gtx C) D{
	// 	func(gtx C) D {
	// 		// return pg.pageSections(gtx, func(gtx C) D {
	// 		return pg.selector.Layout(pg.ParentWindow(), gtx)
	// 		// })
	// 	},
	// 	func(gtx C) D {
	// 		return pg.Theme.Separator().Layout(gtx)
	// 	},
	// 	func(gtx C) D {
	// 		return pg.pageSections(gtx, func(gtx C) D {
	// 			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
	// 				layout.Rigid(func(gtx C) D {
	// 					return pg.titleLayout(gtx)
	// 				}),
	// 				layout.Rigid(func(gtx C) D {
	// 					if pg.selectedWallet.IsWatchingOnlyWallet() {
	// 						warning := pg.Theme.Label(values.TextSize16, values.String(values.StrWarningWatchWallet))
	// 						warning.Color = pg.Theme.Color.Danger
	// 						return layout.Center.Layout(gtx, warning.Layout)
	// 					}
	// 					return D{}
	// 				}),
	// 				layout.Rigid(func(gtx C) D {
	// 					return layout.Center.Layout(gtx, func(gtx C) D {
	// 						return layout.Flex{
	// 							Axis:      layout.Vertical,
	// 							Alignment: layout.Middle,
	// 						}.Layout(gtx,
	// 							layout.Rigid(func(gtx C) D {
	// 								if pg.currentAddress != "" && pg.selectedWallet.IsSynced() {
	// 									// Display generated address only on a synced wallet
	// 									return pg.addressLayout(gtx)
	// 								}
	// 								return D{}
	// 							}),
	// 							layout.Rigid(func(gtx C) D {
	// 								if pg.qrImage == nil || !pg.selectedWallet.IsSynced() {
	// 									// Display generated address only on a synced wallet
	// 									return D{}
	// 								}

	// 								return pg.Theme.ImageIcon(gtx, *pg.qrImage, 180)
	// 							}),
	// 						)
	// 					})
	// 				}),
	// 			)
	// 		})
	// 	},
	// }
	return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return layout.UniformInset(values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding16)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(pg.headerLayout),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout),
					layout.Rigid(func(gtx C) D {
						textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
						lbl := pg.Theme.Label(textSize16, values.String(values.StrAccount))
						lbl.Font.Weight = font.Bold
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return pg.selector.Layout(pg.ParentWindow(), gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return components.VerticalInset(values.MarginPadding24).Layout(gtx, pg.Theme.Separator().Layout)
					}),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									txt := pg.Theme.Body2(values.String(values.StrMyAddress))
									txt.Color = pg.Theme.Color.GrayText2
									return txt.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout),
								layout.Rigid(func(gtx C) D {
									if pg.qrImage == nil || !pg.selectedWallet.IsSynced() {
										// Display generated address only on a synced wallet
										return D{}
									}
									return pg.Theme.ImageIcon(gtx, *pg.qrImage, 150)
								}),
							)
						})
					}),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout),
					layout.Rigid(pg.addressLayout),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout),
					layout.Rigid(pg.copyAndNewAddressLayout),
				)
			})
		})
	})
}

func (pg *ReceivePage) copyAndNewAddressLayout(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return pg.buttonIconLayout(gtx, pg.Theme.Icons.CopyIcon, values.String(values.StrCopy), pg.copy1)
			}),
			layout.Rigid(layout.Spacer{Width: values.MarginPadding32}.Layout),
			layout.Rigid(func(gtx C) D {
				return pg.buttonIconLayout(gtx, pg.Theme.Icons.Restore, values.String(values.StrRegenerate), pg.newAddr1)
			}),
		)
	})
}

func (pg *ReceivePage) buttonIconLayout(gtx C, icon *cryptomaterial.Image, text string, clickable *cryptomaterial.Clickable) D {
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

// func (pg *ReceivePage) layoutMobile(gtx C) D {
// 	pageContent := []func(gtx C) D{
// 		func(gtx C) D {
// 			return pg.pageSections(gtx, func(gtx C) D {
// 				return pg.selector.Layout(pg.ParentWindow(), gtx)
// 			})
// 		},
// 		func(gtx C) D {
// 			return pg.Theme.Separator().Layout(gtx)
// 		},
// 		func(gtx C) D {
// 			return pg.pageSections(gtx, func(gtx C) D {
// 				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 					layout.Rigid(func(gtx C) D {
// 						return pg.titleLayout(gtx)
// 					}),
// 					layout.Rigid(func(gtx C) D {
// 						return layout.Center.Layout(gtx, func(gtx C) D {
// 							return layout.Flex{
// 								Axis:      layout.Vertical,
// 								Alignment: layout.Middle,
// 							}.Layout(gtx,
// 								layout.Rigid(func(gtx C) D {
// 									if pg.qrImage == nil {
// 										return D{}
// 									}

// 									return pg.Theme.ImageIcon(gtx, *pg.qrImage, 500)
// 								}),
// 								layout.Rigid(func(gtx C) D {
// 									if pg.currentAddress != "" {
// 										pg.copyAddressButton.Text = pg.currentAddress
// 										return pg.copyAddressButton.Layout(gtx)
// 									}
// 									return D{}
// 								}),
// 								layout.Rigid(func(gtx C) D {
// 									tapToCopy := pg.Theme.Label(values.TextSize10, values.String(values.StrTapToCopy))
// 									tapToCopy.Color = pg.Theme.Color.Text
// 									return tapToCopy.Layout(gtx)
// 								}),
// 							)
// 						})
// 					}),
// 				)
// 			})
// 		},
// 	}

// 	dims := components.UniformMobile(gtx, false, true, func(gtx C) D {
// 		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 			layout.Rigid(func(gtx C) D {
// 				return layout.Inset{Bottom: values.MarginPadding16, Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
// 					return pg.headerLayout(gtx)
// 				})
// 			}),
// 			layout.Rigid(func(gtx C) D {
// 				return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
// 					return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
// 						return pg.Theme.Card().Layout(gtx, func(gtx C) D {
// 							return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
// 								return pageContent[i](gtx)
// 							})
// 						})
// 					})
// 				})
// 			}),
// 		)
// 	})

// 	return dims
// }

// func (pg *ReceivePage) pageSections(gtx C, body layout.Widget) D {
// 	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
// 		gtx.Constraints.Min.X = gtx.Constraints.Max.X
// 		return layout.UniformInset(values.MarginPadding16).Layout(gtx, body)
// 	})
// }

// pageBackdropLayout layout of background overlay when the popup button generate new address is show,
// click outside of the generate new address button to hide the button
func (pg *ReceivePage) pageBackdropLayout(gtx C) {
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

func (pg *ReceivePage) headerLayout(gtx C) D {
	return layout.Flex{}.Layout(gtx,
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

// func (pg *ReceivePage) titleLayout(gtx C) D {
// 	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
// 		layout.Rigid(func(gtx C) D {
// 			txt := pg.Theme.Body2(values.String(values.StrYourAddress))
// 			txt.Color = pg.Theme.Color.GrayText2
// 			return txt.Layout(gtx)
// 		}),
// 		layout.Rigid(func(gtx C) D {
// 			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 				layout.Rigid(func(gtx C) D {
// 					if pg.isNewAddr {
// 						m := op.Record(gtx.Ops)
// 						layout.Inset{Top: values.MarginPadding30, Left: unit.Dp(-152)}.Layout(gtx, func(gtx C) D {
// 							return pg.Theme.Shadow().Layout(gtx, pg.newAddr.Layout)
// 						})
// 						op.Defer(gtx.Ops, m.Stop())
// 					}
// 					return D{}
// 				}),
// 				layout.Rigid(pg.more.Layout),
// 			)
// 		}),
// 	)
// }

func (pg *ReceivePage) addressLayout(gtx C) D {
	// return layout.Inset{Top: values.MarginPadding14, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
	// return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
	// layout.Rigid(func(gtx C) D {
	border := widget.Border{
		Color:        pg.Theme.Color.Gray4,
		CornerRadius: values.MarginPadding10,
		Width:        values.MarginPadding2,
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return border.Layout(gtx, func(gtx C) D {
		// return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		return components.VerticalInset(values.MarginPadding12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), "")
			if pg.currentAddress != "" && pg.selectedWallet.IsSynced() {
				lbl.Text = pg.currentAddress
			}
			return layout.Center.Layout(gtx, lbl.Layout)
			// return lbl.Layout(gtx)
		})
		// })
	})
	// card := cryptomaterial.Card{Color: pg.Theme.Color.Gray4}
	// card.Radius = cryptomaterial.CornerRadius{TopRight: 0, TopLeft: 8, BottomRight: 0, BottomLeft: 8}
	// return card.Layout(gtx, func(gtx C) D {
	// 	return layout.Inset{
	// 		Top:    values.MarginPadding8,
	// 		Bottom: values.MarginPadding8,
	// 		Left:   values.MarginPadding30,
	// 		Right:  values.MarginPadding30,
	// 	}.Layout(gtx, func(gtx C) D {
	// 		pg.receiveAddress.Text = pg.currentAddress
	// 		return pg.receiveAddress.Layout(gtx)
	// 	})
	// })
	// }),
	// layout.Rigid(func(gtx C) D {
	// 	return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, func(gtx C) D { return D{} })
	// }),
	// layout.Rigid(func(gtx C) D {
	// 	card := cryptomaterial.Card{Color: pg.copy.Background}
	// 	card.Radius = cryptomaterial.CornerRadius{TopRight: 8, TopLeft: 0, BottomRight: 8, BottomLeft: 0}
	// 	return card.Layout(gtx, pg.copy.Layout)
	// }),
	// )
	// })
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

	if pg.newAddr1.Clicked() {
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
		textWithUnit := values.String(values.StrReceive) + " " + string(pg.selectedWallet.GetAssetType())
		info := modal.NewCustomModal(pg.Load).
			Title(textWithUnit).
			Body(values.String(values.StrReceiveInfo)).
			SetContentAlignment(layout.NW, layout.W, layout.Center)
		pg.ParentWindow().ShowModal(info)
	}
}

func (pg *ReceivePage) generateNewAddress() (string, error) {
	selectedAccount := pg.selector.SelectedAccount()
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

func (pg *ReceivePage) handleCopyEvent(gtx C) {
	// Prevent copying again if the timer hasn't expired
	if pg.copy1.Clicked() && !pg.isCopying {
		clipboard.WriteOp{Text: pg.currentAddress}.Add(gtx.Ops)
		pg.Toast.Notify(values.String(values.StrCopied))

		// pg.copy.Text = values.String(values.StrCopied)
		// pg.copy.Background = pg.Theme.Color.Success
		// pg.isCopying = true
		// time.AfterFunc(time.Second*4, func() {
		// 	pg.copy.Text = values.String(values.StrCopy)
		// 	pg.copy.Background = pg.Theme.Color.Primary
		// 	pg.isCopying = false
		// })
	}

	if pg.copyAddressButton.Clicked() {
		clipboard.WriteOp{Text: pg.copyAddressButton.Text}.Add(gtx.Ops)
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
func (pg *ReceivePage) OnNavigatedFrom() {
	pg.selector.StopTxNtfnListener()
}
