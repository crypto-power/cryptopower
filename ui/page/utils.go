// util contains functions that don't contain layout code. They could be considered helpers that aren't particularly
// bounded to a page.

package page

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/values"
)

func translateErr(err error) string {
	switch err.Error() {
	case utils.ErrInvalidPassphrase:
		return values.String(values.StrInvalidPassphrase)
	}

	return err.Error()
}

func EditorsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if e.Text() == "" {
			return false
		}
	}
	return true
}

// getLockWallet returns a list of locked wallets
func getLockedWallets(wallets []*dcr.Wallet) []*dcr.Wallet {
	var walletsLocked []*dcr.Wallet
	for _, wl := range wallets {
		if !wl.HasDiscoveredAccounts && wl.IsLocked() {
			walletsLocked = append(walletsLocked, wl)
		}
	}

	return walletsLocked
}

func computePasswordStrength(pb *cryptomaterial.ProgressBarStyle, th *cryptomaterial.Theme, editors ...*widget.Editor) {
	password := editors[0]
	strength := dcr.ShannonEntropy(password.Text()) / 4.0
	pb.Progress = float32(strength * 100)
	pb.Color = th.Color.Success
}

func HandleSubmitEvent(editors ...*widget.Editor) bool {
	var submit bool
	for _, editor := range editors {
		for _, e := range editor.Events() {
			if _, ok := e.(widget.SubmitEvent); ok {
				submit = true
			}
		}
	}
	return submit
}

func GetAbsolutePath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("error getting executable path: %s", err.Error())
	}

	exSym, err := filepath.EvalSymlinks(ex)
	if err != nil {
		return "", fmt.Errorf("error getting filepath after evaluating sym links")
	}

	return path.Dir(exSym), nil
}

func handleSubmitEvent(editors ...*widget.Editor) bool {
	var submit bool
	for _, editor := range editors {
		for _, e := range editor.Events() {
			if _, ok := e.(widget.SubmitEvent); ok {
				submit = true
			}
		}
	}
	return submit
}
