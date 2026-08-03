package databaseuri_test

import (
	"code.cloudfoundry.org/buildpackapplifecycle/databaseuri"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DatabaseURI", func() {
	var subject *databaseuri.Databaseuri
	BeforeEach(func() {
		subject = databaseuri.New()
	})

	Describe("Credentials", func() {
		It("returns credentials.uri", func() {
			services := []byte(`{"eg":[{"credentials":{"uri":"fred://jane.com/bill"}}]}`)
			Expect(subject.Credentials(services)).To(Equal([]string{"fred://jane.com/bill"}))
		})
		It("ignores services without credentials.uri", func() {
			services := []byte(`{"eg":[{}]}`)
			Expect(subject.Credentials(services)).To(BeEmpty())
		})
		It("returns multiple credentials", func() {
			services := []byte(`{
				"abc":[{"credentials":{"uri":"u1"}}],
				"def":[{"other":"data"}],
				"ghi":[{"credentials":{"other":"data"}}],
				"jkl":[{},{"credentials":{"uri":"u2"}}]
			}`)
			Expect(subject.Credentials(services)).To(ConsistOf("u1", "u2"))
		})
	})

	Describe("Uri", func() {
		Context("when there are relational database services", func() {
			It("and the uri is for mysql", func() {
				service_uris := []string{"mysql://username:password@host/db"}
				Expect(subject.Uri(service_uris)).To(Equal("mysql2://username:password@host/db"))
			})
			It("and the uri is for mysql2", func() {
				service_uris := []string{"mysql2://username:password@host/db"}
				Expect(subject.Uri(service_uris)).To(Equal("mysql2://username:password@host/db"))
			})
			It("and the uri is for postgres", func() {
				service_uris := []string{"postgres://username:password@host/db"}
				Expect(subject.Uri(service_uris)).To(Equal("postgres://username:password@host/db"))
			})
			It("and the uri is for postgresql", func() {
				service_uris := []string{"postgresql://username:password@host/db"}
				Expect(subject.Uri(service_uris)).To(Equal("postgres://username:password@host/db"))
			})
			It("and there are more than one production relational database", func() {
				service_uris := []string{"postgres://username:password@host/db1", "postgres://username:password@host/db2"}
				Expect(subject.Uri(service_uris)).To(Equal("postgres://username:password@host/db1"))
			})
			It("and the uri is invalid", func() {
				service_uris := []string{`postgresql://invalid:password@host/%a`}
				Expect(subject.Uri(service_uris)).To(Equal(""))
			})
		})
		It("when there are non relational databse services", func() {
			service_uris := []string{"sendgrid://foo:bar@host/db"}
			Expect(subject.Uri(service_uris)).To(Equal(""))
		})
		It("when there are no services", func() {
			service_uris := []string{}
			Expect(subject.Uri(service_uris)).To(Equal(""))
		})
	})
})
