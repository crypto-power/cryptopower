package device

import (
	"fmt"

	"gioui.org/app"
	"gioui.org/io/event"
	"git.wow.st/gmp/jni"
)

//go:generate javac -source 8 -target 8 -bootclasspath $ANDROID_HOME/platforms/android-30/android.jar -d $TEMP/x_device/classes device_android.java
//go:generate jar cf device_android.jar -C $TEMP/x_device/classes .

var (
	_Lib = "org/gioui/x/device/device_android"
)

type device struct {
	window *app.Window
	view   uintptr
	// libObject jni.Object
	libClass jni.Class
	methodID jni.MethodID
}

func newDevice(w *app.Window) *device {
	return &device{window: w}
}

func (d *Device) listenEvents(evt event.Event) {
	if evt, ok := evt.(app.AndroidViewEvent); ok {
		d.view = evt.View
	}
}

func (d *Device) init(env jni.Env) error {
	if d.libClass != 0 {
		return nil // Already initialized
	}
	context := app.AppContext()
	class, err := jni.LoadClass(env, jni.ClassLoaderFor(env, jni.Object(context)), _Lib)
	if err != nil {
		return err
	}
	d.libClass = class
	d.methodID = jni.GetStaticMethodID(env, class, "setScreenAwake", "(Landroid/view/View;Z)V")

	return nil
}

func (d *Device) setScreenAwake(isOn bool) error {
	d.window.Run(func() {
		err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
			if err := d.init(env); err != nil {
				return err
			}
			value := jni.FALSE
			if isOn {
				value = jni.TRUE
			}
			return jni.CallStaticVoidMethod(env, d.libClass, d.device.methodID,
				jni.Value(d.view),
				jni.Value(value),
			)
		})

		if err != nil {
			fmt.Println(err)
		}
	})
	return nil
}
