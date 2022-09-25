package notification

import (
	"path/filepath"

	"github.com/gen2brain/beeep"
	"gitlab.com/raedah/cryptopower/ui/utils"
)

const (
	icon  = "ui/assets/decredicons/qrcodeSymbol.png"
	title = "Cryptopower"
)

type SystemNotification struct {
	iconPath string
	message  string
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
