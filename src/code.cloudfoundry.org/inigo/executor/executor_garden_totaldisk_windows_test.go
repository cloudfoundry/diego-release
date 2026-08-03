//go:build windows

package executor_test

import (
	"code.cloudfoundry.org/executor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func registerDiskCapacityTest(resources *executor.ExecutorResources, _ *string, expectedDiskCapacityMB *int) {
	It("returns static configured disk capacity", func() {
		Expect((*resources).DiskMB).To(Equal(*expectedDiskCapacityMB))
	})
}

// expectedRemainingDiskCapacityMB mirrors executor/depot's RemainingResources. Windows
// CI hosts are assumed to have ample free space matching the configured capacity, so no
// live check is performed here (matching registerDiskCapacityTest above).
func expectedRemainingDiskCapacityMB(_ string, configuredDiskCapacityMB int) int {
	return configuredDiskCapacityMB
}
