# Building Cryptopower Wallet for Mobile

This readme assumes you have a working Android or iOS environment.

## 1. Building for Android

Note: To build Cryptopower for Android, you need to have;

1.  gogio
2.  [Android SDK with NDK bundle](https://developer.android.com/tools)
3.  The ANDROID_SDK_ROOT point to the SDK root directory e.g `export ANDROID_SDK_ROOT=$HOME/.local/share/Android/Sdk`

Proceed to the [gioui Android doc](https://gioui.org/doc/install/android) for more info regarding gogio and other dependencies you will be needing.

execute the command below in a terminal window to install gogio:

`go install gioui.org/cmd/gogio@latest`

cd to the cryptopower root directory and execute the command below to generate a .apk file:

`gogio -target android .`

there should now be a cryptopower.apk file in the cryptopower root directory. You can send this file to your android device and install it.

or to send it to your device automatically, execute the command below:

`adb install cryptopower.apk`

## 2. Building for iOS

Note: To build Cryptopower for iOS, you need to have;

1. gogio
2. [Xcode](https://developer.apple.com/xcode/)

Proceed to the [gioui iOS doc](https://gioui.org/doc/install/ios) for more info regarding gogio and other dependencies you will be needing.

execute the command below in a terminal window to install gogio:

`go install gioui.org/cmd/gogio@latest`

cd to the cryptopower root directory and execute the command below to generate a .app file:

`gogio -o cryptopower.app -target ios .`

there should now be a cryptopower.app file in the cryptopower root directory. You can send this file to your iOS simulator.

or to send it to your simulator automatically, execute the command below:

`xcrun simctl install booted cryptopower.app`
