package components

import (
	"bytes"
	"context"
	"image"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/yeqown/go-qrcode"
)

type ReceiveModal struct {
	*load.Load
	*cryptomaterial.Modal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	pageContainer *widget.List

	okBtn cryptomaterial.Button

	addressEditor cryptomaterial.Editor
	copyRedirect  *cryptomaterial.Clickable

	sourceAccountSelector *WalletAndAccountSelector
	sourceWalletSelector  *WalletAndAccountSelector

	isNewAddr, isInfo bool
	currentAddress    string
	qrImage           *image.Image
	newAddr           cryptomaterial.Button
	info, more        cryptomaterial.IconButton
	receiveAddress    cryptomaterial.Label
}

func NewReceiveModal(l *load.Load) *ReceiveModal {
	rm := &ReceiveModal{
		Load:           l,
		Modal:          l.Theme.ModalFloatTitle(values.String(values.StrReceive)),
		copyRedirect:   l.Theme.NewClickable(false),
		info:           l.Theme.IconButton(cryptomaterial.MustIcon(widget.NewIcon(icons.ActionInfo))),
		more:           l.Theme.IconButton(l.Theme.Icons.NavigationMore),
		newAddr:        l.Theme.Button(values.String(values.StrGenerateAddress)),
		receiveAddress: l.Theme.Label(values.TextSize20, ""),
	}

	rm.okBtn = l.Theme.Button(values.String(values.StrOK))
	rm.okBtn.Font.Weight = font.Medium

	rm.addressEditor = l.Theme.IconEditor(new(widget.Editor), "", l.Theme.Icons.ContentCopy, true)
	rm.addressEditor.Editor.SingleLine = true

	rm.info.Inset, rm.info.Size = layout.UniformInset(values.MarginPadding5), values.MarginPadding20
	rm.more.Inset = layout.UniformInset(values.MarginPadding0)
	rm.newAddr.Inset = layout.UniformInset(values.MarginPadding10)
	rm.newAddr.Color = rm.Theme.Color.Text
	rm.newAddr.Background = rm.Theme.Color.Surface
	rm.newAddr.HighlightColor = rm.Theme.Color.Gray4
	rm.newAddr.ButtonStyle.TextSize = values.TextSize14
	rm.newAddr.ButtonStyle.Font.Weight = font.SemiBold

	rm.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	rm.initWalletSelectors()

	return rm
}

func (rm *ReceiveModal) OnResume() {
	rm.ctx, rm.ctxCancel = context.WithCancel(context.TODO())

	rm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		rm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})
}

func (rm *ReceiveModal) OnDismiss() {
	rm.ctxCancel()
}

func (rm *ReceiveModal) Handle() {

	if rm.okBtn.Clicked() || rm.Modal.BackdropClicked(true) {
		rm.Dismiss()
	}

	if rm.more.Button.Clicked() {
		rm.isNewAddr = !rm.isNewAddr
		if rm.isInfo {
			rm.isInfo = false
		}
	}

	if rm.newAddr.Clicked() {
		newAddr, err := rm.generateNewAddress()
		if err != nil {
			log.Debug("Error generating new address: " + err.Error())
			return
		}

		rm.currentAddress = newAddr
		rm.generateQRForAddress()
		rm.isNewAddr = false
	}

	if rm.info.Button.Clicked() {
		textWithUnit := values.String(values.StrReceive) + " " + string(rm.sourceWalletSelector.selectedWallet.GetAssetType())
		info := modal.NewCustomModal(rm.Load).
			Title(textWithUnit).
			Body(values.String(values.StrReceiveInfo)).
			SetContentAlignment(layout.NW, layout.W, layout.Center)
		rm.ParentWindow().ShowModal(info)
	}
}

func (rm *ReceiveModal) generateQRForAddress() {
	rm.addressEditor.Editor.SetText(rm.currentAddress)

	var imgOpt qrcode.ImageOption
	switch rm.sourceWalletSelector.selectedWallet.Asset.GetAssetType() {
	case "DCR":
		imgOpt = qrcode.WithLogoImage(rm.Theme.Icons.DCR)
	case "BTC":
		imgOpt = qrcode.WithLogoImage(rm.Theme.Icons.BTC)
	case "LTC":
		imgOpt = qrcode.WithLogoImage(rm.Theme.Icons.LTC)
	}

	qrImage, err := qrcode.New(rm.currentAddress, imgOpt)
	if err != nil {
		log.Error("Error generating address qrCode: " + err.Error())
		return
	}

	var buffer bytes.Buffer
	err = qrImage.SaveTo(&buffer)
	if err != nil {
		log.Error(err.Error())
		return
	}

	decodeImg, _, err := image.Decode(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		log.Error("Error decoding image: " + err.Error())
		return
	}

	rm.qrImage = &decodeImg
}

func (rm *ReceiveModal) generateNewAddress() (string, error) {
	newAddr, err := rm.sourceWalletSelector.selectedWallet.NextAddress(rm.sourceAccountSelector.selectedAccount.Number)
	if err != nil {
		return "", err
	}

	rm.currentAddress = newAddr

	return newAddr, nil
}

func (rm *ReceiveModal) generateCurrentAddress() error {
	currentAddress, err := rm.sourceWalletSelector.selectedWallet.CurrentAddress(rm.sourceAccountSelector.SelectedAccount().Number)
	if err != nil {
		log.Error("Error getting current address: " + err.Error())
		return err
	}
	rm.currentAddress = currentAddress

	return nil
}

