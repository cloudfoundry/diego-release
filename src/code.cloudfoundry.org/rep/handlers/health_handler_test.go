package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/handlers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("HealthHandler", func() {
	var healthServer *httptest.Server

	BeforeEach(func() {
		router, err := rata.NewRouter(rep.RoutesHealth, handlers.NewHealth(fakeExecutorClient, logger))
		Expect(err).NotTo(HaveOccurred())
		healthServer = httptest.NewServer(router)
	})

	AfterEach(func() {
		healthServer.Close()
	})

	getHealth := func() int {
		resp, err := http.Get(healthServer.URL + "/health")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		return resp.StatusCode
	}

	It("responds with 200 OK when the cell is healthy", func() {
		fakeExecutorClient.HealthyReturns(true)
		Expect(getHealth()).To(Equal(http.StatusOK))
	})

	It("responds with 503 Service Unavailable when the cell is unhealthy", func() {
		fakeExecutorClient.HealthyReturns(false)
		Expect(getHealth()).To(Equal(http.StatusServiceUnavailable))
	})

	It("does not serve the admin or auction routes", func() {
		resp, err := http.Get(healthServer.URL + "/ping")
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})
})
