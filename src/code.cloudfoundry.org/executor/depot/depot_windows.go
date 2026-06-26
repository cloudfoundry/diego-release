//go:build windows

package depot

func liveDiskMB(path string) (int, bool) {
	return 0, false
}
