package org.gioui.x.lifecycle;

import android.app.Activity;
import android.app.Application;
import android.os.Bundle;
import android.util.Log;

public class AppLifecycleListener implements Application.ActivityLifecycleCallbacks {
    private static AppLifecycleListener instance;

    public static native void onAppBackground();
    public static native void onAppForeground();

    public static void register(Application app) {
        if (instance == null) {
            instance = new AppLifecycleListener();
            app.registerActivityLifecycleCallbacks(instance);
        }
    }

    @Override
    public void onActivityResumed(Activity activity) {
        Log.d("GioUI", "The application is back in the foreground.");
        onAppForeground();
    }

    @Override
    public void onActivityPaused(Activity activity) {
        Log.d("GioUI", "The application is running in the background.");
        onAppBackground();
    }

    @Override
    public void onActivityCreated(Activity activity, Bundle savedInstanceState) {
    }

    @Override
    public void onActivityStarted(Activity activity) {
    }

    @Override
    public void onActivityStopped(Activity activity) {
    }

    @Override
    public void onActivitySaveInstanceState(Activity activity, Bundle outState) {
    }

    @Override
    public void onActivityDestroyed(Activity activity) {
    }
}
