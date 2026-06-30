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
