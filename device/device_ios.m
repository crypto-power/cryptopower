#import <device_ios.h>
BOOL setScreenAwake(BOOL isOn){
    [UIApplication sharedApplication].idleTimerDisabled = isOn;
    return isOn;
}