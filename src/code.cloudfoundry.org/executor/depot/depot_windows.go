//go:build windows

package depot

import "golang.org/x/sys/windows"

func liveDiskMB(path string) (int, bool) {
	if path == "" {
		return 0, false
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, false
	}
	return int(freeBytesAvailable / (1024 * 1024)), true
}
