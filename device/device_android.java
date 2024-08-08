package org.gioui.x.device;

import android.app.Activity;
import android.view.View;

public class device_android {
    public static void setScreenAwake(View view, boolean isOn) {
        Activity activity = (Activity) view.getContext();
        activity.runOnUiThread(new Runnable() {
            public void run() {
                if(isOn){
                    activity.getWindow().addFlags(android.view.WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
                } else {
                    activity.getWindow().clearFlags(android.view.WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
                }
            }
        });
    }
}