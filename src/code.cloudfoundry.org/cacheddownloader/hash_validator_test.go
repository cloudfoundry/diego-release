package cacheddownloader_test

import (
	"crypto/md5"
	"fmt"

	"code.cloudfoundry.org/cacheddownloader"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HashValidator", func() {

	algorithms := []string{"md5", "sha1", "sha256"}

	validateAlgorithm := func(algorithm string) {
		It("should create a hash validator", func() {
			validator, err := cacheddownloader.NewHashValidator(algorithm)
			Expect(err).NotTo(HaveOccurred())
			Expect(validator).NotTo(BeNil())
		})
	}

	Context("when creating a new validaotr", func() {
		for _, algorithm := range algorithms {
			validateAlgorithm(algorithm)
		}

		It("should fail with an unknown algorithm", func() {
			validator, err := cacheddownloader.NewHashValidator("unknown algorithm")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("algorithm invalid"))
			Expect(validator).To(BeNil())
		})
	})

	Context("when calculating hex value", func() {
		msg := "manifests kill people"

		It("should validate correct md5", func() {
			value, err := cacheddownloader.HexValue("md5", msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(fmt.Sprintf(`"%x"`, md5.Sum([]byte(msg)))).To(Equal(value))
		})

		It("should invalidate incorrect md5", func() {
			value, err := cacheddownloader.HexValue("md5", "wrong value")
			Expect(err).NotTo(HaveOccurred())
			Expect(fmt.Sprintf(`"%x"`, md5.Sum([]byte(msg)))).NotTo(Equal(value))
		})
	})
})
