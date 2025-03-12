package notify

import "sync"

// impl is a package global notifier initialized to the current platform
// implementation.
var impl Notifier
var implErr error
var implLock sync.Mutex

// Notifier provides methods for creating and managing notifications.
type Notifier interface {
	CreateNotification(title, text string) (Notification, error)
}

// IconNotifier is a notifier that can display an icon notification.
type IconNotifier interface {
	Notifier
	UseIcon(path string)
}

// OngoingNotifier is a notifier that can display an ongoing notification.
// Some platforms (currently Android) support persistent notifications and
// will implement this optional interface.
type OngoingNotifier interface {
	Notifier
	// CreateOngoingNotification creates a notification that cannot be dismissed
	// by the user. Callers must be careful to cancel this notification when it
	// is no longer needed.
	CreateOngoingNotification(title, text string) (Notification, error)
}

// NewNotifier creates a new Manager tailored to the current operating system.
func NewNotifier() (Notifier, error) {
	return newNotifier()
}

// Notification handle that can used to manipulate a platform notification,
// such as by cancelling it.
type Notification interface {
	// Cancel a notification.
	Cancel() error
}

// noop notification for convenience.
type noop struct{}

func (noop) Cancel() error {
	return nil
}

// Push a notification to the OS.
func Push(title, text string) (Notification, error) {
	implLock.Lock()
	defer implLock.Unlock()
	if impl == nil && implErr == nil {
		impl, implErr = newNotifier()
	}
	if implErr != nil {
		return nil, implErr
	}
	return impl.CreateNotification(title, text)
}
