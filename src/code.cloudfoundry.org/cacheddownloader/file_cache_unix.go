//go:build !windows

package cacheddownloader

import (
	"math"
	"syscall"
)

func defaultFreeSpaceOnPartition(path string) int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return math.MaxInt64
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}
