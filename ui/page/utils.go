// util contains functions that don't contain layout code. They could be considered helpers that aren't particularly
// bounded to a page.

package page

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/values"
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
