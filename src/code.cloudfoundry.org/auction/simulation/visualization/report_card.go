package visualization

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/GaryBoone/GoStats/stats"
	. "github.com/onsi/gomega"

	svg "github.com/ajstarks/svgo"
)

var border = 5
var instanceSize = 4
var instanceSpacing = 1
var instanceBoxHeight = instanceSize*100 + instanceSpacing*99
var instanceBoxWidth = instanceSize*100 + instanceSpacing*99

var headerHeight = 80

var graphWidth = 200
var graphTextX = 50
var graphBinX = 55
var binHeight = 14
var binSpacing = 2
var maxBinLength = graphWidth - graphBinX

var ReportCardWidth = border*3 + instanceBoxWidth + graphWidth
var ReportCardHeight = border*3 + instanceBoxHeight

type SVGReport struct {
	SVG                *svg.SVG
	f                  *os.File
	waitTimes          []float64
	distributionScores []float64
	width              int
	height             int
}

func StartSVGReport(path string, width, height int, numCells int) *SVGReport {
	instanceBoxHeight = instanceSize*numCells + instanceSpacing*(numCells-1)
	ReportCardHeight = border*3 + instanceBoxHeight

	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	s := svg.New(f)
	s.Start(width*ReportCardWidth, headerHeight+height*ReportCardHeight)
	return &SVGReport{
		f:      f,
		SVG:    s,
		width:  width,
		height: height,
	}
}

func (r *SVGReport) Done() error {
	r.drawResults()
	r.SVG.End()
	return r.f.Close()
}

func (r *SVGReport) drawResults() {
	r.SVG.Text(border, 70, fmt.Sprintf("Distribution Scores: %.2f, Wait Time: %.2fs", stats.StatsSum(r.distributionScores), stats.StatsSum(r.waitTimes)), `text-anchor:start;font-size:20px;font-family:Helvetica Neue`)
}

func (r *SVGReport) DrawReportCard(x, y int, report *Report) {
	r.SVG.Translate(x*ReportCardWidth, headerHeight+y*ReportCardHeight)

	r.drawInstances(report)
	y = r.drawDurationsHistogram(report)
	y = r.drawAttemptsHistogram(report, y+binSpacing*4)
	r.drawText(report, y+binSpacing*4)

	r.waitTimes = append(r.waitTimes, report.AuctionDuration.Seconds())
	r.distributionScores = append(r.distributionScores, report.DistributionScore())

	r.SVG.Gend()
}

func (r *SVGReport) backgroundColorForZone(zone string) string {
	switch zone {
	case "Z0":
		return "fill:#ffdddd"
	case "Z1":
		return "fill:#ddddff"
	default:
		return "fill:#f7f7f7"
	}
}

func (r *SVGReport) drawInstances(report *Report) {
	y := border
	for i := 0; i < len(report.Cells); i++ {
		guid := cellID(i)
		x := border
		bgColor := r.backgroundColorForZone(report.CellStates[cellID(i)].Zone)
		r.SVG.Rect(x, y, instanceBoxWidth, instanceSize, bgColor)
		instances := report.InstancesByRep[guid]
		for _, instance := range instances {
			instanceWidth := instanceSize*int(instance.MemoryMB) + instanceSpacing*int(instance.MemoryMB-1)
			style := instanceStyle(instance.ProcessGuid)
			if report.IsAuctionedInstance(instance) {
				r.SVG.Rect(x, y, instanceWidth, instanceSize, style)
			} else {
				r.SVG.Rect(x+1, y+1, instanceWidth-2, instanceSize-2, style)
			}
			x += instanceWidth + instanceSpacing
		}
		y += instanceSize + instanceSpacing
	}
}

