package diskcheck_test

import (
	"os"

	"code.cloudfoundry.org/rep/diskcheck"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IsReadOnly", func() {
	It("reports that a writable temp directory is not read-only", func() {
		tmpDir, err := os.MkdirTemp("", "diskcheck-test-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		readOnly, err := diskcheck.IsReadOnly(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(readOnly).To(BeFalse())
	})
})
