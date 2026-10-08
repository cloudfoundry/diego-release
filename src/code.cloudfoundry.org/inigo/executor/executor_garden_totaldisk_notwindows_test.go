//go:build !windows

package executor_test

import (
	"syscall"

	"code.cloudfoundry.org/executor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func registerDiskCapacityTest(resources *executor.ExecutorResources, _ *string, expectedDiskCapacityMB *int) {
	It("returns static configured disk capacity", func() {
		Expect((*resources).DiskMB).To(Equal(*expectedDiskCapacityMB))
	})
}

// expectedRemainingDiskCapacityMB mirrors executor/depot's RemainingResources, which
// caps the configured/reserved capacity at the live free space on the cache partition.
func expectedRemainingDiskCapacityMB(cachePath string, configuredDiskCapacityMB int) int {
	live := liveFreeDiskMB(cachePath)
	if live < configuredDiskCapacityMB {
		return live
	}
	return configuredDiskCapacityMB
}

func liveFreeDiskMB(cachePath string) int {
	var stat syscall.Statfs_t
	Expect(syscall.Statfs(cachePath, &stat)).To(Succeed())
	return int(int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024))
}
