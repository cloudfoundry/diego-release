package auctioncellrep_test

import (
	"code.cloudfoundry.org/rep/auctioncellrep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helpers", func() {
	Context("GenerateGuid", func() {
		It("generates a 28 character identifier", func() {
			guid, err := auctioncellrep.GenerateGuid()
			Expect(err).NotTo(HaveOccurred())
			Expect(guid).To(HaveLen(28))
		})
	})
})
