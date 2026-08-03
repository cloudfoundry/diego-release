package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/locket/metrics/helpers/helpersfakes"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/auctioncellrep/auctioncellrepfakes"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/handlers"
	"code.cloudfoundry.org/rep/handlers/handlersfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

func TestAuctionHttpHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AuctionHttpHandlers Suite")
}

var (
	server              *httptest.Server
	requestGenerator    *rata.RequestGenerator
	client              *http.Client
	fakeLocalRep        *auctioncellrepfakes.FakeAuctionCellClient
	fakeMetricCollector *handlersfakes.FakeMetricCollector
	fakeExecutorClient  *executorfakes.FakeClient
	fakeEvacuatable     *fake_evacuation_context.FakeEvacuatable
	fakeRequestMetrics  *helpersfakes.FakeRequestMetrics
	logger              *lagertest.TestLogger
)

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("handlers")
	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

	fakeLocalRep = new(auctioncellrepfakes.FakeAuctionCellClient)
	fakeMetricCollector = new(handlersfakes.FakeMetricCollector)
	fakeExecutorClient = new(executorfakes.FakeClient)
	fakeEvacuatable = new(fake_evacuation_context.FakeEvacuatable)
	fakeRequestMetrics = new(helpersfakes.FakeRequestMetrics)

	handler, err := rata.NewRouter(rep.Routes, handlers.NewLegacy(fakeLocalRep, fakeMetricCollector, fakeExecutorClient, fakeEvacuatable, fakeRequestMetrics, logger))
	Expect(err).NotTo(HaveOccurred())

	server = httptest.NewServer(handler)
	requestGenerator = rata.NewRequestGenerator(server.URL, rep.Routes)
	client = new(http.Client)
})

var _ = AfterEach(func() {
	server.Close()
})

func JSONFor(obj interface{}) string {
	marshalled, err := json.Marshal(obj)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return string(marshalled)
}

func JSONReaderFor(obj interface{}) io.Reader {
	marshalled, err := json.Marshal(obj)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return bytes.NewBuffer(marshalled)
}

func Request(name string, params rata.Params, body io.Reader) (statusCode int, responseBody []byte) {
	request, err := requestGenerator.CreateRequest(name, params, body)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	response, err := client.Do(request)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	responseBody, err = io.ReadAll(response.Body)
	response.Body.Close()

	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return response.StatusCode, responseBody
}

func RequestTracing(name string, params rata.Params, body io.Reader, requestIdHeader string) (statusCode int, responseBody []byte) {
	request, err := requestGenerator.CreateRequest(name, params, body)
	request.Header.Set(lager.RequestIdHeader, requestIdHeader)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	response, err := client.Do(request)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	responseBody, err = io.ReadAll(response.Body)
	response.Body.Close()

	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return response.StatusCode, responseBody
}
