//go:build windows
// +build windows

package depot

func getDiskMB(_ string, fallback int) int {
	return fallback
}
