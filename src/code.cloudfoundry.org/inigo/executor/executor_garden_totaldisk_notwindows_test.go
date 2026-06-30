//go:build !windows

package executor_test

import (
	"syscall"

	"code.cloudfoundry.org/executor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func registerDiskCapacityTest(resources *executor.ExecutorResources, cachePath *string, _ *int) {
	It("returns live disk space at cache path", Serial, func() {
		var stat syscall.Statfs_t
		Expect(syscall.Statfs(*cachePath, &stat)).To(Succeed())
		expected := int(int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024))
		Expect((*resources).DiskMB).To(Equal(expected))
	})
}
