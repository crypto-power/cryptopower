# Building Cryptopower Wallet for Mobile

This readme assumes you have a working Android or iOS environment.

## 1. Building for Android

Note: You need to have [gogio](https://gioui.org/doc/install/android) to build.

execute the command below in a terminal window to install gogio:

`go install gioui.org/cmd/gogio@latest`

cd to the cryptopower root directory and execute the command below to generate a .apk file:

`gogio -target android .`

there should now be a cryptopower.apk file in the cryptopower root directory. You can send this file to your android device and install it.

or to send it to your device autmatically, execute the command below:

`adb install cryptopower.apk`

## 2. Building for iOS

Note: You need to have [gogio](https://gioui.org/doc/install/ios) to build.

execute the command below in a terminal window to install gogio:

`go install gioui.org/cmd/gogio@latest`

cd to the cryptopower root directory and execute the command below to generate a .app file:

`gogio -o cryptopower.app -target ios .`

there should now be a cryptopower.app file in the cryptopower root directory. You can send this file to your iOS simulator.

or to send it to your simulator autmatically, execute the command below:

`xcrun simctl install booted cryptopower.app`