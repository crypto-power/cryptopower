//go:build darwin && cgo
// +build darwin,cgo

package macos

//#cgo LDFLAGS: -framework Foundation -framework UserNotifications

/*
#cgo CFLAGS: -x objective-c -fno-objc-arc -fmodules
#pragma clang diagnostic ignored "-Wformat-security"

#import <stdlib.h>

@import Foundation;
@import UserNotifications;

const unsigned char NIOTIFY_SUCCESS = 0;
const unsigned char NIOTIFY_NO_PERMISSION = 1;
const unsigned char NIOTIFY_NO_BUNDLE = 2;
const unsigned char NIOTIFY_UKNOWN_ERR = 3;

@interface UNDelegate : NSObject <UNUserNotificationCenterDelegate>
{ }
- (void)userNotificationCenter:(UNUserNotificationCenter *)center didReceiveNotificationResponse:(UNNotificationResponse *)response withCompletionHandler:(void (^)(void))completionHandler;
- (void)userNotificationCenter:(UNUserNotificationCenter *)center willPresentNotification:(UNNotification *)notification withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler;
- (void)userNotificationCenter:(UNUserNotificationCenter *)center openSettingsForNotification:(UNNotification *)notification;
@end

@implementation UNDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center didReceiveNotificationResponse:(UNNotificationResponse *)response withCompletionHandler:(void (^)(void))completionHandler {
	NSLog(@"didReceiveNotificationResponse");
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center willPresentNotification:(UNNotification *)notification withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler {
	NSLog(@"willPresentNotification");
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center openSettingsForNotification:(UNNotification *)notification {
	NSLog(@"openSettingsForNotification");
}
@end

UNUserNotificationCenter *nc;
BOOL enabled;
BOOL hasBundle;
UNDelegate *del;

void setup() {
	@autoreleasepool {
		NSLog(@"Getting application bundle");
		enabled = NO;
		hasBundle = NO;
		NSBundle *main = [NSBundle mainBundle];
		if (main.bundleIdentifier == nil) {
			NSLog(@"No app bundle.");
			return;
		}
		hasBundle = YES;
		NSLog(@"Bundle ID: %@", main.bundleIdentifier);
		NSLog(@"Getting notification center");
		nc = [UNUserNotificationCenter currentNotificationCenter];
		del = [[UNDelegate alloc] init];
		nc.delegate = del;
	}
}

NSString*
notify(char *id, char *title, char *content, unsigned char *errorCode) {
	NSString *ret;

	if (!hasBundle) {
    		*errorCode = NIOTIFY_NO_BUNDLE;
    		return ret;
	}

	@autoreleasepool {
		dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
		NSLog(@"Creating notification");
		UNMutableNotificationContent *note = [[UNMutableNotificationContent alloc] init];
		note.title = [[NSString alloc] initWithUTF8String: title];
		note.body = [[NSString alloc] initWithUTF8String: content];
		NSString *identifier = [[NSUUID UUID] UUIDString];

		NSLog(@"Creating request");
		UNNotificationRequest *req = [UNNotificationRequest requestWithIdentifier:identifier content: note trigger:nil];
		ret = req.identifier;
		[ret retain];
		NSLog(@"Requesting authorization");
		[nc requestAuthorizationWithOptions: UNAuthorizationOptionBadge | UNAuthorizationOptionSound | UNAuthorizationOptionAlert completionHandler: ^(BOOL granted, NSError *error){
			NSLog(@"Granted = %s", granted?"true":"false");
			NSLog(@"Error = %@", error);
			enabled = granted;
			if (enabled == YES) {
                		NSLog(@"Adding notification request");
                		[nc addNotificationRequest:req withCompletionHandler: ^(NSError *error) {
                			NSLog(@"added notification. Error: %@", error);
                			*errorCode=NIOTIFY_SUCCESS;
                			dispatch_semaphore_signal(semaphore);
                		}];
			} else {
    				*errorCode=NIOTIFY_NO_PERMISSION;
    				dispatch_semaphore_signal(semaphore);
			}
		}];
		dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
	}
	return ret;
}

void
cancel(void *nid, unsigned char *errorCode) {
	if (!hasBundle) {
    		*errorCode = NIOTIFY_NO_BUNDLE;
    		return;
	}

    @try {
	[nc removePendingNotificationRequestsWithIdentifiers: @[(NSString*)nid]];
	[nc removeDeliveredNotificationsWithIdentifiers: @[(NSString*)nid]];
    }
    @catch(NSException *ne) {
        NSLog(@"caught exception when cancelling notification %@: %@", nid, ne);
        *errorCode=NIOTIFY_UKNOWN_ERR;
    }
}

*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

func init() {
	runtime.LockOSThread()
	C.setup()
}

type NotificationChannel struct {
	id *C.char
}

func NewNotificationChannel(id string) NotificationChannel {
	return NotificationChannel{id: C.CString(id)}
}

func (c NotificationChannel) Send(title, text string) (*Notification, error) {
	return notify(c.id, title, text)
}

type Notification C.NSString

// toErr converts C integer error codes into Go errors with semi-useful string
// messages.
func toErr(errCode C.uchar) error {
	switch errCode {
	case C.NIOTIFY_SUCCESS:
		return nil
	case C.NIOTIFY_NO_PERMISSION:
		return fmt.Errorf("permission denied")
	case C.NIOTIFY_NO_BUNDLE:
		return fmt.Errorf("no app bundle detected")
	default:
		return fmt.Errorf("unknown error: %v", errCode)
	}
}

func notify(cid *C.char, title, content string) (*Notification, error) {
	ct := C.CString(title)
	defer C.free(unsafe.Pointer(ct))
	cc := C.CString(content)
	defer C.free(unsafe.Pointer(cc))
	var errCode C.uchar

	id := C.notify(cid, ct, cc, &errCode)
	err := toErr(errCode)
	if err != nil {
		return nil, err
	}

	return (*Notification)(id), nil
}

func (n *Notification) Cancel() error {
	if n == nil {
		return fmt.Errorf("attempted to cancel nil notification")
	}
	var errCode C.uchar
	C.cancel(unsafe.Pointer(n), &errCode)
	return toErr(errCode)
}
