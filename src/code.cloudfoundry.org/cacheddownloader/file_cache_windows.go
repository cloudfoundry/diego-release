//go:build windows

package cacheddownloader

import "math"

func defaultFreeSpaceOnPartition(path string) int64 {
	return math.MaxInt64
}
