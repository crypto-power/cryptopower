//go:build (linux && !android) || openbsd || freebsd || netbsd
// +build linux,!android openbsd freebsd netbsd

package notify

import (
	"fmt"

	"github.com/esiqveland/notify"
	dbus "github.com/godbus/dbus/v5"
)

type dbusNotifier struct {
	notify.Notifier
}

var _ Notifier = (*dbusNotifier)(nil)

func newNotifier() (Notifier, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed connecting to dbus: %w", err)
	}
	notifier, err := notify.New(conn)
	if err != nil {
		return nil, fmt.Errorf("failed creating notifier: %w", err)
	}
	return &dbusNotifier{
		Notifier: notifier,
	}, nil
}

type dbusNotification struct {
	id uint32
	*dbusNotifier
}

var _ Notification = &dbusNotification{}

func (l *dbusNotifier) CreateNotification(title, text string) (Notification, error) {
	id, err := l.Notifier.SendNotification(notify.Notification{
		Summary: title,
		Body:    text,
	})
	if err != nil {
		return nil, err
	}
	return &dbusNotification{
		id:           id,
		dbusNotifier: l,
	}, nil
}

func (l dbusNotification) Cancel() error {
	_, err := l.dbusNotifier.CloseNotification(l.id)
	return err
}
