//go:build linux || android || ios || darwin
// +build linux android ios darwin

package utils

import (
	"syscall"
)

// DiskSpace returns the available disk space in MB for the specified path
func DiskSpace(path string) (uint64, error) {

	var stat syscall.Statfs_t

	// Get file system statistics for the specified path
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}

	// Calculate available space: free blocks * block size
	freeBytes := stat.Bavail * uint64(stat.Bsize)

	// Convert bytes to MB
	return freeBytes / (1024 * 1024), nil
}
