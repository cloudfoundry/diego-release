package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	fake_auction_runner "code.cloudfoundry.org/auction/auctiontypes/fakes"
	"code.cloudfoundry.org/auctioneer/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("TaskAuctionHandler", func() {
	var (
		logger            *lagertest.TestLogger
		runner            *fake_auction_runner.FakeAuctionRunner
		responseRecorder  *httptest.ResponseRecorder
		handler           *handlers.TaskAuctionHandler
		requestIdHeader   string
		b3RequestIdHeader string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		runner = new(fake_auction_runner.FakeAuctionRunner)
		responseRecorder = httptest.NewRecorder()
		handler = handlers.NewTaskAuctionHandler(runner)
		requestIdHeader = "25f23d6a-f46d-460e-7135-7ddc0759a198"
		b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
	})

	Describe("Create", func() {
		Context("when the request body is a task", func() {
			var tasks []models.TaskStartRequest

			BeforeEach(func() {
				resource := models.NewResource(1, 2, 3)
				pc := models.NewPlacementConstraint("rootfs", []string{}, []string{})
				task := models.NewSchedulingTask("the-task-guid", "test", resource, pc)
				tasks = []models.TaskStartRequest{models.TaskStartRequest{Task: task}}
				req := newTestRequest(tasks)
				req.Header.Add(lager.RequestIdHeader, requestIdHeader)
				handler.Create(responseRecorder, req, logger)
			})

			It("responds with 202", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("responds with an empty JSON body", func() {
				Expect(responseRecorder.Body.String()).To(Equal("{}"))
			})

			It("should submit the task to the auction runner", func() {
				Expect(runner.ScheduleTasksForAuctionsCallCount()).To(Equal(1))

				submittedTasks, traceID := runner.ScheduleTasksForAuctionsArgsForCall(0)
				Expect(submittedTasks).To(Equal(tasks))
				Expect(traceID).To(Equal(requestIdHeader))
			})

			It("logs trace ID", func() {
				Expect(logger.Buffer()).To(Say(b3RequestIdHeader))
			})
		})

		Context("when the request body is a not a valid task", func() {
			var tasks []models.TaskStartRequest

			BeforeEach(func() {
				task := models.SchedulingTask{}
				tasks = []models.TaskStartRequest{models.TaskStartRequest{Task: task}}

				req := newTestRequest(tasks)
				req.Header.Add(lager.RequestIdHeader, requestIdHeader)
				handler.Create(responseRecorder, req, logger)
			})

			It("responds with 202", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("logs an error", func() {
				Expect(logger).To(Say("test.task-auction-handler.create.task-validate-failed"))
			})

			It("should submit the task to the auction runner", func() {
				Expect(runner.ScheduleTasksForAuctionsCallCount()).To(Equal(1))

				submittedTasks, traceID := runner.ScheduleTasksForAuctionsArgsForCall(0)
				Expect(submittedTasks).To(BeEmpty())
				Expect(traceID).To(Equal(requestIdHeader))
			})
		})

		Context("when the request body is a not a task", func() {
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

			It("should not submit the task to the auction runner", func() {
				Expect(runner.ScheduleTasksForAuctionsCallCount()).To(Equal(0))
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

			It("should not submit the task auction to the auction runner", func() {
				Expect(runner.ScheduleTasksForAuctionsCallCount()).To(Equal(0))
			})
		})
	})
})
