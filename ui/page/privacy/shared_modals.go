package privacy

import (
	"fmt"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/values"
)

type sharedModalConfig struct {
	*load.Load
	window        app.WindowNavigator
	pageNavigator app.PageNavigator
	checkBox      cryptomaterial.CheckBoxStyle
}

func showInfoModal(conf *sharedModalConfig, title, body, btnText string, isError bool) {
	var info *modal.InfoModal
	if isError {
		info = modal.NewErrorModal(conf.Load, title, modal.DefaultClickFunc())
	} else {
		info = modal.NewSuccessModal(conf.Load, title, modal.DefaultClickFunc())
	}
	info.Body(body).SetPositiveButtonText(btnText)
	conf.window.ShowModal(info)
}

func showModalSetupMixerInfo(conf *sharedModalConfig) {
	info := modal.NewCustomModal(conf.Load).
		Title(values.String(values.StrMultipleMixerAccNeeded)).
		SetupWithTemplate(modal.SetupMixerInfoTemplate).
		CheckBox(conf.checkBox, false).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonText(values.String(values.StrInitiateSetup)).
		SetPositiveButtonCallback(func(movefundsChecked bool, _ *modal.InfoModal) bool {
			showModalSetupMixerAcct(conf, movefundsChecked)
			return true
		})
	conf.window.ShowModal(info)
}

func showModalSetupMixerAcct(conf *sharedModalConfig, movefundsChecked bool) {
	accounts, _ := conf.WL.SelectedWallet.Wallet.GetAccountsRaw()
	for _, acct := range accounts.Acc {
		if acct.Name == "mixed" || acct.Name == "unmixed" {
			info := modal.NewErrorModal(conf.Load, values.String(values.StrTakenAccount), modal.DefaultClickFunc()).
				Body(values.String(values.StrMixerAccErrorMsg)).
				SetPositiveButtonText(values.String(values.StrBackAndRename)).
				SetPositiveButtonCallback(func(movefundsChecked bool, _ *modal.InfoModal) bool {
					conf.pageNavigator.CloseCurrentPage()
					return true
				})
			conf.window.ShowModal(info)
			return
		}
	}

	passwordModal := modal.NewCreatePasswordModal(conf.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title("Confirm to create needed accounts").
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			go func() {
				err := conf.WL.SelectedWallet.Wallet.CreateMixerAccounts("mixed", "unmixed", password)
				if err != nil {
					pm.SetError(err.Error())
					pm.SetLoading(false)
					return
				}
				conf.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(libwallet.AccountMixerConfigSet, true)

				if movefundsChecked {
					err := moveFundsFromDefaultToUnmixed(conf, password)
					if err != nil {
						log.Error(err)
						txt := fmt.Sprintf("Error moving funds: %s.\n%s", err.Error(), "Auto funds transfer has been skipped. Move funds to unmixed account manually from the send page.")
						showInfoModal(conf, values.String(values.StrMoveToUnmixed), txt, values.String(values.StrGotIt), true)
					}
				}

				pm.Dismiss()

				conf.pageNavigator.Display(NewAccountMixerPage(conf.Load))
			}()

			return false
		})
	conf.window.ShowModal(passwordModal)
}

// moveFundsFromDefaultToUnmixed moves funds from the default wallet account to the
// newly created unmixed account
func moveFundsFromDefaultToUnmixed(conf *sharedModalConfig, password string) error {
	acc, err := conf.WL.SelectedWallet.Wallet.GetAccountsRaw()
	if err != nil {
		return err
	}

	// get the first account in the wallet as this is the default
	sourceAccount := acc.Acc[0]
	destinationAccount := conf.WL.SelectedWallet.Wallet.UnmixedAccountNumber()

	destinationAddress, err := conf.WL.SelectedWallet.Wallet.CurrentAddress(destinationAccount)
	if err != nil {
		return err
	}

	unsignedTx, err := conf.WL.MultiWallet.NewUnsignedTx(sourceAccount.WalletID, sourceAccount.Number)
	if err != nil {
		return err
	}

	// get tx fees
	feeAndSize, err := unsignedTx.EstimateFeeAndSize()
	if err != nil {
		return err
	}

	// calculate max amount to be sent
	amountAtom := sourceAccount.Balance.Spendable - feeAndSize.Fee.AtomValue
	err = unsignedTx.AddSendDestination(destinationAddress, amountAtom, true)
	if err != nil {
		return err
	}

	// send fund
	_, err = unsignedTx.Broadcast([]byte(password))
	if err != nil {
		return err
	}

	showInfoModal(conf, values.String(values.StrTxSent), "", values.String(values.StrGotIt), false)

	return err
}
