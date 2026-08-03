//go:build windows

package main_test

import (
	"golang.org/x/sys/windows"

	. "github.com/onsi/gomega"
)

func expectedTotalDiskMB(cachePath string) int32 {
	pathPtr, err := windows.UTF16PtrFromString(cachePath)
	Expect(err).NotTo(HaveOccurred())
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	Expect(windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)).To(Succeed())
	return int32(freeBytesAvailable / (1024 * 1024))
}
