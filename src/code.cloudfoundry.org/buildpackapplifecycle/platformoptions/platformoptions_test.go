package platformoptions_test

import (
	"code.cloudfoundry.org/buildpackapplifecycle/platformoptions"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Platformoptions", func() {
	var (
		platformOptions     *platformoptions.PlatformOptions
		err                 error
		vcapPlatformOptions string
	)

	JustBeforeEach(func() {
		platformOptions, err = platformoptions.Get(vcapPlatformOptions)
	})

	Context("when VCAP_PLATFORM_OPTIONS is an empty string", func() {
		BeforeEach(func() {
			vcapPlatformOptions = ""
		})

		It("returns nil PlatformOptions without error", func() {
			Expect(platformOptions).To(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when VCAP_PLATFORM_OPTIONS is an empty JSON object", func() {
		BeforeEach(func() {
			vcapPlatformOptions = "{}"
		})

		It("returns an unset PlatformOptions", func() {
			Expect(platformOptions).NotTo(BeNil())
			Expect(err).ToNot(HaveOccurred())
			Expect(platformOptions).To(Equal(&platformoptions.PlatformOptions{}))
		})
	})

	Context("when VCAP_PLATFORM_OPTIONS is an invalid JSON object", func() {
		BeforeEach(func() {
			vcapPlatformOptions = `{"credhub-uri":"missing quote and brace`
		})

		It("returns a nil PlatformOptions with an error", func() {
			Expect(platformOptions).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when VCAP_PLATFORM_OPTIONS is a valid JSON object", func() {
		BeforeEach(func() {
			vcapPlatformOptions = `{"credhub-uri":"valid_json"}`
		})

		It("returns populated PlatformOptions", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(platformOptions.CredhubURI).To(Equal("valid_json"))
		})

		It("returns the same populated PlatformOptions on subsequent invocations", func() {
			platformOptions, err = platformoptions.Get(vcapPlatformOptions)
			Expect(err).ToNot(HaveOccurred())
			Expect(platformOptions).NotTo(BeNil())
			Expect(platformOptions.CredhubURI).To(Equal("valid_json"))
		})
	})
})
