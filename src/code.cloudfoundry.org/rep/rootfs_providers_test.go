package rep_test

import (
	"encoding/json"
	"net/url"

	models "code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RootFSProviders", func() {
	var (
		arbitrary models.ArbitraryRootFSProvider
		fixedSet  models.FixedSetRootFSProvider
		providers models.RootFSProviders

		providersJSON string
	)

	BeforeEach(func() {
		arbitrary = models.ArbitraryRootFSProvider{}
		fixedSet = models.NewFixedSetRootFSProvider("baz", "quux")
		providers = models.RootFSProviders{
			"foo": arbitrary,
			"bar": fixedSet,
		}

		providersJSON = `{
				"foo": {
					"type": "arbitrary"
				},
				"bar": {
					"type": "fixed_set",
					"set": {"baz":{}, "quux":{}}
				}
			}`

	})

	It("serializes", func() {
		payload, err := json.Marshal(providers)
		Expect(err).NotTo(HaveOccurred())

		Expect(payload).To(MatchJSON(providersJSON))
	})

	It("deserializes", func() {
		var providersResult models.RootFSProviders
		err := json.Unmarshal([]byte(providersJSON), &providersResult)
		Expect(err).NotTo(HaveOccurred())

		Expect(providersResult).To(Equal(providers))
	})

	Describe("Match", func() {
		Describe("ArbitraryRootFSProvider", func() {
			It("matches any URL", func() {
				rootFS, err := url.Parse("some://url")
				Expect(err).NotTo(HaveOccurred())

				Expect(arbitrary.Match(*rootFS)).To(BeTrue())
			})
		})

		Describe("FixedSetRootFSProvider", func() {
			It("matches a URL in the set", func() {
				rootFS, err := url.Parse("some:baz")
				Expect(err).NotTo(HaveOccurred())

				Expect(fixedSet.Match(*rootFS)).To(BeTrue())
			})

			It("does not match a URL not in the set", func() {
				rootFS, err := url.Parse("some://baz-not-present/here")
				Expect(err).NotTo(HaveOccurred())

				Expect(fixedSet.Match(*rootFS)).To(BeFalse())
			})
		})

		Describe("RootFSProviders", func() {
			Context("for a scheme with an arbitrary provider", func() {
				It("matches any url", func() {
					rootFS, err := url.Parse("foo://any/url/is#ok")
					Expect(err).NotTo(HaveOccurred())

					Expect(providers.Match(*rootFS)).To(BeTrue())
				})
			})

			Context("for a scheme with a fixed-set provider", func() {
				It("matches for a url in the set", func() {
					rootFS, err := url.Parse("bar:quux")
					Expect(err).NotTo(HaveOccurred())

					Expect(providers.Match(*rootFS)).To(BeTrue())
				})

				It("does not match for a url not in the set", func() {
					rootFS, err := url.Parse("bar:quux/not?in=theset")
					Expect(err).NotTo(HaveOccurred())

					Expect(providers.Match(*rootFS)).To(BeFalse())
				})
			})

			Context("for a scheme not in the map", func() {
				It("does not match", func() {
					rootFS, err := url.Parse("missingscheme://host/path")
					Expect(err).NotTo(HaveOccurred())

					Expect(providers.Match(*rootFS)).To(BeFalse())
				})
			})
		})
	})
})
