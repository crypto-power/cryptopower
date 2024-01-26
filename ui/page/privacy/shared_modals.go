package privacy

import (
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
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

func showModalSetupMixerInfo(conf *sharedModalConfig, dcrWallet *dcr.Asset) {
	info := modal.NewCustomModal(conf.Load).
		Title(values.String(values.StrMultipleMixerAccNeeded)).
		SetupWithTemplate(modal.SetupMixerInfoTemplate).
		CheckBox(conf.checkBox, false).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonText(values.String(values.StrInitiateSetup)).
		SetPositiveButtonCallback(func(movefundsChecked bool, _ *modal.InfoModal) bool {
			showModalSetupMixerAcct(conf, dcrWallet, movefundsChecked)
			return true
		})
	conf.window.ShowModal(info)
}

func showModalSetupMixerAcct(conf *sharedModalConfig, dcrWallet *dcr.Asset, movefundsChecked bool) {
	if dcrWallet.GetAssetType() != utils.DCRWalletAsset {
		log.Warnf("Mixer Account for (%v) not supported.",
			dcrWallet.GetAssetType())
		return
	}

	accounts, _ := dcrWallet.GetAccountsRaw()
	for _, acct := range accounts.Accounts {
		if acct.Name == values.String(values.StrMixed) || acct.Name == values.String(values.StrUnmixed) {
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
		Title(values.String(values.StrConfirmToCreateAccs)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			defer pm.Dismiss()
			err := dcrWallet.CreateMixerAccounts(values.String(values.StrMixed), values.String(values.StrUnmixed), password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}

			if movefundsChecked {
				err := moveFundsFromDefaultToUnmixed(conf, dcrWallet, password)
				if err != nil {
					log.Error(err)
					txt := values.StringF(values.StrErrorMovingFunds, err.Error())
					showInfoModal(conf, values.String(values.StrMoveToUnmixed), txt, values.String(values.StrGotIt), true)
					return false
				}
			}

			conf.pageNavigator.Display(NewAccountMixerPage(conf.Load, dcrWallet))

			return true
		})
	conf.window.ShowModal(passwordModal)
}

// moveFundsFromDefaultToUnmixed moves funds from the default wallet account to the
// newly created unmixed account
func moveFundsFromDefaultToUnmixed(conf *sharedModalConfig, dcrWallet *dcr.Asset, password string) error {
	acc, err := dcrWallet.GetAccountsRaw()
	if err != nil {
		return err
	}

	// get the first account in the wallet as this is the default
	sourceAccount := acc.Accounts[0]

	balAtom := sourceAccount.Balance.Spendable.ToInt()
	if balAtom <= 0 {
		// Nothing to do.
		return nil
	}

	destinationAccount := dcrWallet.UnmixedAccountNumber()

	destinationAddress, err := dcrWallet.CurrentAddress(destinationAccount)
	if err != nil {
		return err
	}

	err = dcrWallet.NewUnsignedTx(sourceAccount.Number, nil)
	if err != nil {
		return err
	}

	// get tx fees
	feeAndSize, err := dcrWallet.EstimateFeeAndSize()
	if err != nil {
		return err
	}

	// calculate max amount to be sent
	amountAtom := sourceAccount.Balance.Spendable.ToInt() - feeAndSize.Fee.UnitValue
	err = dcrWallet.AddSendDestination(0, destinationAddress, amountAtom, true)
	if err != nil {
		return err
	}

	// send fund
	_, err = dcrWallet.Broadcast(password, "")
	if err != nil {
		return err
	}

	showInfoModal(conf, values.String(values.StrTxSent), "", values.String(values.StrGotIt), false)

	return err
}
