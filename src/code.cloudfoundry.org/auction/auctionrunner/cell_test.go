package auctionrunner_test

import (
	"errors"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/repfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cell", func() {
	var (
		client          *repfakes.FakeSimClient
		emptyCell, cell *auctionrunner.Cell
	)

	BeforeEach(func() {
		client = &repfakes.FakeSimClient{}
		emptyState := BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
		emptyCell = auctionrunner.NewCell(logger, "empty-cell", client, emptyState)

		state := BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 10, linuxOnlyRootFSProviders, []rep.LRP{
			*BuildLRP("pg-1", "domain", 0, linuxRootFSURL, 10, 20, 10, []string{}),
			*BuildLRP("pg-1", "domain", 1, linuxRootFSURL, 10, 20, 10, []string{}),
			*BuildLRP("pg-2", "domain", 0, linuxRootFSURL, 10, 20, 10, []string{}),
			*BuildLRP("pg-3", "domain", 0, linuxRootFSURL, 10, 20, 10, []string{}),
			*BuildLRP("pg-4", "domain", 0, linuxRootFSURL, 10, 20, 10, []string{}),
		},
			[]string{},
			[]string{},
			[]string{},
			0,
		)
		cell = auctionrunner.NewCell(logger, "the-cell", client, state)
	})

	Describe("ScoreForLRP", func() {
		It("factors in memory usage", func() {
			bigInstance := BuildLRP("pg-big", "domain", 0, linuxRootFSURL, 20, 10, 10, []string{})
			smallInstance := BuildLRP("pg-small", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})

			By("factoring in the amount of memory taken up by the instance")
			bigScore, err := emptyCell.ScoreForLRP(bigInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForLRP(smallInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		Context("when the cell has proxies enabled", func() {
			var (
				proxiedCellMemory   int32
				lrpDesiredMemory    int32
				proxyMemoryOverhead int
				proxiedCell         *auctionrunner.Cell
				lrp                 *rep.LRP
			)

			JustBeforeEach(func() {
				proxiedCellState := BuildCellState("cellID", 0, "the-zone", proxiedCellMemory, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, proxyMemoryOverhead)
				proxiedCell = auctionrunner.NewCell(logger, "proxied-cell", client, proxiedCellState)
				lrp = BuildLRP("pg-big", "domain", 0, linuxRootFSURL, lrpDesiredMemory, 10, 10, []string{})
			})

			Context("when there is enough memory for the cell and the proxy", func() {
				BeforeEach(func() {
					proxiedCellMemory = 260
					lrpDesiredMemory = 256
					proxyMemoryOverhead = 4
				})

				It("succeeds placing the lrp", func() {
					score, err := proxiedCell.ScoreForLRP(lrp, 0.0, 0.0)
					Expect(err).NotTo(HaveOccurred())
					Expect(score).To(BeNumerically(">", 0))
				})
			})

			Context("when there is not enough memory for the cell and the proxy", func() {
				BeforeEach(func() {
					proxiedCellMemory = 259
					lrpDesiredMemory = 256
					proxyMemoryOverhead = 4
				})

				It("errors with memory placement error", func() {
					score, err := proxiedCell.ScoreForLRP(lrp, 0.0, 0.0)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("insufficient resources: memory"))
					Expect(score).To(BeZero())
				})
			})
		})

		It("factors in disk usage", func() {
			bigInstance := BuildLRP("pg-big", "domain", 0, linuxRootFSURL, 10, 20, 10, []string{})
			smallInstance := BuildLRP("pg-small", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})

			By("factoring in the amount of memory taken up by the instance")
			bigScore, err := emptyCell.ScoreForLRP(bigInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForLRP(smallInstance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in container usage", func() {
			instance := BuildLRP("pg-big", "domain", 0, linuxRootFSURL, 20, 20, 10, []string{})

			bigState := BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
			bigCell := auctionrunner.NewCell(logger, "big-cell", client, bigState)

			smallState := BuildCellState("cellID", 0, "the-zone", 100, 200, 20, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
			smallCell := auctionrunner.NewCell(logger, "small-cell", client, smallState)

			bigScore, err := bigCell.ScoreForLRP(instance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := smallCell.ScoreForLRP(instance, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(bigScore).To(BeNumerically("<", smallScore), "prefer Cells with more resources")
		})

		Context("Weighted Bin Pack First Fit algorithm", func() {
			var instance *rep.LRP
			var cellStateZero, cellStateOne rep.CellState
			var cellZero, cellOne *auctionrunner.Cell

			BeforeEach(func() {
				instance = BuildLRP("pg-0", "domain", 0, linuxRootFSURL, 20, 20, 10, []string{})

				cellZeroLRPs := []rep.LRP{*BuildLRP("pg-1", "domain", 0, linuxRootFSURL, 20, 20, 10, []string{})}
				cellStateZero = BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, cellZeroLRPs, []string{}, []string{}, []string{}, 0)
				cellZero = auctionrunner.NewCell(logger, "cell-0", client, cellStateZero)

				cellOneLRPs := []rep.LRP{*BuildLRP("pg-2", "domain", 0, linuxRootFSURL, 20, 20, 10, []string{})}
				cellStateOne = BuildCellState("cellID", 1, "other-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, cellOneLRPs, []string{}, []string{}, []string{}, 0)
				cellOne = auctionrunner.NewCell(logger, "cell-1", client, cellStateOne)
			})

			It("factors in Bin Pack First Fit algorithm when a weight is provided", func() {
				binPackFirstFitWeight := 0.2

				cellZeroScore, err := cellZero.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())
				cellOneScore, err := cellOne.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(cellZeroScore).To(BeNumerically("<", cellOneScore), "prefer Cells that have a lower index number")
			})

			It("privileges spreading LRPs across cells over Bin Pack First Fit algorithm", func() {
				instance = BuildLRP("pg-1", "domain", 1, linuxRootFSURL, 20, 20, 10, []string{})
				binPackFirstFitWeight := 0.2

				cellZeroScore, err := cellZero.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())
				cellOneScore, err := cellOne.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(cellZeroScore).To(BeNumerically(">", cellOneScore), "prefer Cells that do not have an instance of self already running")
			})

			It("ignores Bin Pack First Fit algorithm when a weight is not provided", func() {
				binPackFirstFitWeight := 0.0

				cellZeroScore, err := cellZero.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())
				cellOneScore, err := cellOne.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(cellZeroScore).To(BeNumerically("==", cellOneScore), "ignore Bin Pack First Fit algorithm")
			})

			It("prefers normalised cell indices", func() {
				binPackFirstFitWeight := 1.0

				cellZero.Index = 0
				cellZeroScore, err := cellZero.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())

				cellOne.Index = 0
				cellOneScore, err := cellOne.ScoreForLRP(instance, 0.0, binPackFirstFitWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(cellZeroScore).To(BeNumerically("==", cellOneScore), "has a separate normalised cell ordering for each zone")
			})
		})

		Context("Starting Containers", func() {
			var instance *rep.LRP
			var busyState, boredState rep.CellState
			var busyCell, boredCell *auctionrunner.Cell

			BeforeEach(func() {
				instance = BuildLRP("pg-busy", "domain", 0, linuxRootFSURL, 20, 20, 10, []string{})

				busyState = BuildCellState(
					"cellID",
					0,
					"the-zone",
					100,
					200,
					50,
					false,
					10,
					linuxOnlyRootFSProviders,
					[]rep.LRP{{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "not-HA"}}},
					[]string{},
					[]string{},
					[]string{},
					0,
				)
				busyCell = auctionrunner.NewCell(logger, "busy-cell", client, busyState)

				boredState = BuildCellState(
					"cellID",
					0,
					"the-zone",
					100,
					200,
					50,
					false,
					0,
					linuxOnlyRootFSProviders,
					[]rep.LRP{{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "HA"}}},
					[]string{},
					[]string{},
					[]string{},
					0,
				)
				boredCell = auctionrunner.NewCell(logger, "bored-cell", client, boredState)
			})

			It("factors in starting containers when a weight is provided", func() {
				startingContainerWeight := 0.25

				busyScore, err := busyCell.ScoreForLRP(instance, startingContainerWeight, 0.0)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForLRP(instance, startingContainerWeight, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically(">", boredScore), "prefer Cells that have less starting containers")

				smallerWeightState := BuildCellState(
					"cellID",
					0,
					"the-zone",
					100,
					200,
					50,
					false,
					10,
					linuxOnlyRootFSProviders,
					nil,
					[]string{},
					[]string{},
					[]string{},
					0,
				)
				smallerWeightCell := auctionrunner.NewCell(logger, "busy-cell", client, smallerWeightState)
				smallerWeightScore, err := smallerWeightCell.ScoreForLRP(instance, startingContainerWeight-0.1, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically(">", smallerWeightScore), "the number of starting containers is weighted")
			})

			It("privileges spreading LRPs across cells over starting containers", func() {
				instance = BuildLRP("HA", "domain", 1, linuxRootFSURL, 20, 20, 10, []string{})
				startingContainerWeight := 0.25

				busyScore, err := busyCell.ScoreForLRP(instance, startingContainerWeight, 0.0)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForLRP(instance, startingContainerWeight, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically("<", boredScore), "prefer Cells that do not have an instance of self already running")
			})

			It("ignores starting containers when a weight is not provided", func() {
				startingContainerWeight := 0.0

				busyScore, err := busyCell.ScoreForLRP(instance, startingContainerWeight, 0.0)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForLRP(instance, startingContainerWeight, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically("==", boredScore), "ignore how many starting Containers a cell has")
			})
		})

		It("factors in process-guids that are already present", func() {
			instanceWithTwoMatches := BuildLRP("pg-1", "domain", 2, linuxRootFSURL, 10, 10, 10, []string{})
			instanceWithOneMatch := BuildLRP("pg-2", "domain", 1, linuxRootFSURL, 10, 10, 10, []string{})
			instanceWithNoMatches := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})

			twoMatchesScore, err := cell.ScoreForLRP(instanceWithTwoMatches, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			oneMatchesScore, err := cell.ScoreForLRP(instanceWithOneMatch, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())
			noMatchesScore, err := cell.ScoreForLRP(instanceWithNoMatches, 0.0, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(noMatchesScore).To(BeNumerically("<", oneMatchesScore))
			Expect(oneMatchesScore).To(BeNumerically("<", twoMatchesScore))
		})

		Context("when the LRP does not fit", func() {
			Context("because of memory constraints", func() {
				It("should error", func() {
					massiveMemoryInstance := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10000, 10, 1024, []string{})
					score, err := cell.ScoreForLRP(massiveMemoryInstance, 0.0, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: memory"))
				})
			})

			Context("because of disk constraints", func() {
				It("should error", func() {
					massiveDiskInstance := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10000, 1024, []string{})
					score, err := cell.ScoreForLRP(massiveDiskInstance, 0.0, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: disk"))
				})
			})

			Context("because of container constraints", func() {
				It("should error", func() {
					instance := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})
					zeroState := BuildCellState("cellID", 0, "the-zone", 100, 100, 0, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
					zeroCell := auctionrunner.NewCell(logger, "zero-cell", client, zeroState)
					score, err := zeroCell.ScoreForLRP(instance, 0.0, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: containers"))
				})
			})
		})
	})

	Describe("ScoreForTask", func() {
		It("factors in number of tasks currently running", func() {
			bigTask := BuildTask("tg-big", "domain", linuxRootFSURL, 20, 10, 10, []string{}, []string{})
			smallTask := BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})

			By("factoring in the amount of memory taken up by the task")
			bigScore, err := emptyCell.ScoreForTask(bigTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in memory usage", func() {
			bigTask := BuildTask("tg-big", "domain", linuxRootFSURL, 20, 10, 10, []string{}, []string{})
			smallTask := BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})

			By("factoring in the amount of memory taken up by the task")
			bigScore, err := emptyCell.ScoreForTask(bigTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in disk usage", func() {
			bigTask := BuildTask("tg-big", "domain", linuxRootFSURL, 10, 20, 10, []string{}, []string{})
			smallTask := BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})

			By("factoring in the amount of memory taken up by the task")
			bigScore, err := emptyCell.ScoreForTask(bigTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in container usage", func() {
			task := BuildTask("tg-big", "domain", linuxRootFSURL, 20, 20, 10, []string{}, []string{})

			bigState := BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
			bigCell := auctionrunner.NewCell(logger, "big-cell", client, bigState)

			smallState := BuildCellState("cellID", 0, "the-zone", 100, 200, 20, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
			smallCell := auctionrunner.NewCell(logger, "small-cell", client, smallState)

			bigScore, err := bigCell.ScoreForTask(task, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := smallCell.ScoreForTask(task, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(bigScore).To(BeNumerically("<", smallScore), "prefer Cells with more resources")
		})

		Context("Starting Containers", func() {
			var task *rep.Task
			var busyState, boredState rep.CellState
			var busyCell, boredCell *auctionrunner.Cell

			BeforeEach(func() {
				task = BuildTask("tg-big", "domain", linuxRootFSURL, 20, 20, 20, []string{}, []string{})

				busyState = BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 10, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
				busyCell = auctionrunner.NewCell(logger, "busy-cell", client, busyState)

				boredState = BuildCellState("cellID", 0, "the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
				boredCell = auctionrunner.NewCell(logger, "bored-cell", client, boredState)
			})

			It("factors in starting containers when a weight is provided", func() {
				startingContainerWeight := 0.25
				busyScore, err := busyCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				Expect(busyScore).To(BeNumerically(">", boredScore), "prefer Cells that have less starting containers")
			})

			It("ignores starting containers when a weight is not provided", func() {
				startingContainerWeight := 0.0
				busyScore, err := busyCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				Expect(busyScore).To(BeNumerically("==", boredScore), "ignore how many starting Containers a cell has")
			})
		})

		Context("when the task does not fit", func() {
			Context("because of memory constraints", func() {
				It("should error", func() {
					massiveMemoryTask := BuildTask("pg-new", "domain", linuxRootFSURL, 10000, 10, 1024, []string{}, []string{})
					score, err := cell.ScoreForTask(massiveMemoryTask, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: memory"))
				})
			})

			Context("because of disk constraints", func() {
				It("should error", func() {
					massiveDiskTask := BuildTask("pg-new", "domain", linuxRootFSURL, 10, 10000, 1024, []string{}, []string{})
					score, err := cell.ScoreForTask(massiveDiskTask, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: disk"))
				})
			})

			Context("because of container constraints", func() {
				It("should error", func() {
					task := BuildTask("pg-new", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})
					zeroState := BuildCellState("cellID", 0, "the-zone", 100, 100, 0, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0)
					zeroCell := auctionrunner.NewCell(logger, "zero-cell", client, zeroState)
					score, err := zeroCell.ScoreForTask(task, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: containers"))
				})
			})
		})
	})

	Describe("ReserveLRP", func() {
		Context("when there is room for the LRP", func() {
			It("should register its resources usage and keep it in mind when handling future requests", func() {
				instance := BuildLRP("pg-test", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})
				instanceToAdd := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})

				initialScore, err := cell.ScoreForLRP(instance, 0.0, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(cell.ReserveLRP(instanceToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForLRP(instance, 0.0, 0.0)
				Expect(err).NotTo(HaveOccurred())
				Expect(initialScore).To(BeNumerically("<", subsequentScore), "the score should have gotten worse")
			})

			It("should register the LRP and keep it in mind when handling future requests", func() {
				instance := BuildLRP("pg-test", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})
				instanceWithMatchingProcessGuid := BuildLRP("pg-new", "domain", 1, linuxRootFSURL, 10, 10, 10, []string{})
				instanceToAdd := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, 10, []string{})

				initialScore, err := cell.ScoreForLRP(instance, 0.0, 0.0)
				Expect(err).NotTo(HaveOccurred())

				initialScoreForInstanceWithMatchingProcessGuid, err := cell.ScoreForLRP(instanceWithMatchingProcessGuid, 0.0, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(initialScore).To(BeNumerically("==", initialScoreForInstanceWithMatchingProcessGuid))

				Expect(cell.ReserveLRP(instanceToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForLRP(instance, 0.0, 0.0)
				Expect(err).NotTo(HaveOccurred())

				subsequentScoreForInstanceWithMatchingProcessGuid, err := cell.ScoreForLRP(instanceWithMatchingProcessGuid, 0.0, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(initialScore).To(BeNumerically("<", subsequentScore), "the score should have gotten worse")
				Expect(initialScoreForInstanceWithMatchingProcessGuid).To(BeNumerically("<", subsequentScoreForInstanceWithMatchingProcessGuid), "the score should have gotten worse")

				Expect(subsequentScore).To(BeNumerically("<", subsequentScoreForInstanceWithMatchingProcessGuid), "the score should be substantially worse for the instance with the matching process guid")
			})
		})

		Context("when there is no room for the LRP", func() {
			It("should error", func() {
				instance := BuildLRP("pg-test", "domain", 0, linuxRootFSURL, 10000, 10, 10, []string{})
				err := cell.ReserveLRP(instance)
				Expect(err).To(MatchError("insufficient resources: memory"))
			})
		})
	})

	Describe("ReserveTask", func() {
		Context("when there is room for the task", func() {
			It("should register its resources usage and keep it in mind when handling future requests", func() {
				task := BuildTask("tg-test", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})
				taskToAdd := BuildTask("tg-new", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})

				initialScore, err := cell.ScoreForTask(task, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(cell.ReserveTask(taskToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForTask(task, 0.0)
				Expect(err).NotTo(HaveOccurred())
				Expect(initialScore).To(BeNumerically("<", subsequentScore), "the score should have gotten worse")
			})

			It("should register the Task and keep it in mind when handling future requests", func() {
				task := BuildTask("tg-test", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})
				taskToAdd := BuildTask("tg-new", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{})

				initialScore, err := cell.ScoreForTask(task, 0.25)
				Expect(err).NotTo(HaveOccurred())

				initialScoreForTaskToAdd, err := cell.ScoreForTask(taskToAdd, 0.25)
				Expect(err).NotTo(HaveOccurred())

				Expect(initialScore).To(BeNumerically("==", initialScoreForTaskToAdd))

				Expect(cell.ReserveTask(taskToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForTask(task, 0.25)
				Expect(err).NotTo(HaveOccurred())

				Expect(subsequentScore).To(BeNumerically(">", initialScore+auctionrunner.LocalityOffset), "the score should have gotten worse by at least 1")
			})
		})

		Context("when there is no room for the Task", func() {
			It("should error", func() {
				task := BuildTask("tg-test", "domain", linuxRootFSURL, 10000, 10, 10, []string{}, []string{})
				err := cell.ReserveTask(task)
				Expect(err).To(MatchError("insufficient resources: memory"))
			})
		})
	})

	Describe("Commit", func() {
		Context("with nothing to commit", func() {
			It("does nothing and returns empty", func() {
				failedWork := cell.Commit()
				Expect(failedWork).To(BeZero())
				Expect(client.PerformCallCount()).To(Equal(0))
			})
		})

		Context("with work to commit", func() {
			var lrp rep.LRP

			BeforeEach(func() {
				lrp = *BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 20, 10, 10, []string{})
				Expect(cell.ReserveLRP(&lrp)).To(Succeed())
			})

			It("asks the client to perform", func() {
				cell.Commit()
				Expect(client.PerformCallCount()).To(Equal(1))
				_, work := client.PerformArgsForCall(0)
				Expect(work).To(Equal(rep.Work{LRPs: []rep.LRP{lrp}, CellID: cell.Guid}))
			})

			Context("when the client returns some failed work", func() {
				It("forwards the failed work", func() {
					failedWork := rep.Work{
						LRPs: []rep.LRP{lrp},
					}
					client.PerformReturns(failedWork, nil)
					Expect(cell.Commit()).To(Equal(failedWork))
				})
			})

			Context("when the client returns an error", func() {
				It("does not return any failed work", func() {
					client.PerformReturns(rep.Work{}, errors.New("boom"))
					Expect(cell.Commit()).To(BeZero())
				})
			})
		})
	})
})
