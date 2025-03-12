//go:build android
// +build android

package lifecycle

/*
#include <jni.h>

// Declare C function to avoid implicit declaration error
void Java_org_gioui_x_lifecycle_AppLifecycleListener_onAppForeground();
void Java_org_gioui_x_lifecycle_AppLifecycleListener_onAppBackground();
*/
import "C"
import (
	giouiApp "gioui.org/app"
	"git.wow.st/gmp/jni"
)

//export Java_org_gioui_x_lifecycle_AppLifecycleListener_onAppForeground
func Java_org_gioui_x_lifecycle_AppLifecycleListener_onAppForeground() {
	// TODO: Handling when the application returns to the foreground
}

//export Java_org_gioui_x_lifecycle_AppLifecycleListener_onAppBackground
func Java_org_gioui_x_lifecycle_AppLifecycleListener_onAppBackground() {
	// TODO: Handling when the application runs in the background
}

const (
	lifecycleClass = "org/gioui/x/lifecycle/AppLifecycleListener"
)

func RegisterAppLifecycle() {
	jni.Do(jni.JVMFor(giouiApp.JavaVM()), func(env jni.Env) error {
		appCtx := jni.Object(giouiApp.AppContext())
		classLoader := jni.ClassLoaderFor(env, appCtx)
		activityClass, err := jni.LoadClass(env, classLoader, lifecycleClass)
		if err != nil {
			return err
		}
		// register lifecycle on android device
		registerMethod := jni.GetStaticMethodID(env, activityClass, "register", "(Landroid/app/Application;)V")
		jni.CallStaticVoidMethod(env, activityClass, registerMethod, jni.Value(giouiApp.AppContext()))
		return nil
	})
}
