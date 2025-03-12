//go:build android
// +build android

package notify

import (
	"github.com/crypto-power/cryptopower/ui/notify/android"
)

type androidNotifier struct {
	channel *android.NotificationChannel
	icon    string
}

var _ Notifier = (*androidNotifier)(nil)

func newNotifier() (Notifier, error) {
	channel, err := android.NewChannel(android.ImportanceDefault, "DEFAULT", "niotify", "background notifications")
	if err != nil {
		return nil, err
	}
	return &androidNotifier{
		channel: channel,
	}, nil
}

func (a *androidNotifier) CreateNotification(title, text string) (Notification, error) {
	return a.createNotification(title, text, false)
}

func (a *androidNotifier) createNotification(title, text string, ongoing bool) (Notification, error) {
	notification, err := a.channel.Send(title, text, ongoing)
	if err != nil {
		return nil, err
	}
	return notification, nil
}

func (a *androidNotifier) CreateOngoingNotification(title, text string) (Notification, error) {
	return a.createNotification(title, text, true)
}

// UseIcon configures an icon to use for notifications, specified as a filepath.
func (m *androidNotifier) UseIcon(path string) {
	m.icon = path
}