func (rm *ReceiveModal) Layout(gtx layout.Context) D {
	w := []layout.Widget{
		func(gtx C) D {
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Stack{Alignment: layout.NE}.Layout(gtx,
						layout.Expanded(func(gtx C) D {
							return layout.Inset{
								Bottom: values.MarginPadding16,
							}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Bottom: values.MarginPadding8,
										}.Layout(gtx, func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													assetTxt := rm.sourceWalletSelector.selectedWallet.GetAssetType().ToFull()
													txt := rm.Theme.Label(values.TextSize20, values.String(values.StrReceive)+" "+assetTxt)
													txt.Font.Weight = font.SemiBold
													return txt.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													return rm.info.Layout(gtx)
												}),
											)
										})

									}),
									layout.Rigid(func(gtx C) D {
										return rm.Theme.List(rm.pageContainer).Layout(gtx, 1, func(gtx C, i int) D {
											return cryptomaterial.LinearLayout{
												Width:     cryptomaterial.MatchParent,
												Height:    cryptomaterial.WrapContent,
												Direction: layout.Center,
											}.Layout2(gtx, func(gtx C) D {
												return cryptomaterial.LinearLayout{
													Width:  cryptomaterial.MatchParent,
													Height: cryptomaterial.WrapContent,
												}.Layout2(gtx, func(gtx C) D {
													return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding16},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Inset{
																			Bottom: values.MarginPadding16,
																		}.Layout(gtx, func(gtx C) D {
																			return rm.sourceWalletSelector.Layout(rm.ParentWindow(), gtx)
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		return rm.sourceAccountSelector.Layout(rm.ParentWindow(), gtx)
																	}),
																	layout.Rigid(func(gtx C) D {
																		if !rm.sourceWalletSelector.SelectedWallet().IsSynced() {
																			txt := rm.Theme.Label(values.TextSize14, values.String(values.StrSourceWalletNotSynced))
																			txt.Font.Weight = font.SemiBold
																			txt.Color = rm.Theme.Color.Danger
																			return txt.Layout(gtx)
																		}
																		return D{}
																	}),
																)
															})
														}),
														layout.Rigid(func(gtx C) D {
															gtx.Constraints.Min.X = gtx.Constraints.Max.X
															if rm.sourceWalletSelector.selectedWallet.IsSynced() {
																return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		txt := rm.Theme.Body2(values.String(values.StrYourAddress))
																		txt.Color = rm.Theme.Color.GrayText2
																		return txt.Layout(gtx)
																	}),
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				if rm.isNewAddr {
																					m := op.Record(gtx.Ops)
																					layout.Inset{Top: values.MarginPadding30, Left: unit.Dp(-152)}.Layout(gtx, func(gtx C) D {
																						return rm.Theme.Shadow().Layout(gtx, rm.newAddr.Layout)
																					})
																					op.Defer(gtx.Ops, m.Stop())
																				}
																				return D{}
																			}),
																			layout.Rigid(rm.more.Layout),
																		)
																	}),
																)
															}
															return D{}
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Top: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
																	if rm.sourceWalletSelector.selectedWallet.IsSynced() {
																		return layout.Flex{}.Layout(gtx,
																			layout.Flexed(0.9, rm.Load.Theme.Body1(rm.addressEditor.Editor.Text()).Layout),
																			layout.Flexed(0.1, func(gtx C) D {
																				return layout.E.Layout(gtx, func(gtx C) D {
																					mGtx := gtx
																					if rm.addressEditor.Editor.Text() == "" {
																						mGtx = gtx.Disabled()
																					}
																					if rm.copyRedirect.Clicked() {
																						clipboard.WriteOp{Text: rm.addressEditor.Editor.Text()}.Add(mGtx.Ops)
																						rm.Load.Toast.Notify(values.String(values.StrCopied))
																					}
																					return rm.copyRedirect.Layout(mGtx, rm.Load.Theme.Icons.CopyIcon.Layout24dp)
																				})
																			}),
																		)
																	}
																	return D{}
																})
															})
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding16},
																	Direction:   layout.Center,
																	Alignment:   layout.Middle,
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Center.Layout(gtx, func(gtx C) D {
																			return layout.Flex{
																				Axis:      layout.Vertical,
																				Alignment: layout.Middle,
																			}.Layout(gtx,
																				layout.Rigid(func(gtx C) D {
																					if rm.qrImage == nil || !rm.sourceWalletSelector.selectedWallet.IsSynced() {
																						// Display generated address only on a synced wallet
																						return D{}
																					}

																					return rm.Theme.ImageIcon(gtx, *rm.qrImage, 180)
																				}),
																			)
																		})
																	}),
																)
															})
														}),
													)
												})
											})
										})
									}),
								)
							})
						}),
					)
				}),
				layout.Stacked(func(gtx C) D {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

					return layout.S.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
										layout.Rigid(rm.okBtn.Layout),
									)
								})
							}),
						)

					})
				}),
			)
		},
	}
	return rm.Modal.Layout(gtx, w, 450)
}

func (rm *ReceiveModal) initWalletSelectors() {
	// Source wallet picker
	rm.sourceWalletSelector = NewWalletAndAccountSelector(rm.Load).
		Title(values.String(values.StrSelectWallet))

	// Source account picker
	rm.sourceAccountSelector = NewWalletAndAccountSelector(rm.Load).
		Title(values.String(values.StrSelectAcc)).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32

			return accountIsValid
		})
	rm.sourceAccountSelector.SelectFirstValidAccount(rm.sourceWalletSelector.SelectedWallet())

	rm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		rm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
		rm.generateAddressAndQRCode()
	})

	rm.sourceAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		rm.generateAddressAndQRCode()
	})

	rm.generateAddressAndQRCode()
}

func (rm *ReceiveModal) generateAddressAndQRCode() {
	err := rm.generateCurrentAddress()
	if err != nil {
		log.Error("Error getting current address: " + err.Error())
		return
	}
	rm.generateQRForAddress()
}
