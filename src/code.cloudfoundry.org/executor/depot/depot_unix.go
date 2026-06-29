//go:build !windows
// +build !windows

package depot

import "syscall"

func getDiskMB(diskPath string, fallback int) int {
	if diskPath == "" {
		return fallback
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(diskPath, &stat); err == nil {
		return int(int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024))
	}
	return fallback
}
