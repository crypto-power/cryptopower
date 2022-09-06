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
	info.Body(body).
		PositiveButton(btnText, modal.DefaultClickFunc())
	conf.window.ShowModal(info)
}

func showModalSetupMixerInfo(conf *sharedModalConfig) {
	info := modal.NewCustomModal(conf.Load).
		Title("Set up mixer by creating two needed accounts").
		SetupWithTemplate(modal.SetupMixerInfoTemplate).
		CheckBox(conf.checkBox, false).
		NegativeButton(values.String(values.StrCancel), func() {}).
		PositiveButton("Begin setup", func(movefundsChecked bool, _ *modal.InfoModal) bool {
			showModalSetupMixerAcct(conf, movefundsChecked)
			return true
		})
	conf.window.ShowModal(info)
}

func showModalSetupMixerAcct(conf *sharedModalConfig, movefundsChecked bool) {
	accounts, _ := conf.WL.SelectedWallet.Wallet.GetAccountsRaw()
	txt := "There are existing accounts named mixed or unmixed. Please change the name to something else for now. You can change them back after the setup."
	for _, acct := range accounts.Acc {
		if acct.Name == "mixed" || acct.Name == "unmixed" {
			info := modal.NewErrorModal(conf.Load, "Account name is taken", modal.DefaultClickFunc()).
				Body(txt).
				PositiveButton("Go back & rename", func(movefundsChecked bool, _ *modal.InfoModal) bool {
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
		NegativeButton("", func() {}).
		PositiveButton("", func(_, password string, pm *modal.CreatePasswordModal) bool {
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
						showInfoModal(conf, "Move funds to unmixed account", txt, "Got it", true)
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

	showInfoModal(conf, "Transaction sent!", "", "Got it", false)

	return err
}
