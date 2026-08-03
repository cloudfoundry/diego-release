package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	fake_auction_runner "code.cloudfoundry.org/auction/auctiontypes/fakes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/handlers"
	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auction Handlers", func() {
	var (
		logger           *lagertest.TestLogger
		runner           *fake_auction_runner.FakeAuctionRunner
		responseRecorder *httptest.ResponseRecorder
		handler          http.Handler
		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		runner = new(fake_auction_runner.FakeAuctionRunner)
		responseRecorder = httptest.NewRecorder()

		fakeMetronClient = &mfakes.FakeIngressClient{}

		handler = handlers.New(logger, runner, fakeMetronClient)
	})

	Describe("Task Handler", func() {
		Context("with a valid task", func() {
			BeforeEach(func() {
				resource := models.NewResource(1, 2, 3)
				pc := models.NewPlacementConstraint("rootfs", []string{}, []string{})
				task := models.NewSchedulingTask("the-task-guid", "test", resource, pc)

				tasks := []models.TaskStartRequest{models.TaskStartRequest{Task: task}}
				reqGen := rata.NewRequestGenerator("http://localhost", auctioneer.Routes)

				payload, err := json.Marshal(tasks)
				Expect(err).NotTo(HaveOccurred())

				req, err := reqGen.CreateRequest(auctioneer.CreateTaskAuctionsRoute, rata.Params{}, bytes.NewBuffer(payload))
				Expect(err).NotTo(HaveOccurred())

				handler.ServeHTTP(responseRecorder, req)
			})

			It("responds with 202", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("logs with the correct session nesting", func() {
				Expect(logger.TestSink.LogMessages()).To(Equal([]string{
					"test.request.serving",
					"test.request.task-auction-handler.create.submitted",
					"test.request.done",
				}))
			})

			It("sends the correct metrics", func() {
				Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
				name, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
				Expect(name).To(Equal("RequestLatency"))
				Expect(value).To(BeNumerically(">", 0))

				Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(1))
				name = fakeMetronClient.IncrementCounterArgsForCall(0)
				Expect(name).To(Equal("RequestCount"))
			})
		})
	})

	Describe("LRP Handler", func() {
		Context("with a valid LRPStart", func() {
			BeforeEach(func() {
				starts := []models.LRPStartRequest{{
					Indices:     []int{2},
					Domain:      "tests",
					ProcessGuid: "some-guid",
					Resource: models.Resource{
						MemoryMB: 1024,
						DiskMB:   512,
					},
					PlacementConstraint: models.PlacementConstraint{
						RootFs: "docker:///docker.com/docker",
					},
				}}

				reqGen := rata.NewRequestGenerator("http://localhost", auctioneer.Routes)

				payload, err := json.Marshal(starts)
				Expect(err).NotTo(HaveOccurred())

				req, err := reqGen.CreateRequest(auctioneer.CreateLRPAuctionsRoute, rata.Params{}, bytes.NewBuffer(payload))
				Expect(err).NotTo(HaveOccurred())

				handler.ServeHTTP(responseRecorder, req)
			})

			It("responds with 202", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("logs with the correct session nesting", func() {
				Expect(logger.TestSink.LogMessages()).To(Equal([]string{
					"test.request.serving",
					"test.request.lrp-auction-handler.create.submitted",
					"test.request.done",
				}))
			})

			It("sends the correct metrics", func() {
				Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
				name, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
				Expect(name).To(Equal("RequestLatency"))
				Expect(value).To(BeNumerically(">", 0))

				Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(1))
				name = fakeMetronClient.IncrementCounterArgsForCall(0)
				Expect(name).To(Equal("RequestCount"))
			})
		})
	})
})
