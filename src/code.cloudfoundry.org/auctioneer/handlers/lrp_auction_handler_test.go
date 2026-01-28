package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	fake_auction_runner "code.cloudfoundry.org/auction/auctiontypes/fakes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/handlers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LRPAuctionHandler", func() {
	var (
		logger            *lagertest.TestLogger
		runner            *fake_auction_runner.FakeAuctionRunner
		responseRecorder  *httptest.ResponseRecorder
		handler           *handlers.LRPAuctionHandler
		requestIdHeader   string
		b3RequestIdHeader string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		runner = new(fake_auction_runner.FakeAuctionRunner)
		responseRecorder = httptest.NewRecorder()
		handler = handlers.NewLRPAuctionHandler(runner)
		requestIdHeader = "25f23d6a-f46d-460e-7135-7ddc0759a198"
		b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
	})

	Describe("Create", func() {
		Context("when the request body is an LRP start auction request", func() {
			var starts []auctioneer.LRPStartRequest

			BeforeEach(func() {
				starts = []auctioneer.LRPStartRequest{{
					Indices:     []int{2, 3},
					Domain:      "tests",
					ProcessGuid: "some-guid",
					Resource: rep.Resource{
						MemoryMB: 1024,
						DiskMB:   512,
					},
					PlacementConstraint: rep.PlacementConstraint{
						RootFs: "docker:///docker.com/docker",
					},
				}}

				req := newTestRequest(starts)
				req.Header.Add(lager.RequestIdHeader, requestIdHeader)
				handler.Create(responseRecorder, req, logger)
			})

			It("responds with 202", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("responds with an empty JSON body", func() {
				Expect(responseRecorder.Body.String()).To(Equal("{}"))
			})

			It("should submit the start auction to the auction runner", func() {
				Expect(runner.ScheduleLRPsForAuctionsCallCount()).To(Equal(1))

				submittedStart, traceID := runner.ScheduleLRPsForAuctionsArgsForCall(0)
				Expect(submittedStart).To(Equal(starts))
				Expect(traceID).To(Equal(requestIdHeader))
			})

			It("should log the list of lrps as a json object with guid and indices keys", func() {
				Expect(logger.Buffer()).To(gbytes.Say(`"guid":"some-guid","indices":\[2,3\]`))
			})

			It("logs trace ID", func() {
				Expect(logger.Buffer()).To(gbytes.Say(b3RequestIdHeader))
			})
		})

		Context("when the start auction has invalid index", func() {
			var start auctioneer.LRPStartRequest

			BeforeEach(func() {
				start = auctioneer.LRPStartRequest{}

				handler.Create(responseRecorder, newTestRequest(start), logger)
			})

			It("responds with 400", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("responds with a JSON body containing the error", func() {
				handlerError := handlers.HandlerError{}
				err := json.NewDecoder(responseRecorder.Body).Decode(&handlerError)
				Expect(err).NotTo(HaveOccurred())
				Expect(handlerError.Error).NotTo(BeEmpty())
			})

			It("should not submit the start auction to the auction runner", func() {
				Expect(runner.ScheduleLRPsForAuctionsCallCount()).To(Equal(0))
			})
		})

		Context("when the request body is a not a start auction", func() {
			BeforeEach(func() {
				handler.Create(responseRecorder, newTestRequest(`{invalidjson}`), logger)
			})

			It("responds with 400", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("responds with a JSON body containing the error", func() {
				handlerError := handlers.HandlerError{}
				err := json.NewDecoder(responseRecorder.Body).Decode(&handlerError)
				Expect(err).NotTo(HaveOccurred())
				Expect(handlerError.Error).NotTo(BeEmpty())
			})

			It("should not submit the start auction to the auction runner", func() {
				Expect(runner.ScheduleLRPsForAuctionsCallCount()).To(Equal(0))
			})
		})

		Context("when the request body returns a non-EOF error on read", func() {
			BeforeEach(func() {
				req := newTestRequest("")
				req.Body = badReader{}
				handler.Create(responseRecorder, req, logger)
			})

			It("responds with 500", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
			})

			It("responds with a JSON body containing the error", func() {
				handlerError := handlers.HandlerError{}
				err := json.NewDecoder(responseRecorder.Body).Decode(&handlerError)
				Expect(err).NotTo(HaveOccurred())
				Expect(handlerError.Error).To(Equal(ErrBadRead.Error()))
			})

			It("should not submit the start auction to the auction runner", func() {
				Expect(runner.ScheduleLRPsForAuctionsCallCount()).To(Equal(0))
			})
		})
	})
})
