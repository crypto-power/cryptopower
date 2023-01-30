package notification

import (
	"path/filepath"

	"code.cryptopower.dev/group/cryptopower/ui/utils"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"github.com/gen2brain/beeep"
)

const (
	icon = "ui/assets/decredicons/qrcodeSymbol.png"
)

var title = values.String(values.StrAppName)

type SystemNotification struct {
	iconPath string
}

func NewSystemNotification() (*SystemNotification, error) {
	absolutePath, err := utils.GetAbsolutePath()
	if err != nil {
		return nil, err
	}

	return &SystemNotification{
		iconPath: filepath.Join(absolutePath, icon),
	}, nil
}

func (s *SystemNotification) Notify(message string) error {
	err := beeep.Notify(title, message, s.iconPath)
	if err != nil {
		return err
	}

	return nil
}
