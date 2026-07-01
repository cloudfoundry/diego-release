//go:build windows
// +build windows

package depot

import "golang.org/x/sys/windows"

func getDiskMB(diskPath string, fallback int) int {
	if diskPath == "" {
		return fallback
	}
	pathPtr, err := windows.UTF16PtrFromString(diskPath)
	if err != nil {
		return fallback
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return fallback
	}
	return int(freeBytesAvailable / (1024 * 1024))
}
