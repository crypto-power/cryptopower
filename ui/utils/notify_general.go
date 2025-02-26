//go:build linux || android || ios || darwin || openbsd || freebsd || netbsd
// +build linux android ios darwin openbsd freebsd netbsd

package utils

import "gioui.org/x/notify"

// Create notifier
func CreateNewNotifierWithIcon(iconPath string) (notifier notify.Notifier, err error) {
	return CreateNewNotifier()
}
