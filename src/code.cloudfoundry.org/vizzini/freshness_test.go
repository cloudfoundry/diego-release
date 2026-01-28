package vizzini_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Freshness", func() {
	Describe("Creating a fresh domain", func() {
		Context("with no TTL", func() {
			It("should create a fresh domain that never disappears", func() {
				Expect(bbsClient.UpsertDomain(logger, traceID, domain, 0)).To(Succeed())
				Consistently(func() ([]string, error) {
					return bbsClient.Domains(logger, traceID)
				}, 3).Should(ContainElement(domain))
				bbsClient.UpsertDomain(logger, traceID, domain, 1*time.Second) //to clear it out
			})
		})

		Context("with a TTL", func() {
			It("should create a fresh domain that eventually disappears", func() {
				Expect(bbsClient.UpsertDomain(logger, traceID, domain, 2*time.Second)).To(Succeed())

				Expect(bbsClient.Domains(logger, traceID)).To(ContainElement(domain))
				Eventually(func() ([]string, error) {
					return bbsClient.Domains(logger, traceID)
				}, 5).ShouldNot(ContainElement(domain))
			})
		})

		Context("with no domain", func() {
			It("should error", func() {
				Expect(bbsClient.UpsertDomain(logger, traceID, "", 0)).NotTo(Succeed())
			})
		})
	})
})
