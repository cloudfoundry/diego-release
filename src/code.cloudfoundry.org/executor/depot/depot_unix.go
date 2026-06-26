//go:build !windows

package depot

import "syscall"

func liveDiskMB(path string) (int, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, false
	}
	return int(int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024)), true
}
