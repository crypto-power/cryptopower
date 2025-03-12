//go:build darwin
// +build darwin

package notify

import (
	"github.com/crypto-power/cryptopower/ui/notify/macos"
)

type darwinNotifier struct {
	channel macos.NotificationChannel
}

var _ Notifier = (*darwinNotifier)(nil)

func newNotifier() (Notifier, error) {
	c := macos.NewNotificationChannel("Gio App")

	return &darwinNotifier{channel: c}, nil
}

func (a *darwinNotifier) CreateNotification(title, text string) (Notification, error) {
	notification, err := a.channel.Send(title, text)
	if err != nil {
		return nil, err
	}
	return notification, nil
}