func (r *SVGReport) drawDurationsHistogram(report *Report) int {
	waitTimes := []float64{}
	for _, start := range report.AuctionResults.SuccessfulLRPs {
		waitTimes = append(waitTimes, start.WaitDuration.Seconds())
	}
	sort.Float64s(waitTimes)

	bins := binUp([]float64{0, 0.25, 0.5, 1, 2, 5, 10, 20, 40, 1e9}, waitTimes)
	labels := []string{"<0.25s", "0.25-0.5s", "0.5-1s", "1-2s", "2-5s", "5-10s", "10-20s", "20-40s", ">40s"}

	r.SVG.Translate(border*2+instanceBoxWidth, border)

	yBottom := r.drawHistogram(bins, labels)

	r.SVG.Gend()

	return yBottom + border //'cause of the translate
}

func (r *SVGReport) drawAttemptsHistogram(report *Report, y int) int {
	attempts := []float64{}
	for _, start := range report.AuctionResults.SuccessfulLRPs {
		attempts = append(attempts, float64(start.Attempts))
	}
	for _, start := range report.AuctionResults.FailedLRPs {
		attempts = append(attempts, float64(start.Attempts))
	}
	sort.Float64s(attempts)

	bins := binUp([]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, attempts)
	labels := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}

	r.SVG.Translate(border*2+instanceBoxWidth, y)

	yBottom := r.drawHistogram(bins, labels)

	r.SVG.Gend()

	return yBottom + y
}

func (r *SVGReport) drawText(report *Report, y int) {
	waitStats := report.WaitTimeStats()

	missing := ""
	missingInstances := report.NMissingInstances()
	if missingInstances > 0 {
		missing = fmt.Sprintf("MISSING %d (%.2f%%)", missingInstances, float64(missingInstances)/float64(report.NumAuctions)*100)
	}

	lines := []string{
		fmt.Sprintf("%d over %d Reps %s", report.NumAuctions, report.NReps(), missing),
		fmt.Sprintf("%.2fs (%.2f a/s)", report.AuctionDuration.Seconds(), report.AuctionsPerSecond()),
		fmt.Sprintf("Dist: %.3f => %.3f", report.InitialDistributionScore(), report.DistributionScore()),
	}
	statLines := []string{
		"Wait Times",
		fmt.Sprintf("...%.2fs | %.2f ± %.2f", report.AuctionDuration.Seconds(), waitStats.Mean, waitStats.StdDev),
		fmt.Sprintf("...%.3f - %.3f", waitStats.Min, waitStats.Max),
	}

	r.SVG.Translate(border*2+instanceBoxWidth, y)
	r.SVG.Gstyle("font-family:Helvetica Neue")
	r.SVG.Textlines(8, 8, lines, 16, 18, "#333", "start")
	r.SVG.Textlines(8, 80, statLines, 13, 16, "#333", "start")
	r.SVG.Gend()
	r.SVG.Gend()
}

func (r *SVGReport) drawHistogram(bins []float64, labels []string) int {
	y := 0
	for i, percentage := range bins {
		r.SVG.Rect(graphBinX, y, maxBinLength, binHeight, `fill:#eee`)
		r.SVG.Text(graphTextX, y+binHeight-4, labels[i], `text-anchor:end;font-size:10px;font-family:Helvetica Neue`)
		if percentage > 0 {
			r.SVG.Rect(graphBinX, y, int(percentage*float64(maxBinLength)), binHeight, `fill:#333`)
			r.SVG.Text(graphBinX+binSpacing, y+binHeight-4, fmt.Sprintf("%.1f%%", percentage*100.0), `text-anchor:start;font-size:10px;font-family:Helvetica Neue;fill:#fff`)
		}
		y += binHeight + binSpacing
	}

	return y
}

func binUp(binBoundaries []float64, sortedData []float64) []float64 {
	bins := make([]float64, len(binBoundaries)-1)
	currentBin := 0
	for _, d := range sortedData {
		for binBoundaries[currentBin+1] < d {
			currentBin += 1
		}
		bins[currentBin] += 1
	}

	for i := range bins {
		bins[i] = (bins[i] / float64(len(sortedData)))
	}

	return bins
}

func instanceStyle(processGuid string) string {
	components := strings.Split(processGuid, "-")
	color := processGuid
	if len(components) > 1 {
		color = components[len(components)-1]
	}
	return "fill:" + color + ";" + "stroke:none"
}
