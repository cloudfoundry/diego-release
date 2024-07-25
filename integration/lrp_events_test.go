package integration_test

import (
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"google.golang.org/protobuf/proto"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("lrp-events", func() {
	itValidatesBBSFlags("lrp-events")
	itHasNoArgs("lrp-events", false)

	Context("events filtering by cell id", func() {
		Context("when the cell id is specified", func() {
			BeforeEach(func() {
				expectedRequest := &models.EventsByCellId{CellId: "some-cell-id"}
				expectedBody, err := proto.Marshal(expectedRequest.ToProto())
				Expect(err).NotTo(HaveOccurred())
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/events.r1"),
						ghttp.VerifyBody(expectedBody),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
						ghttp.VerifyBody(expectedBody),
					),
				)
				sess := RunCFDot("lrp-events", "-c", "some-cell-id")
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("passes the cell id to the bbs client", func() {
				Eventually(bbsServer.ReceivedRequests).Should(HaveLen(2))
			})
		})

		Context("when the cell id is not specified", func() {
			BeforeEach(func() {
				expectedRequest := &models.EventsByCellId{CellId: ""}
				expectedBody, err := proto.Marshal(expectedRequest.ToProto())
				Expect(err).NotTo(HaveOccurred())
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/events.r1"),
						ghttp.VerifyBody(expectedBody),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
						ghttp.VerifyBody(expectedBody),
					),
				)
				sess := RunCFDot("lrp-events")
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("passes empty cell id to the bbs client", func() {
				Eventually(bbsServer.ReceivedRequests).Should(HaveLen(2))
			})
		})

	})

	Context("when the server responds with events", func() {
		BeforeEach(func() {
			actualLrp := model_helpers.NewValidActualLRP("some-process-guid", 0)
			//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
			lrpCreatedEvent := models.NewActualLRPCreatedEvent(actualLrp.ToActualLRPGroup())
			sseEvent, err := events.NewEventFromModelEvent(1, lrpCreatedEvent)
			Expect(err).ToNot(HaveOccurred())
			lrpInstanceCreatedEvent := models.NewActualLRPInstanceCreatedEvent(actualLrp, "some-trace-id")
			sseInstanceEvent, err := events.NewEventFromModelEvent(1, lrpInstanceCreatedEvent)
			Expect(err).ToNot(HaveOccurred())

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/events.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(200, sseInstanceEvent.Encode()),
				),
			)
		})

		It("prints out the event stream", func() {
			sess := RunCFDot("lrp-events")
			Eventually(sess).Should(gexec.Exit(0))
			stdOut := string(sess.Out.Contents())
			//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
			Expect(stdOut).To(ContainSubstring(models.EventTypeActualLRPCreated))
			Expect(stdOut).To(ContainSubstring(models.EventTypeActualLRPInstanceCreated))
		})
	})

	Context("when ActualLRPGroup events are not excluded", func() {
		BeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/events.r1"),
					ghttp.RespondWith(200, []byte{}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(200, []byte{}),
				),
			)
		})

		It("prints a warning message to stderr that	group events are deprecated", func() {
			sess := RunCFDot("lrp-events")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Err).To(gbytes.Say(`Event types "actual_lrp_created", "actual_lrp_changed" and "actual_lrp_removed" are deprecated. Use "--exclude-actual-lrp-groups" flag to exclude them.`))
		})
	})

	Context("when ActualLRPGroup events are excluded", func() {
		BeforeEach(func() {
			actualLRP := model_helpers.NewValidActualLRP("some-process-guid", 0)
			actualLRPEvent := models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id")
			sseEvent, err := events.NewEventFromModelEvent(1, actualLRPEvent)
			Expect(err).ToNot(HaveOccurred())

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
			)
		})

		It("does not print out ActualLRPGroup events or query the ActualLRPGroup events endpoint", func() {
			sess := RunCFDot("lrp-events", "--exclude-actual-lrp-groups")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("some-process-guid"))
			Expect(sess.Out).NotTo(gbytes.Say("some-process-guid"))
		})
	})

	Context("when both an ActualLRP Instance event and an ActualLRPGroup event are reported by the instance event stream and legacy event stream respectively", func() {
		BeforeEach(func() {
			actualLRP := model_helpers.NewValidActualLRP("some-process-guid", 0)
			//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
			actualLRPEvent1 := models.NewActualLRPCreatedEvent(actualLRP.ToActualLRPGroup())
			actualLRPEvent2 := models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id")
			sseEvent1, err := events.NewEventFromModelEvent(1, actualLRPEvent1)
			Expect(err).ToNot(HaveOccurred())
			sseEvent2, err := events.NewEventFromModelEvent(1, actualLRPEvent2)
			Expect(err).ToNot(HaveOccurred())

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/events.r1"),
					ghttp.RespondWith(200, sseEvent1.Encode()),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(200, sseEvent2.Encode()),
				),
			)
		})

		It("prints out multiple events", func() {
			sess := RunCFDot("lrp-events")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("some-process-guid"))
			Expect(sess.Out).To(gbytes.Say("some-process-guid"))
		})
	})

	Context("when duplicate ActualLRPCrashed events are reported by the instance event stream and legacy event stream", func() {
		BeforeEach(func() {
			actualLRP := model_helpers.NewValidActualLRP("some-process-guid", 0)
			actualLRP2 := model_helpers.NewValidActualLRP("other-guid", 0)
			actualLRPEvent := models.NewActualLRPCrashedEvent(actualLRP, actualLRP2)
			sseEvent, err := events.NewEventFromModelEvent(1, actualLRPEvent)
			Expect(err).ToNot(HaveOccurred())

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/events.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
			)
		})

		It("prints out a single event", func() {
			sess := RunCFDot("lrp-events")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("other-guid"))
			Expect(sess.Out).NotTo(gbytes.Say("some-process-guid"))
		})
	})

	Context("when duplicate DesiredLRP events are reported by the instance event stream and legacy event stream", func() {
		BeforeEach(func() {
			lrp := models.DesiredLRP{ProcessGuid: "some-process-guid"}
			desiredLRPEvent := models.NewDesiredLRPRemovedEvent(&lrp, "some-trace-id")
			sseEvent, err := events.NewEventFromModelEvent(1, desiredLRPEvent)
			Expect(err).ToNot(HaveOccurred())

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/events.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
			)
		})

		It("prints out a single event", func() {
			sess := RunCFDot("lrp-events")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("some-process-guid"))
			Expect(sess.Out).NotTo(gbytes.Say("some-process-guid"))
		})
	})

	Context("when there is a BBS error", func() {
		BeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/events.r1"),
					ghttp.RespondWith(418, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/lrp_instances.r1"),
					ghttp.RespondWith(418, ""),
				),
			)
		})

		It("responds with a status code 4", func() {
			sess := RunCFDot("lrp-events")
			Eventually(sess).Should(gexec.Exit(4))
		})
	})
})
