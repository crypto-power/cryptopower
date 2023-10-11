package send

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type sendConfirmModal struct {
	*load.Load
	*cryptomaterial.Modal
	modal.CreatePasswordModal

	closeConfirmationModalButton cryptomaterial.Button
	confirmButton                cryptomaterial.Button
	passwordEditor               cryptomaterial.Editor

	txSent    func()
	isSending bool

	*authoredTxData
	asset           load.WalletMapping
	exchangeRateSet bool
	txLabel         string
}

func newSendConfirmModal(l *load.Load, data *authoredTxData, asset load.WalletMapping) *sendConfirmModal {
	scm := &sendConfirmModal{
		Load:           l,
		Modal:          l.Theme.ModalFloatTitle("send_confirm_modal"),
		authoredTxData: data,
		asset:          asset,
	}

	scm.closeConfirmationModalButton = l.Theme.OutlineButton(values.String(values.StrCancel))
	scm.closeConfirmationModalButton.Font.Weight = font.Medium

	scm.confirmButton = l.Theme.Button("")
	scm.confirmButton.Font.Weight = font.Medium
	scm.confirmButton.SetEnabled(false)

	scm.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	scm.passwordEditor.Editor.SetText("")
	scm.passwordEditor.Editor.SingleLine = true
	scm.passwordEditor.Editor.Submit = true

	return scm
}

func (scm *sendConfirmModal) OnResume() {
	scm.passwordEditor.Editor.Focus()
}

func (scm *sendConfirmModal) SetError(err string) {
	scm.passwordEditor.SetError(values.TranslateErr(err))
}

func (scm *sendConfirmModal) SetLoading(loading bool) {
	scm.isSending = loading
	scm.Modal.SetDisabled(loading)
}

func (scm *sendConfirmModal) OnDismiss() {}

func (scm *sendConfirmModal) broadcastTransaction() {
	password := scm.passwordEditor.Editor.Text()
	if password == "" || scm.isSending {
		return
	}

	scm.SetLoading(true)
	go func() {
		_, err := scm.asset.Broadcast(password, scm.txLabel)
		if err != nil {
			scm.SetError(err.Error())
			scm.SetLoading(false)
			return
		}
		successModal := modal.NewSuccessModal(scm.Load, values.String(values.StrTxSent), modal.DefaultClickFunc())
		scm.ParentWindow().ShowModal(successModal)

		scm.txSent()
		scm.Dismiss()
	}()
}

func (scm *sendConfirmModal) Handle() {
	for _, evt := range scm.passwordEditor.Editor.Events() {
		if scm.passwordEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				scm.confirmButton.SetEnabled(scm.passwordEditor.Editor.Text() != "")
			case widget.SubmitEvent:
				scm.broadcastTransaction()
			}
		}
	}

	for scm.confirmButton.Clicked() {
		scm.broadcastTransaction()
	}

	for scm.closeConfirmationModalButton.Clicked() {
		if !scm.isSending {
			scm.Dismiss()
		}
	}
}

