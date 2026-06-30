//go:build windows

package main_test

func expectedTotalDiskMB(_ string) int32 {
	return 10 * 1024
}
