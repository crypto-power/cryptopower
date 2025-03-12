//go:build !linux && !android && !openbsd && !freebsd && !dragonfly && !netbsd && !darwin && !windows
// +build !linux,!android,!openbsd,!freebsd,!dragonfly,!netbsd,!darwin,!windows

package notify

type unsupported struct{}

func newNotifier() (Notifier, error) {
	return unsupported{}, nil
}

func (unsupported) CreateNotification(title, text string) (Notification, error) {
	return &noop{}, nil
}
