package cell_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("TraceId", func() {
	var (
		fileServerStaticDir string
		fileServer          ifrit.Runner
		ifritRuntime        ifrit.Process
		lrp                 *models.DesiredLRP
		testSink            *lagertest.TestSink
		processGuid         string
		rep                 *ginkgomon.Runner
		auctioneer          *ginkgomon.Runner
		router              *ginkgomon.Runner
		routeEmitter        *ginkgomon.Runner
		requestId           string
		loggedRequestId     string
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet supported on windows")
		}
		requestId = "0bc29108-c522-4360-93dd-30ca38cce13d"
		loggedRequestId = strings.ReplaceAll(requestId, "-", "")
		fileServer, fileServerStaticDir = componentMaker.FileServer()
		test_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)
		rep = componentMaker.Rep()
		auctioneer = componentMaker.Auctioneer()
		router = componentMaker.Router()
		routeEmitter = componentMaker.RouteEmitter()
		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "rep", Runner: rep},
			{Name: "auctioneer", Runner: auctioneer},
			{Name: "router", Runner: router},
			{Name: "route-emitter", Runner: routeEmitter},
			{Name: "file-server", Runner: fileServer},
		}))
		processGuid = helpers.GenerateGuid()
		lrp = helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
		err := bbsClient.DesireLRP(lgr, requestId, lrp)
		Expect(err).NotTo(HaveOccurred())
		testSink = lagertest.NewTestSink()
		lgr.RegisterSink(testSink)
	})

	AfterEach(func() {
		helpers.StopProcesses(ifritRuntime)
	})

	It("logs request trace id", func() {
		Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
		Expect(bbsRunner).To(gbytes.Say(`"trace-id":"` + loggedRequestId + `"`))
		Expect(auctioneer).To(gbytes.Say(`"trace-id":"` + loggedRequestId + `"`))
		Expect(rep).To(gbytes.Say(`"trace-id":"` + loggedRequestId + `"`))
		Expect(routeEmitter).To(gbytes.Say(`"trace-id":"` + loggedRequestId + `"`))
		Expect(gardenRunner.Runner).To(gbytes.Say(`"trace-id":"` + loggedRequestId + `"`))
	})
})
