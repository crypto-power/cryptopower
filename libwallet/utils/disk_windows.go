//go:build windows
// +build windows

package utils

import (
	"golang.org/x/sys/windows"
)

// DiskSpace returns the total and free disk space in bytes for the specified directory
func DiskSpace(dir string) (free uint64, err error) {
	var (
		directoryName              = windows.StringToUTF16Ptr(dir)
		freeBytesAvailableToCaller uint64
		totalNumberOfBytes         uint64
		totalNumberOfFreeBytes     uint64
	)
	err = windows.GetDiskFreeSpaceEx(
		directoryName,
		&freeBytesAvailableToCaller,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes,
	)
	if err != nil {
		return 0, err
	}

	// Convert bytes to MB
	return totalNumberOfFreeBytes / (1024 * 1024), nil
}
