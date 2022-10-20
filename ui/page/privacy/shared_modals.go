package privacy

import (
	"fmt"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
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
	if conf.WL.SelectedWallet.Wallet.GetAssetType() != utils.DCRWalletAsset {
		log.Warnf("Mixer Account for (%v) not supported.",
			conf.WL.SelectedWallet.Wallet.GetAssetType())
		return
	}

	accounts, _ := conf.WL.SelectedWallet.Wallet.GetAccountsRaw()
	for _, acct := range accounts.Accounts {
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
			dcrUniqueImpl := conf.WL.SelectedWallet.Wallet.(dcr.DCRUniqueAsset)
			err := dcrUniqueImpl.CreateMixerAccounts("mixed", "unmixed", password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			conf.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, true)

			if movefundsChecked {
				err := moveFundsFromDefaultToUnmixed(conf, password)
				if err != nil {
					log.Error(err)
					txt := fmt.Sprintf("Error moving funds: %s.\n%s", err.Error(), "Auto funds transfer has been skipped. Move funds to unmixed account manually from the send page.")
					showInfoModal(conf, values.String(values.StrMoveToUnmixed), txt, values.String(values.StrGotIt), true)
					return false
				}
			}

			pm.Dismiss()

			conf.pageNavigator.Display(NewAccountMixerPage(conf.Load))

			return true
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

	dcrUniqueImpl := conf.WL.SelectedWallet.Wallet.(dcr.DCRUniqueAsset)
	// get the first account in the wallet as this is the default
	sourceAccount := acc.Accounts[0]
	destinationAccount := dcrUniqueImpl.UnmixedAccountNumber()

	destinationAddress, err := conf.WL.SelectedWallet.Wallet.CurrentAddress(destinationAccount)
	if err != nil {
		return err
	}

	err = dcrUniqueImpl.NewUnsignedTx(sourceAccount.Number)
	if err != nil {
		return err
	}

	// get tx fees
	feeAndSize, err := dcrUniqueImpl.EstimateFeeAndSize()
	if err != nil {
		return err
	}

	// calculate max amount to be sent
	amountAtom := sourceAccount.Balance.Spendable.ToInt() - feeAndSize.Fee.UnitValue
	err = dcrUniqueImpl.AddSendDestination(destinationAddress, amountAtom, true)
	if err != nil {
		return err
	}

	// send fund
	_, err = dcrUniqueImpl.Broadcast(password)
	if err != nil {
		return err
	}

	showInfoModal(conf, values.String(values.StrTxSent), "", values.String(values.StrGotIt), false)

	return err
}