func (scm *sendConfirmModal) Layout(gtx C) D {
	w := []layout.Widget{
		func(gtx C) D {
			scm.SetPadding(unit.Dp(0))
			min := gtx.Constraints.Min

			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					defer clip.RRect{
						Rect: image.Rectangle{Max: image.Point{
							X: gtx.Constraints.Min.X,
							Y: gtx.Constraints.Min.Y,
						}},

						NE: 14,
						NW: 14,
					}.Push(gtx.Ops).Pop()
					paint.Fill(gtx.Ops, scm.Theme.Color.Gray5)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx C) D {
					gtx.Constraints.Min = min

					return layout.Inset{Top: values.MarginPadding24, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										icon := scm.Theme.Icons.SendIcon
										return layout.Inset{Top: values.MarginPaddingMinus8}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, icon.Layout24dp)
										})
									}),
									layout.Rigid(func(gtx C) D {
										sendInfoLabel := scm.Theme.Label(unit.Sp(16), values.String(values.StrSendConfModalTitle))
										return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, func(gtx C) D {
												return sendInfoLabel.Layout(gtx)
											})
										})
									}),
									layout.Rigid(func(gtx C) D {
										balLabel := scm.Theme.Label(unit.Sp(24), scm.sendAmount+" ("+scm.sendAmountUSD+")")
										return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, func(gtx C) D {
												return balLabel.Layout(gtx)
											})
										})
									}),
								)
							}),
						)
					})
				}),
			)
		},
		func(gtx C) D {
			return layout.Inset{
				Left: values.MarginPadding16,
				Top:  values.MarginPadding16, Right: values.MarginPadding16,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						sendWallet := scm.WL.AssetsManager.WalletWithID(scm.sourceAccount.WalletID)
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								txt := scm.Theme.Body2(values.String(values.StrFrom))
								txt.Color = scm.Theme.Color.GrayText2
								return txt.Layout(gtx)
							}),
							layout.Rigid(scm.setWalletLogo),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{}.Layout(gtx, func(gtx C) D {
									txt := scm.Theme.Label(unit.Sp(16), sendWallet.GetWalletName())
									txt.Color = scm.Theme.Color.Text
									txt.Font.Weight = font.Medium
									return txt.Layout(gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								card := scm.Theme.Card()
								card.Radius = cryptomaterial.Radius(0)
								card.Color = scm.Theme.Color.Gray4
								inset := layout.Inset{
									Left: values.MarginPadding5,
								}
								return inset.Layout(gtx, func(gtx C) D {
									return card.Layout(gtx, func(gtx C) D {
										return layout.UniformInset(values.MarginPadding2).Layout(gtx, func(gtx C) D {
											txt := scm.Theme.Caption(scm.sourceAccount.Name)
											txt.Color = scm.Theme.Color.GrayText1
											return txt.Layout(gtx)
										})
									})
								})
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, scm.Theme.Icons.ArrowDownIcon.Layout16dp)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								txt := scm.Theme.Body2(values.String(values.StrTo))
								txt.Color = scm.Theme.Color.GrayText2
								return txt.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								if scm.destinationAccount != nil {
									return layout.E.Layout(gtx, func(gtx C) D {
										return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(scm.setWalletLogo),
											layout.Rigid(func(gtx C) D {
												return layout.Inset{}.Layout(gtx, func(gtx C) D {
													destinationWallet := scm.WL.AssetsManager.WalletWithID(scm.destinationAccount.WalletID)
													txt := scm.Theme.Label(unit.Sp(16), destinationWallet.GetWalletName())
													txt.Color = scm.Theme.Color.Text
													txt.Font.Weight = font.Medium
													return txt.Layout(gtx)
												})
											}),
											layout.Rigid(func(gtx C) D {
												card := scm.Theme.Card()
												card.Radius = cryptomaterial.Radius(0)
												card.Color = scm.Theme.Color.Gray4
												inset := layout.Inset{
													Left: values.MarginPadding5,
												}
												return inset.Layout(gtx, func(gtx C) D {
													return card.Layout(gtx, func(gtx C) D {
														return layout.UniformInset(values.MarginPadding2).Layout(gtx, func(gtx C) D {
															txt := scm.Theme.Caption(scm.destinationAccount.Name)
															txt.Color = scm.Theme.Color.GrayText1
															return txt.Layout(gtx)
														})
													})
												})
											}),
										)
									})
								}

								inset := layout.Inset{
									Left: values.MarginPadding5,
								}
								return inset.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding2).Layout(gtx, scm.Theme.Body2(scm.destinationAddress).Layout)
								})
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, scm.Theme.Separator().Layout)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
							txFeeText := scm.txFee
							if scm.exchangeRateSet {
								txFeeText = fmt.Sprintf("%s (%s)", scm.txFee, scm.txFeeUSD)
							}

							return scm.contentRow(gtx, values.String(values.StrFee), txFeeText, "")
						})
					}),
					layout.Rigid(func(gtx C) D {
						totalCostText := scm.totalCost
						if scm.exchangeRateSet {
							totalCostText = fmt.Sprintf("%s (%s)", scm.totalCost, scm.totalCostUSD)
						}
						return scm.contentRow(gtx, values.String(values.StrTotalCost), totalCostText, "")
					}),
				)
			})
		},
		func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding16, Right: values.MarginPadding16}.Layout(gtx, scm.passwordEditor.Layout)
		},
		func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding16, Right: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Right: values.MarginPadding8,
							}.Layout(gtx, func(gtx C) D {
								if scm.isSending {
									return D{}
								}
								return scm.closeConfirmationModalButton.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx C) D {
							if scm.isSending {
								return layout.Inset{Top: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
									return material.Loader(scm.Theme.Base).Layout(gtx)
								})
							}
							scm.confirmButton.Text = values.StrSend
							return scm.confirmButton.Layout(gtx)
						}),
					)
				})
			})
		},
	}
	return scm.Modal.Layout(gtx, w)
}

func (scm *sendConfirmModal) contentRow(gtx C, leftValue, rightValue, walletName string) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := scm.Theme.Body2(leftValue)
			txt.Color = scm.Theme.Color.GrayText2
			return txt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(scm.Theme.Body1(rightValue).Layout),
					layout.Rigid(func(gtx C) D {
						if walletName != "" {
							card := scm.Theme.Card()
							card.Radius = cryptomaterial.Radius(0)
							card.Color = scm.Theme.Color.Gray4
							inset := layout.Inset{
								Left: values.MarginPadding5,
							}
							return inset.Layout(gtx, func(gtx C) D {
								return card.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding2).Layout(gtx, func(gtx C) D {
										txt := scm.Theme.Caption(walletName)
										txt.Color = scm.Theme.Color.GrayText2
										return txt.Layout(gtx)
									})
								})
							})
						}
						return layout.Dimensions{}
					}),
				)
			})
		}),
	)
}

func (scm *sendConfirmModal) setWalletLogo(gtx C) D {
	walletIcon := components.CoinImageBySymbol(scm.Load, scm.asset.GetAssetType(), false)
	if walletIcon == nil {
		return D{}
	}
	inset := layout.Inset{
		Right: values.MarginPadding8, Left: values.MarginPadding25,
	}
	return inset.Layout(gtx, walletIcon.Layout16dp)
}
