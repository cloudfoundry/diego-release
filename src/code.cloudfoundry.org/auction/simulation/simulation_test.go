package simulation_test

import (
	"fmt"
	"math"
	"sync"
	"time"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/simulation/util"
	"code.cloudfoundry.org/auction/simulation/visualization"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/rep"
	"github.com/GaryBoone/GoStats/stats"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Auction", func() {
	var initialDistributions map[int][]rep.LRP
	var linuxRootFSURL = models.PreloadedRootFS(linuxStack)

	newLRP := func(processGuid string, index int, memoryMB int) rep.LRP {
		lrpKey := models.NewActualLRPKey(processGuid, int32(index), "domain")
		return rep.NewLRP("", lrpKey, rep.NewResource(int32(memoryMB), 1, 10), rep.NewPlacementConstraint(linuxRootFSURL, []string{}, []string{}))
	}

	generateUniqueLRPs := func(numInstances int, index int, memoryMB int) []rep.LRP {
		instances := []rep.LRP{}
		for i := 0; i < numInstances; i++ {
			instances = append(instances, newLRP(util.NewGrayscaleGuid("AAA"), index, memoryMB))
		}
		return instances
	}

	newLRPStartAuction := func(processGuid string, index int, memoryMB int32) auctioneer.LRPStartRequest {
		return auctioneer.NewLRPStartRequest(processGuid, "auction", []int{index}, rep.NewResource(memoryMB, 1, 10), rep.NewPlacementConstraint(linuxRootFSURL, []string{}, []string{}))
	}

	generateUniqueLRPStartAuctions := func(numInstances int, memoryMB int32) []auctioneer.LRPStartRequest {
		instances := []auctioneer.LRPStartRequest{}
		for i := 0; i < numInstances; i++ {
			instances = append(instances, newLRPStartAuction(util.NewGrayscaleGuid("BBB"), i, memoryMB))
		}
		return instances
	}

	generateLRPStartAuctionsWithRandomColor := func(numInstances int, memoryMB int32, colors []string) []auctioneer.LRPStartRequest {
		instances := []auctioneer.LRPStartRequest{}
		for i := 0; i < numInstances; i++ {
			color := colors[util.R.Intn(len(colors))]
			instances = append(instances, newLRPStartAuction(color, i, memoryMB))
		}
		return instances
	}

	generateLRPStartAuctionsForProcessGuid := func(numInstances int, processGuid string, memoryMB int32) []auctioneer.LRPStartRequest {
		instances := []auctioneer.LRPStartRequest{}
		for i := 0; i < numInstances; i++ {
			instances = append(instances, newLRPStartAuction(processGuid, i, memoryMB))
		}
		return instances
	}

	workForInstances := func(lrps []rep.LRP) rep.Work {
		return rep.Work{LRPs: lrps}
	}

	runStartAuction := func(lrpStartAuctions []auctioneer.LRPStartRequest, numCells int) {
		runnerDelegate.SetCellLimit(numCells)
		runner.ScheduleLRPsForAuctions(lrpStartAuctions, "some-trace-id")

		Eventually(runnerDelegate.ResultSize, 2*time.Minute, 100*time.Millisecond).Should(Equal(len(lrpStartAuctions)))
	}

	runAndReportStartAuction := func(lrpStartAuctions []auctioneer.LRPStartRequest, numCells int, i int, j int) *visualization.Report {
		t := time.Now()
		runStartAuction(lrpStartAuctions, numCells)

		Eventually(runnerDelegate.ResultSize, time.Minute, 100*time.Millisecond).Should(Equal(len(lrpStartAuctions)))
		duration := time.Since(t)

		cells, _ := runnerDelegate.FetchCellReps(logger, "some-trace-id")
		report := visualization.NewReport(len(lrpStartAuctions), cells, runnerDelegate.Results(), duration)

		visualization.PrintReport(report)
		svgReport.DrawReportCard(i, j, report)
		reports = append(reports, report)
		fmt.Println("Done...")
		return report
	}

	getFinalDistributions := func() map[string]float64 {
		finalDistributions := make(map[string]float64)
		for _, lrpAuction := range runnerDelegate.Results().SuccessfulLRPs {
			finalDistributions[lrpAuction.Winner] += 1.0
		}

		return finalDistributions
	}

	getResultVector := func() []float64 {
		finalDistributions := getFinalDistributions()

		distroVector := make([]float64, 0)
		for _, v := range finalDistributions {
			distroVector = append(distroVector, v)
		}

		return distroVector
	}

	assertSampleDistributionTolerances := func(sample []float64, tolerance int) {
		mean := stats.StatsMean(sample)

		for j := range sample {
			deviance := math.Abs(sample[j] - mean)
			Expect(deviance).To(BeNumerically("<=", tolerance))
		}
	}

	// tolerance is tolerated the number of LRP deviance from the mean
	assertDistributionTolerances := func(tolerance int) {
		distroVector := getResultVector()
		assertSampleDistributionTolerances(distroVector, tolerance)
	}

	getZones := func(numCells int) map[int][]string {
		zones := map[int][]string{}

		for zoneIdx := 0; zoneIdx < numZones; zoneIdx++ {
			zones[zoneIdx] = []string{}

			for i := zoneIdx; i < numCells; i += numZones {
				zones[zoneIdx] = append(zones[zoneIdx], cellGuid(i))
			}
		}

		return zones
	}

	assertNonDecreasingMonotonicSample := func(cells []string, finalDistributions map[string]float64) {
		for i := 0; i+1 < len(cells); i++ {
			cellID := cells[i]
			nextCellID := cells[i+1]
			Expect(finalDistributions[cellID]).To(BeNumerically(">=", finalDistributions[nextCellID]))
		}
	}

	getNumLRPsPerZone := func(zone []string, finalDistributions map[string]float64) (numLRPs float64) {
		for _, cell := range zone {
			numLRPs += finalDistributions[cell]
		}

		return numLRPs
	}

	assertLRPBalanceBetweenZones := func(tolerance int, zones map[int][]string, finalDistributions map[string]float64) {
		lrpsPerZones := []float64{}

		for _, zone := range zones {
			numLRPs := getNumLRPsPerZone(zone, finalDistributions)
			lrpsPerZones = append(lrpsPerZones, numLRPs)
		}

		assertSampleDistributionTolerances(lrpsPerZones, tolerance)
	}

	BeforeEach(func() {
		util.ResetGuids()
		initialDistributions = map[int][]rep.LRP{}
	})

	JustBeforeEach(func() {
		wg := &sync.WaitGroup{}
		wg.Add(len(initialDistributions))
		for index, instances := range initialDistributions {
			guid := cellGuid(index)
			instances := instances
			workPool.Submit(func() {
				cells[guid].Perform(logger, workForInstances(instances))
				wg.Done()
			})
		}
		wg.Wait()
	})

	Describe("Experiments", func() {
		Context("Small Cold LRPStarts", func() {
			napps := []int{8, 40, 200, 800}
			ncells := []int{4, 10, 20, 40}
			for i := range ncells {
				i := i
				It("should distribute evenly", func() {
					instances := generateUniqueLRPStartAuctions(napps[i], 1)

					runAndReportStartAuction(instances, ncells[i], i, 0)

					assertDistributionTolerances(1)
				})
			}
		})

		Context("Bin Pack First Fit", func() {
			napps := []int{8, 40, 200, 800}
			ncells := []int{4, 10, 20, 40}
			binPackWeights := []float64{0.25, 0.5, 0.75}

			for _, w := range binPackWeights {
				weight := w
				Context(fmt.Sprintf("with weight %f", w), func() {
					JustBeforeEach(func() {
						metricEmitterDelegate := NewAuctionMetricEmitterDelegate()
						runner = auctionrunner.New(
							logger,
							runnerDelegate,
							metricEmitterDelegate,
							clock.NewClock(),
							workPool,
							weight,
							0.5,
							defaultMaxContainerStartCount,
						)
						runnerProcess = ifrit.Invoke(runner)
					})

					for i := range ncells {
						i := i
						It("favors cells with lower index", func() {
							instances := generateUniqueLRPStartAuctions(napps[i], 1)
							runAndReportStartAuction(instances, ncells[i], i, 4)

							finalDistributions := getFinalDistributions()

							zones := getZones(ncells[i])
							for _, zone := range zones {
								assertNonDecreasingMonotonicSample(zone, finalDistributions)
							}

							tolerance := 1
							assertLRPBalanceBetweenZones(tolerance, zones, finalDistributions)
						})

						It("should distribute evenly", func() {
							instances := generateLRPStartAuctionsForProcessGuid(napps[i], "yellow", 1)
							runAndReportStartAuction(instances, ncells[i], i+1, 2)

							assertDistributionTolerances(1)
						})
					}
				})
			}
		})

		Context("Large Cold LRPStarts", func() {
			ncells := []int{25, 4 * 25}
			n1apps := []int{1800, 4 * 1800}
			n2apps := []int{200, 4 * 200}
			n4apps := []int{50, 4 * 50}
			for i := range ncells {
				i := i
				Context("with single-instance and multi-instance apps", func() {
					It("should distribute evenly", func() {
						instances := []auctioneer.LRPStartRequest{}
						colors := []string{"purple", "red", "orange", "teal", "gray", "blue", "pink", "green", "lime", "cyan", "lightseagreen", "brown"}

						instances = append(instances, generateUniqueLRPStartAuctions(n1apps[i]/2, 1)...)
						instances = append(instances, generateLRPStartAuctionsWithRandomColor(n1apps[i]/2, 1, colors[:4])...)
						instances = append(instances, generateUniqueLRPStartAuctions(n2apps[i]/2, 2)...)
						instances = append(instances, generateLRPStartAuctionsWithRandomColor(n2apps[i]/2, 2, colors[4:8])...)
						instances = append(instances, generateUniqueLRPStartAuctions(n4apps[i]/2, 4)...)
						instances = append(instances, generateLRPStartAuctionsWithRandomColor(n4apps[i]/2, 4, colors[8:12])...)

						permutedInstances := make([]auctioneer.LRPStartRequest, len(instances))
						for i, index := range util.R.Perm(len(instances)) {
							permutedInstances[i] = instances[index]
						}

						runAndReportStartAuction(permutedInstances, ncells[i], i+1, 3)

						assertDistributionTolerances(10)
					})
				})
			}
		})

		Context("Imbalanced scenario (e.g. a deploy)", func() {
			ncells := []int{100, 100}
			nempty := []int{5, 1}
			napps := []int{500, 100}

			for i := range ncells {
				i := i
				Context("scenario", func() {
					BeforeEach(func() {
						for j := 0; j < ncells[i]-nempty[i]; j++ {
							initialDistributions[j] = generateUniqueLRPs(50, 0, 1)
						}
					})

					It("should distribute evenly without overloading empty cells", func() {
						instances := generateUniqueLRPStartAuctions(napps[i], 1)

						runAndReportStartAuction(instances, ncells[i], i+2, 1)

						assertDistributionTolerances(1)
					})
				})
			}
		})

		Context("AZ distribution", func() {
			ncells := 3
			napps := 40
			initialAppsOnZone0 := 50

			BeforeEach(func() {
				initialDistributions[1] = generateUniqueLRPs(initialAppsOnZone0, 0, 1)
			})

			It("should distribute across the zones", func() {
				instances := generateLRPStartAuctionsForProcessGuid(napps, "red", 1)

				report := runAndReportStartAuction(instances, ncells, 0, 2)

				By("populating the lone cell in Z1 even though it is heavily-loaded ")
				numOnZone0 := 0
				numOnZone0 += len(report.InstancesByRep[cellGuid(0)])
				numOnZone0 += len(report.InstancesByRep[cellGuid(2)])

				numOnZone1 := len(report.InstancesByRep[cellGuid(1)]) - initialAppsOnZone0

				Expect(numOnZone0).To(Equal(numOnZone1))
			})
		})

		Context("The Watters demo", func() {
			ncells := []int{10, 30, 100}
			napps := []int{80, 200, 400}

			for i := range ncells {
				i := i

				Context("scenario", func() {
					BeforeEach(func() {
						for j := 0; j < ncells[i]; j++ {
							initialDistributions[j] = generateUniqueLRPs(util.RandomIntIn(78, 80), 0, 1)
						}
					})

					It("should distribute evenly", func() {
						instances := generateLRPStartAuctionsForProcessGuid(napps[i], "red", 1)

						runAndReportStartAuction(instances, ncells[i], i+1, 2)

						assertDistributionTolerances(1)
					})
				})
			}
		})

		Context("Packing optimally when memory is low", func() {
			nCells := 1

			It("should place boulders in before pebbles, but prevent boulders from saturating available capacity", func() {
				instances := []auctioneer.LRPStartRequest{}
				for i := 0; i < 80; i++ {
					instances = append(instances, generateUniqueLRPStartAuctions(1, 1)...)
				}
				instances = append(instances, generateLRPStartAuctionsForProcessGuid(2, "red", 50)...)

				runAndReportStartAuction(instances, nCells, 0, 3)
				results := runnerDelegate.Results()

				winners := []string{}
				losers := []string{}

				for _, result := range results.SuccessfulLRPs {
					winners = append(winners, fmt.Sprintf("%s-%d", result.ProcessGuid, result.Index))
				}
				for _, result := range results.FailedLRPs {
					losers = append(losers, fmt.Sprintf("%s-%d", result.ProcessGuid, result.Index))
				}

				Expect(winners).To(HaveLen(51))
				Expect(losers).To(HaveLen(31))

				Expect(winners).To(ContainElement("red-0"))
				Expect(losers).To(ContainElement("red-1"))
			})
		})
	})
})
