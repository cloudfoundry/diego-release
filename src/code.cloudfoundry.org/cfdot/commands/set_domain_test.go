package commands_test

import (
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("Set Domain", func() {
	Context("ValidateSetDomainArgs", func() {
		It("returns the domain", func() {
			domain, err := commands.ValidateSetDomainArgs([]string{"domain"})
			Expect(err).NotTo(HaveOccurred())
			Expect(domain).To(Equal("domain"))
		})

		Context("when no arguments are specified", func() {
			It("returns an error", func() {
				_, err := commands.ValidateSetDomainArgs([]string{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when too many arguments are specified", func() {
			It("returns an error", func() {
				_, err := commands.ValidateSetDomainArgs([]string{"domain1", "domain2"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the domain is empty", func() {
			It("returns an error", func() {
				_, err := commands.ValidateSetDomainArgs([]string{""})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("SetDomain", func() {
		var (
			fakeBBSClient  *fake_bbs.FakeClient
			stdout, stderr *gbytes.Buffer
		)

		BeforeEach(func() {
			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()
			fakeBBSClient = &fake_bbs.FakeClient{}
		})

		BeforeEach(func() {
			fakeBBSClient.UpsertDomainReturns(nil)
		})

		It("prints a success message when a domain is given", func() {
			err := commands.SetDomain(stdout, stderr, fakeBBSClient, "anything", 5*time.Second)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.UpsertDomainCallCount()).To(Equal(1))
			_, traceID, domain, ttl := fakeBBSClient.UpsertDomainArgsForCall(0)

			_, err = model.TraceIDFromHex(traceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain).To(Equal("anything"))
			Expect(ttl).To(Equal(5 * time.Second))
		})

		Context("when the bbs errors", func() {
			BeforeEach(func() {
				fakeBBSClient.UpsertDomainReturns(models.ErrUnknownError)
			})

			It("fails with a relevant error", func() {
				err := commands.SetDomain(stdout, stderr, fakeBBSClient, "anything", 0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrUnknownError))
			})
		})
	})
})
