//go:build !windows

package main_test

import (
	"syscall"

	. "github.com/onsi/gomega"
)

func expectedTotalDiskMB(cachePath string) int32 {
	var stat syscall.Statfs_t
	Expect(syscall.Statfs(cachePath, &stat)).To(Succeed())
	return int32(int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024))
}
