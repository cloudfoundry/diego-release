package simulation_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"code.cloudfoundry.org/clock"

	"code.cloudfoundry.org/workpool"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auction/simulation/simulationrep"
	"code.cloudfoundry.org/auction/simulation/util"
	"code.cloudfoundry.org/auction/simulation/visualization"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

const InProcess = "inprocess"
const HTTP = "http"
const linuxStack = "linux"

const numCells = 100

var numZones = 2

var cells map[string]rep.SimClient

var repResources = rep.Resources{
	MemoryMB:   100.0,
	DiskMB:     100.0,
	Containers: 100,
}

var defaultMaxContainerStartCount int = 0

var defaultDrivers = []string{"my-driver"}

var workers int

var svgReport *visualization.SVGReport
var reports []*visualization.Report
var reportName string
var disableSVGReport bool

var runnerProcess ifrit.Process
var runnerDelegate *auctionRunnerDelegate
var workPool *workpool.WorkPool
var runner auctiontypes.AuctionRunner
var logger lager.Logger

func init() {
	flag.IntVar(&workers, "workers", 500, "number of concurrent communication worker pools")
	flag.BoolVar(&disableSVGReport, "disableSVGReport", false, "disable displaying SVG reports of the simulation runs")
	flag.StringVar(&reportName, "reportName", "report", "report name")
}

func TestAuction(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auction Suite")
}

var _ = BeforeSuite(func() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	startReport()

	logger = lager.NewLogger("sim")
	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

	cells = buildInProcessReps()
})

var _ = BeforeEach(func() {
	var err error
	workPool, err = workpool.NewWorkPool(workers)
	Expect(err).NotTo(HaveOccurred())

	wg := &sync.WaitGroup{}
	wg.Add(len(cells))
	for _, cell := range cells {
		cell := cell
		workPool.Submit(func() {
			cell.Reset()
			wg.Done()
		})
	}
	wg.Wait()

	util.ResetGuids()

	runnerDelegate = NewAuctionRunnerDelegate(cells)
	metricEmitterDelegate := NewAuctionMetricEmitterDelegate()
	runner = auctionrunner.New(
		logger,
		runnerDelegate,
		metricEmitterDelegate,
		clock.NewClock(),
		workPool,
		0.0,
		0.25,
		defaultMaxContainerStartCount,
	)
	runnerProcess = ifrit.Invoke(runner)
})

var _ = AfterEach(func() {
	runnerProcess.Signal(os.Interrupt)
	Eventually(runnerProcess.Wait(), 20).Should(Receive())
	workPool.Stop()
})

var _ = AfterSuite(func() {
	if !disableSVGReport {
		finishReport()
	}
})

func cellGuid(index int) string {
	return fmt.Sprintf("REP-%d", index+1)
}

func zone(index int) string {
	return fmt.Sprintf("Z%d", index%numZones)
}

func buildInProcessReps() map[string]rep.SimClient {
	cells := map[string]rep.SimClient{}

	for i := 0; i < numCells; i++ {
		guid := cellGuid(i)
		cells[guid] = simulationrep.New(guid, i, linuxStack, zone(i), repResources, defaultDrivers)
	}

	return cells
}

func startReport() {
	svgReport = visualization.StartSVGReport("./"+reportName+".svg", 4, 3, numCells)
}

func finishReport() {
	err := svgReport.Done()
	Expect(err).NotTo(HaveOccurred())
	_, err = exec.LookPath("rsvg-convert")
	if err == nil {
		exec.Command("rsvg-convert", "-h", "2000", "--background-color=#fff", "./"+reportName+".svg", "-o", "./"+reportName+".png").Run()
		exec.Command("open", "./"+reportName+".png").Run()
	}

	data, err := json.Marshal(reports)
	Expect(err).NotTo(HaveOccurred())
	os.WriteFile("./"+reportName+".json", data, 0777)
}
