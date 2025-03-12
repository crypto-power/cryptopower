//go:build android
// +build android

package utils

import "github.com/crypto-power/cryptopower/ui/notify"

// Create notifier
func CreateNewNotifierWithIcon(iconPath string) (notifier notify.Notifier, err error) {
	notifier, err = notify.NewNotifier()
	return
}
