//go:build windows
// +build windows

package utils

import "github.com/crypto-power/cryptopower/ui/notify"

// // Create notifier with icon
func CreateNewNotifierWithIcon(iconPath string) (notifier notify.Notifier, err error) {
	notifier, err = CreateNewNotifier()
	if err != nil {
		log.Error(err.Error())
		return
	}
	notifier.(notify.IconNotifier).UseIcon(iconPath)
	return
}
