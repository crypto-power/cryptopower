package android

import (
	"fmt"
	"sync"

	"gioui.org/app"

	"git.wow.st/gmp/jni"
)

//go:generate javac -target 1.8 -source 1.8 -bootclasspath $ANDROID_HOME/platforms/android-26/android.jar ./NotificationHelper.java
//go:generate jar cf NotificationHelper.jar ./NotificationHelper.class
//go:generate rm ./NotificationHelper.class

// Importance represents the priority of notifications sent over a particular NotificationChannel.
// You MUST use one of the constants defined here when specifying an importance for a channel.
// These constants map to different values within the JVM.
type Importance int

const (
	ImportanceDefault Importance = iota
	ImportanceHigh
	ImportanceLow
	ImportanceMax
	ImportanceMin
	ImportanceNone
	ImportanceUnspecified
	importanceEnd // compile-time hack to track the number of importance constants and size the
	// array holding their values correctly. If new constants need to be added, add them above
	// this.
)

// value returns the JVM value for this importance constant. It must not be invoked before the
// importances have been resolved.
func (i Importance) value() int32 {
	return importances[i]
}

const (
	helperClass               = "ht/sr/git/whereswaldon/niotify/NotificationHelper"
	importanceDefaultName     = "IMPORTANCE_DEFAULT"
	importanceHighName        = "IMPORTANCE_HIGH"
	importanceLowName         = "IMPORTANCE_LOW"
	importanceMaxName         = "IMPORTANCE_MAX"
	importanceMinName         = "IMPORTANCE_MIN"
	importanceNoneName        = "IMPORTANCE_NONE"
	importanceUnspecifiedName = "IMPORTANCE_UNSPECIFIED"
)

var (
	// idlock protects the nextNotificationID to ensure that no notification is ever
	// sent with a duplicate id.
	//
	// BUG(whereswaldon): Notification ID generation does not handle 32 bit integer
	// overflow. Sending more than 2 billion notifications results in undefined
	// behavior.
	idlock             sync.Mutex
	nextNotificationID int32

	// jvmConstLock protects the mapping of JVM constants that must be resolved at runtime
	jvmConstLock sync.Once
	// importances tracks the IMPORTANCE_* constants from the JVM's values. Since they must
	// be resolved at runtime, this array tracks their actual runtime values and the exported
	// constants are simply indicies into this array.
	importances [importanceEnd]int32
	// map the JVM constant name to the index in the array
	importancesMap = map[string]Importance{
		importanceDefaultName:     ImportanceDefault,
		importanceHighName:        ImportanceHigh,
		importanceLowName:         ImportanceLow,
		importanceMaxName:         ImportanceMax,
		importanceMinName:         ImportanceMin,
		importanceNoneName:        ImportanceNone,
		importanceUnspecifiedName: ImportanceUnspecified,
	}
)

// nextID safely returns the next unused notification id number. This function should
// always be used to get a notificationID.
func nextID() int32 {
	idlock.Lock()
	defer idlock.Unlock()
	id := nextNotificationID
	nextNotificationID++
	return id
}

// NotificationChannel represents a stream of notifications that an application
// provisions on android. Such streams can be selectively enabled and disabled
// by the user, and should be used for different purposes.
type NotificationChannel struct {
	id string
}

// NewChannel creates a new notification channel identified by the provided id
// and with the given user-visible name and description. The importance field
// specifies how the android system should prioritize notifications sent over this
// channel, and the value provided MUST be one of the constants declared by this
// package. The actual value of the importance constant is translated into the
// java value at runtime.
func NewChannel(importance Importance, id, name, description string) (*NotificationChannel, error) {
	if err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		appCtx := jni.Object(app.AppContext())
		classLoader := jni.ClassLoaderFor(env, appCtx)
		notifyClass, err := jni.LoadClass(env, classLoader, helperClass)
		if err != nil {
			return err
		}
		jvmConstLock.Do(func() {
			var managerClass jni.Class
			managerClass, err = jni.LoadClass(env, classLoader, "android/app/NotificationManager")
			if err != nil {
				return
			}
			for name, index := range importancesMap {
				fieldID := jni.GetStaticFieldID(env, managerClass, name, "I")
				importances[index] = jni.GetStaticIntField(env, managerClass, fieldID)
			}

		})
		newChannelMethod := jni.GetStaticMethodID(env, notifyClass, "newChannel", "(Landroid/content/Context;ILjava/lang/String;Ljava/lang/String;Ljava/lang/String;)V")
		jname := jni.Value(jni.JavaString(env, name))
		jdescription := jni.Value(jni.JavaString(env, description))
		jID := jni.Value(jni.JavaString(env, id))
		jimportance := jni.Value(importance.value())
		err = jni.CallStaticVoidMethod(env, notifyClass, newChannelMethod, jni.Value(app.AppContext()), jimportance, jID, jname, jdescription)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed creating notification channel: %w", err)
	}
	nc := &NotificationChannel{
		id: id,
	}
	return nc, nil
}

// Notification represents a notification that has been requested to be shown to the user.
// This type provides methods to cancel or update the contents of the notification.
type Notification struct {
	id int32
}

// Send creates a new Notification and requests that it be displayed on this channel.
// The onGoing field specifies if the notification should be ongoing. Ongoing
// notifications are ones that cannot be swiped away.
func (nc *NotificationChannel) Send(title, text string, onGoing bool) (*Notification, error) {
	notificationID := nextID()
	if err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		appCtx := jni.Object(app.AppContext())
		classLoader := jni.ClassLoaderFor(env, appCtx)
		notifyClass, err := jni.LoadClass(env, classLoader, helperClass)
		if err != nil {
			return err
		}
		// (Context ctx, String channelID, int notificationID, String title, String text, boolean onGoing
		newChannelMethod := jni.GetStaticMethodID(env, notifyClass, "sendNotification", "(Landroid/content/Context;Ljava/lang/String;ILjava/lang/String;Ljava/lang/String;Z)V")
		jtitle := jni.Value(jni.JavaString(env, title))
		jtext := jni.Value(jni.JavaString(env, text))
		jID := jni.Value(jni.JavaString(env, nc.id))
		jOnGoing := jni.FALSE
		if onGoing {
			jOnGoing = jni.TRUE
		}
		err = jni.CallStaticVoidMethod(env, notifyClass, newChannelMethod, jni.Value(app.AppContext()), jID, jni.Value(notificationID), jtitle, jtext, jni.Value(jOnGoing))
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed sending notification: %w", err)
	}
	return &Notification{
		id: notificationID,
	}, nil
}

// Cancel removes a previously created notification from display.
func (n *Notification) Cancel() error {
	notificationID := n.id
	if err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		appCtx := jni.Object(app.AppContext())
		classLoader := jni.ClassLoaderFor(env, appCtx)
		notifyClass, err := jni.LoadClass(env, classLoader, helperClass)
		if err != nil {
			return err
		}
		newChannelMethod := jni.GetStaticMethodID(env, notifyClass, "cancelNotification", "(Landroid/content/Context;I)V")
		return jni.CallStaticVoidMethod(env, notifyClass, newChannelMethod, jni.Value(app.AppContext()), jni.Value(notificationID))
	}); err != nil {
		return fmt.Errorf("failed cancelling notification: %w", err)
	}
	return nil
}
