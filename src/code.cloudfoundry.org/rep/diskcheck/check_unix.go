//go:build linux

package diskcheck

import "golang.org/x/sys/unix"

// IsReadOnly reports whether the filesystem containing path is mounted read-only.
// It resolves any path — mountpoint or subdirectory — to the owning filesystem.
func IsReadOnly(path string) (bool, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return false, err
	}
	return stat.Flags&unix.ST_RDONLY != 0, nil
}
