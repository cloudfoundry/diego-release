package bbsresolver_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/spiffe-agent/bbsresolver"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	var (
		fakeBBS  *fake_bbs.FakeInternalClient
		resolver *bbsresolver.Resolver
		ctx      context.Context
	)

	const cellID = "cell-1"

	BeforeEach(func() {
		fakeBBS = new(fake_bbs.FakeInternalClient)
		logger := lagertest.NewTestLogger("bbsresolver")
		resolver = bbsresolver.New(fakeBBS, cellID, logger)
		ctx = context.Background()
	})

	Describe("ProcessType", func() {
		It("resolves the handle to its process type", func() {
			fakeBBS.ActualLRPsReturns([]*models.ActualLRP{{
				ActualLRPKey:         models.ActualLRPKey{ProcessGuid: "pg"},
				ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "handle-1"},
			}}, nil)
			fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{
				MetricTags: map[string]*models.MetricTagValue{"process_type": {Static: "web"}},
			}, nil)

			processType, err := resolver.ProcessType(ctx, "handle-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(processType).To(Equal("web"))
		})

		It("filters ActualLRPs to the resolver's cell", func() {
			fakeBBS.ActualLRPsReturns([]*models.ActualLRP{{
				ActualLRPKey:         models.ActualLRPKey{ProcessGuid: "pg"},
				ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "handle-1"},
			}}, nil)
			fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{
				MetricTags: map[string]*models.MetricTagValue{"process_type": {Static: "web"}},
			}, nil)

			_, err := resolver.ProcessType(ctx, "handle-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBBS.ActualLRPsCallCount()).To(Equal(1))
			_, _, filter := fakeBBS.ActualLRPsArgsForCall(0)
			Expect(filter.CellID).To(Equal(cellID))
			_, _, processGuid := fakeBBS.DesiredLRPByProcessGuidArgsForCall(0)
			Expect(processGuid).To(Equal("pg"))
		})

		It("returns an error when no LRP on the cell matches the handle", func() {
			fakeBBS.ActualLRPsReturns([]*models.ActualLRP{{
				ActualLRPKey:         models.ActualLRPKey{ProcessGuid: "pg"},
				ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "other-handle"},
			}}, nil)

			_, err := resolver.ProcessType(ctx, "handle-1")

			Expect(err).To(HaveOccurred())
			Expect(fakeBBS.DesiredLRPByProcessGuidCallCount()).To(Equal(0))
		})

		It("propagates BBS errors from ActualLRPs", func() {
			fakeBBS.ActualLRPsReturns(nil, errors.New("boom"))

			_, err := resolver.ProcessType(ctx, "handle-1")

			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the process_type metric tag is missing", func() {
			fakeBBS.ActualLRPsReturns([]*models.ActualLRP{{
				ActualLRPKey:         models.ActualLRPKey{ProcessGuid: "pg"},
				ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "handle-1"},
			}}, nil)
			fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{}, nil)

			_, err := resolver.ProcessType(ctx, "handle-1")

			Expect(err).To(HaveOccurred())
		})
	})
})
