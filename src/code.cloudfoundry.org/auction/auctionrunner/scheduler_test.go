package auctionrunner_test

import (
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/workpool"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/repfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var defaultStartingContainerCountMaximum int = 0

var _ = Describe("Scheduler", func() {
	var clients map[string]*repfakes.FakeSimClient
	var zones map[string]auctionrunner.Zone
	var clock *fakeclock.FakeClock
	var workPool *workpool.WorkPool
	var results auctiontypes.AuctionResults
	var logger *lagertest.TestLogger
	var scheduler *auctionrunner.Scheduler

	BeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())

		var err error
		workPool, err = workpool.NewWorkPool(5)
		Expect(err).NotTo(HaveOccurred())

		clients = map[string]*repfakes.FakeSimClient{}
		zones = map[string]auctionrunner.Zone{}

		logger = lagertest.NewTestLogger("fakelogger")

		scheduler = auctionrunner.NewScheduler(workPool, map[string]auctionrunner.Zone{}, clock, logger, 0.0, 0.0, 0)
	})

	AfterEach(func() {
		workPool.Stop()
	})

	Context("when there are no cells", func() {
		It("immediately returns everything as having failed, incrementing the attempt number", func() {
			startAuction := BuildLRPAuction("pg-7", "domain", 0, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})

			taskAuction := BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 0, 0, 0, []string{}, []string{}), clock.Now())

			auctionRequest := auctiontypes.AuctionRequest{
				LRPs:  []auctiontypes.LRPAuction{startAuction},
				Tasks: []auctiontypes.TaskAuction{taskAuction},
			}

			By("no auctions are marked successful")
			results := scheduler.Schedule(auctionRequest)
			Expect(results.SuccessfulLRPs).To(BeEmpty())
			Expect(results.SuccessfulTasks).To(BeEmpty())

			By("all lrp starts are marked failed, and their attempts are incremented")
			Expect(results.FailedLRPs).To(HaveLen(1))
			failedLRPStart := results.FailedLRPs[0]
			Expect(failedLRPStart.Identifier()).To(Equal(startAuction.Identifier()))
			Expect(failedLRPStart.Attempts).To(Equal(startAuction.Attempts + 1))
			Expect(failedLRPStart.PlacementError).To(Equal(auctiontypes.ErrorCellCommunication.Error()))

			By("all tasks are marked failed, and their attempts are incremented")
			Expect(results.FailedTasks).To(HaveLen(1))
			failedTask := results.FailedTasks[0]
			Expect(failedTask.Identifier()).To(Equal(taskAuction.Identifier()))
			Expect(failedTask.Attempts).To(Equal(taskAuction.Attempts + 1))
			Expect(failedLRPStart.PlacementError).To(Equal(auctiontypes.ErrorCellCommunication.Error()))
		})
	})

	Context("when cells have inflight container creations", func() {
		BeforeEach(func() {
			inflightStartsPerCell := 1

			clients["A-cell"] = &repfakes.FakeSimClient{}
			zones["A-zone"] = auctionrunner.Zone{
				auctionrunner.NewCell(
					logger,
					"A-cell",
					clients["A-cell"],
					BuildCellState("cellID", 0, "A-zone", 100, 100, 100, false, inflightStartsPerCell, linuxOnlyRootFSProviders, []rep.LRP{
						*BuildLRP("pg-1", "domain", 0, "", 10, 10, 10, []string{}),
						*BuildLRP("pg-2", "domain", 0, "", 10, 10, 10, []string{}),
					}, []string{}, []string{}, []string{}, 0),
				),
			}

			clients["B-cell"] = &repfakes.FakeSimClient{}
			zones["B-zone"] = auctionrunner.Zone{
				auctionrunner.NewCell(
					logger,
					"B-cell",
					clients["B-cell"],
					BuildCellState("cellID", 0, "B-zone", 100, 100, 100, false, inflightStartsPerCell, linuxOnlyRootFSProviders, []rep.LRP{
						*BuildLRP("pg-3", "domain", 0, "", 10, 10, 10, []string{}),
					}, []string{}, []string{}, []string{}, 0),
				),
			}
		})

		Context("and the current auction would exceed the maximum inflight container creations", func() {
			var auctionRequest auctiontypes.AuctionRequest
			var startingContainerCountMaximum int

			BeforeEach(func() {
				startingContainerCountMaximum = 5
				pg70 := BuildLRPAuction("pg-7", "domain", 0, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
				pg71 := BuildLRPAuction("pg-7", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
				taskAuction1 := BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
				taskAuction2 := BuildTaskAuction(BuildTask("tg-2", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())

				scheduler = auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, startingContainerCountMaximum)
				startLRPAuctions := []auctiontypes.LRPAuction{pg70, pg71}
				startTaskAuctions := []auctiontypes.TaskAuction{taskAuction1, taskAuction2}
				auctionRequest = auctiontypes.AuctionRequest{LRPs: startLRPAuctions, Tasks: startTaskAuctions}
			})

			It("only starts the maximum number of containers", func() {
				// 2 LRPs, 2 Tasks, 2 Cell inflights
				// Order of auctioning: pg-7[0], tg-1, tg-2, pg-7[1]
				// Tasks succeed
				// First LRP succeeds second fails.
				results = scheduler.Schedule(auctionRequest)
				Expect(results.SuccessfulLRPs).To(HaveLen(1))
				Expect(results.FailedLRPs).To(HaveLen(1))
				Expect(results.SuccessfulTasks).To(HaveLen(2))
				Expect(results.FailedTasks).To(BeEmpty())
			})
		})
	})

	Describe("handling start auctions", func() {

		var startAuction auctiontypes.LRPAuction

		BeforeEach(func() {
			clients["A-cell"] = &repfakes.FakeSimClient{}
			zones["A-zone"] = auctionrunner.Zone{
				auctionrunner.NewCell(
					logger,
					"A-cell",
					clients["A-cell"],
					BuildCellState("cellID", 0, "A-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
						*BuildLRP("pg-1", "domain", 0, "", 10, 10, 10, []string{}),
						*BuildLRP("pg-2", "domain", 0, "", 10, 10, 10, []string{}),
					}, []string{}, []string{}, []string{}, 0),
				),
			}

			clients["B-cell"] = &repfakes.FakeSimClient{}
			zones["B-zone"] = auctionrunner.Zone{
				auctionrunner.NewCell(
					logger,
					"B-cell",
					clients["B-cell"],
					BuildCellState("cellID", 0, "B-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
						*BuildLRP("pg-3", "domain", 0, "", 10, 10, 10, []string{}),
					}, []string{}, []string{}, []string{}, 0),
				),
			}
		})

		Context("when only one of many zones supports a specific RootFS", func() {
			BeforeEach(func() {
				clients["C-cell"] = &repfakes.FakeSimClient{}
				zones["C-zone"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"C-cell",
						clients["C-cell"],
						BuildCellState("cellID", 0, "C-zone", 100, 100, 100, false, 0, windowsOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-win-1", "domain", 0, "", 10, 10, 10, []string{}),
						}, []string{}, []string{}, []string{}, 0),
					),
				}
			})

			Context("with a new LRP only supported in one of many zones", func() {
				BeforeEach(func() {
					startAuction = BuildLRPAuction("pg-win-2", "domain", 1, windowsRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
				})

				Context("when it picks a winner", func() {
					BeforeEach(func() {
						clock.Increment(time.Minute)
						s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
						results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
					})

					It("picks the best cell for the job", func() {
						Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
						Expect(clients["B-cell"].PerformCallCount()).To(Equal(0))
						Expect(clients["C-cell"].PerformCallCount()).To(Equal(1))

						_, work := clients["C-cell"].PerformArgsForCall(0)
						Expect(work.LRPs).To(ConsistOf(startAuction.LRP))
					})

					It("marks the start auction as succeeded", func() {
						setLRPWinner("C-cell", &startAuction)
						startAuction.WaitDuration = time.Minute
						Expect(results.SuccessfulLRPs).To(ConsistOf(startAuction))
						Expect(results.FailedLRPs).To(BeEmpty())
					})
				})
			})
		})

		Context("when filtering on volume drivers", func() {
			BeforeEach(func() {
				clients["A-cell"] = &repfakes.FakeSimClient{}
				zones["A-zone"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"A-cell",
						clients["A-cell"],
						BuildCellState("cellID", 0, "A-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-win-1", "domain", 0, "", 10, 10, 10, []string{}),
						}, []string{"driver-1", "driver-2"}, []string{}, []string{}, 0),
					),
				}

				clients["B-cell"] = &repfakes.FakeSimClient{}
				zones["B-zone"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"B-cell",
						clients["B-cell"],
						BuildCellState("cellID", 0, "B-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-win-1", "domain", 0, "", 10, 10, 10, []string{}),
						}, []string{"driver-3"}, []string{}, []string{}, 0),
					),
				}
			})

			Context("all cells does not have the required volume drivers", func() {
				BeforeEach(func() {
					startAuction = BuildLRPAuction("pg-4", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), []string{"driver-1", "driver-3"}, []string{})
					clock.Increment(time.Minute)

					s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
					results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
				})

				It("does not place the desired lrp on the cell", func() {
					Expect(len(results.FailedLRPs)).To(Equal(1))
					Expect(results.FailedLRPs[0].LRP).To(Equal(startAuction.LRP))
					Expect(results.FailedLRPs[0].AuctionRecord.PlacementError).To(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
				})
			})

			Context("a cell has the required volume drivers", func() {
				BeforeEach(func() {
					startAuction = BuildLRPAuction("pg-4", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), []string{"driver-3"}, []string{})
					clock.Increment(time.Minute)

					s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
					results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
				})

				It("does place the desired lrp on the cell", func() {
					Expect(len(results.FailedLRPs)).To(Equal(0))
					Expect(len(results.SuccessfulLRPs)).To(Equal(1))
					Expect(results.SuccessfulLRPs[0].LRP).To(Equal(startAuction.LRP))

					Expect(clients["B-cell"].PerformCallCount()).To(Equal(1))
					_, work := clients["B-cell"].PerformArgsForCall(0)
					Expect(len(work.LRPs)).To(Equal(1))
					Expect(work.LRPs[0]).To(Equal(startAuction.LRP))
				})
			})
		})

		Context("filtering on placement tags", func() {
			var (
				scheduler      *auctionrunner.Scheduler
				auctionRequest auctiontypes.AuctionRequest
			)

			BeforeEach(func() {
				clients["cell-z1-1"] = &repfakes.FakeSimClient{}
				clients["cell-z1-2"] = &repfakes.FakeSimClient{}
				zones["z1"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"cell-z1-1",
						clients["cell-z1-1"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{"quack", "moo"}),
						}, []string{}, []string{"quack", "moo"}, []string{"chirp"}, 0),
					),
					auctionrunner.NewCell(
						logger,
						"cell-z1-2",
						clients["cell-z1-2"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{}),
						}, []string{}, []string{}, []string{}, 0),
					),
				}

				clients["cell-z2-1"] = &repfakes.FakeSimClient{}
				zones["z2"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"cell-z2-1",
						clients["cell-z2-1"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{"quack"}),
						}, []string{}, []string{"quack"}, []string{"chirp", "baa"}, 0),
					),
					auctionrunner.NewCell(
						logger,
						"cell-z2-2",
						clients["cell-z2-2"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{"quack", "moo", "oink"}),
						}, []string{}, []string{"quack", "moo", "oink"}, []string{}, 0),
					),
				}

				startAuction = BuildLRPAuction("pg-5", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), []string{}, []string{"moo", "quack"})
			})

			JustBeforeEach(func() {
				auctionRequest = auctiontypes.AuctionRequest{
					LRPs:  []auctiontypes.LRPAuction{startAuction},
					Tasks: []auctiontypes.TaskAuction{},
				}
				scheduler = auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, defaultStartingContainerCountMaximum)
			})

			It("places the lrp on a cell with matching placement tags", func() {
				results := scheduler.Schedule(auctionRequest)
				Expect(len(results.SuccessfulLRPs)).To(Equal(1))
				Expect(results.SuccessfulLRPs[0].LRP).To(Equal(startAuction.LRP))

				Expect(clients["cell-z1-1"].PerformCallCount()).To(Equal(1))
				_, work := clients["cell-z1-1"].PerformArgsForCall(0)
				Expect(len(work.LRPs)).To(Equal(1))
				Expect(work.LRPs[0]).To(Equal(startAuction.LRP))
			})

			Context("when no cells have the required placement tag", func() {
				BeforeEach(func() {
					startAuction = BuildLRPAuction("pg-5", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), []string{}, []string{"oink", "kakaaaaa"})
				})

				It("does not place the lrp on a cell", func() {
					results := scheduler.Schedule(auctionRequest)
					Expect(len(results.SuccessfulLRPs)).To(Equal(0))
					Expect(len(results.FailedLRPs)).To(Equal(1))
					Expect(results.FailedLRPs[0].LRP).To(Equal(startAuction.LRP))
					Expect(results.FailedLRPs[0].AuctionRecord.PlacementError).To(ContainSubstring("found no compatible cell with placement tags "))
					Expect(results.FailedLRPs[0].AuctionRecord.PlacementError).To(ContainSubstring("\"kakaaaaa\""))
					Expect(results.FailedLRPs[0].AuctionRecord.PlacementError).To(ContainSubstring("\"oink\""))
				})
			})

			Context("when cells have the required placement tag and matching optional tag", func() {
				BeforeEach(func() {
					startAuction = BuildLRPAuction("pg-5", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), []string{}, []string{"quack", "chirp"})
				})

				It("places the lrp on a cell with matching placement tags", func() {
					results := scheduler.Schedule(auctionRequest)
					Expect(len(results.SuccessfulLRPs)).To(Equal(1))
					Expect(results.SuccessfulLRPs[0].LRP).To(Equal(startAuction.LRP))

					Expect(clients["cell-z2-1"].PerformCallCount()).To(Equal(1))
					_, work := clients["cell-z2-1"].PerformArgsForCall(0)
					Expect(len(work.LRPs)).To(Equal(1))
					Expect(work.LRPs[0]).To(Equal(startAuction.LRP))
				})
			})
		})

		Context("with an existing LRP (zone balancing)", func() {
			BeforeEach(func() {
				startAuction = BuildLRPAuction("pg-3", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
			})

			Context("when it picks a winner", func() {
				BeforeEach(func() {
					clock.Increment(time.Minute)

					s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
					results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
				})

				It("picks the best cell for the job", func() {
					Expect(clients["A-cell"].PerformCallCount()).To(Equal(1))
					Expect(clients["B-cell"].PerformCallCount()).To(Equal(0))

					_, startsToA := clients["A-cell"].PerformArgsForCall(0)
					Expect(startsToA.LRPs).To(ConsistOf(startAuction.LRP))
				})

				It("marks the start auction as succeeded", func() {
					setLRPWinner("A-cell", &startAuction)
					startAuction.WaitDuration = time.Minute
					Expect(results.SuccessfulLRPs).To(ConsistOf(startAuction))
					Expect(results.FailedLRPs).To(BeEmpty())
				})
			})
		})

		Context("with a new LRP (cell balancing)", func() {
			BeforeEach(func() {
				startAuction = BuildLRPAuction("pg-4", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
			})

			Context("when it picks a winner", func() {
				BeforeEach(func() {
					clock.Increment(time.Minute)
					s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
					results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
				})

				It("picks the best cell for the job", func() {
					Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
					Expect(clients["B-cell"].PerformCallCount()).To(Equal(1))

					_, startsToB := clients["B-cell"].PerformArgsForCall(0)
					Expect(startsToB.LRPs).To(ConsistOf(startAuction.LRP))
				})

				It("marks the start auction as succeeded", func() {
					setLRPWinner("B-cell", &startAuction)
					startAuction.WaitDuration = time.Minute
					Expect(results.SuccessfulLRPs).To(ConsistOf(startAuction))
					Expect(results.FailedLRPs).To(BeEmpty())
				})
			})
		})

		Context("when the cell rejects the start auction", func() {
			BeforeEach(func() {
				startAuction = BuildLRPAuction("pg-3", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})

				clients["A-cell"].PerformReturns(rep.Work{LRPs: []rep.LRP{startAuction.LRP}}, nil)
				clients["B-cell"].PerformReturns(rep.Work{LRPs: []rep.LRP{startAuction.LRP}}, nil)

				clock.Increment(time.Minute)
				s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
				results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
			})

			It("marks the start auction as failed", func() {
				startAuction.Attempts = 1
				Expect(results.SuccessfulLRPs).To(BeEmpty())
				Expect(results.FailedLRPs).To(ConsistOf(startAuction))
			})
		})

		Context("when the startingContainerCountMaximum is set", func() {

			var (
				startingContainerCountMaximum int
				startAuctions                 []auctiontypes.LRPAuction
			)

			BeforeEach(func() {
				pg70 := BuildLRPAuction("pg-7", "domain", 0, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
				pg71 := BuildLRPAuction("pg-7", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
				pg81 := BuildLRPAuction("pg-8", "domain", 1, linuxRootFSURL, 40, 40, 40, clock.Now(), nil, []string{})
				pg82 := BuildLRPAuction("pg-8", "domain", 2, linuxRootFSURL, 40, 40, 40, clock.Now(), nil, []string{})
				pg90 := BuildLRPAuction("pg-9", "domain", 0, linuxRootFSURL, 40, 40, 40, clock.Now(), nil, []string{})

				startAuctions = []auctiontypes.LRPAuction{pg70, pg71, pg81, pg82, pg90}
			})

			Context("when auctioning more than the maximum inflight container creations", func() {

				BeforeEach(func() {
					startingContainerCountMaximum = 3
				})

				It("only starts the maximum number of containers", func() {
					scheduler = auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, startingContainerCountMaximum)
					results = scheduler.Schedule(auctiontypes.AuctionRequest{LRPs: startAuctions})

					Expect(results.SuccessfulLRPs).To(HaveLen(startingContainerCountMaximum))
					Expect(results.FailedLRPs).To(HaveLen(len(startAuctions) - startingContainerCountMaximum))
				})
			})

			Context("when the maximum inflight container creations is negative", func() {

				BeforeEach(func() {
					startingContainerCountMaximum = -3
				})

				It("should behave as if there is no limit", func() {
					scheduler = auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, startingContainerCountMaximum)
					results = scheduler.Schedule(auctiontypes.AuctionRequest{LRPs: startAuctions})

					Expect(results.SuccessfulLRPs).To(HaveLen(len(startAuctions)))
					Expect(results.FailedLRPs).To(BeEmpty())
				})
			})
		})

		Context("when there is no room", func() {
			var requestedDisk int32

			BeforeEach(func() {
				requestedDisk = 50
			})

			JustBeforeEach(func() {
				startAuction = BuildLRPAuction("pg-4", "domain", 0, linuxRootFSURL, 1000, requestedDisk, 10, clock.Now(), []string{}, []string{})
				clock.Increment(time.Minute)
				s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
				results = s.Schedule(auctiontypes.AuctionRequest{LRPs: []auctiontypes.LRPAuction{startAuction}})
			})

			It("should not attempt to start the LRP", func() {
				Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
				Expect(clients["B-cell"].PerformCallCount()).To(Equal(0))
			})

			It("should log an error indicating the LRP and which resources failed", func() {
				logText := string(logger.Buffer().Contents())
				Expect(logText).To(MatchRegexp("lrp-auction-failed.*insufficient resources: memory.*pg-4"))
				Expect(logText).To(MatchRegexp("lrp-auction-failed.*lrp-placement-constraints.*"))
				Expect(logText).To(MatchRegexp(".*cells-failing-score-for-lrp.*"))
				Expect(logText).To(MatchRegexp(".*AvailableResources.*"))
				Expect(logText).ToNot(MatchRegexp(".*LRPs.*"))
			})

			It("should mark the start auction as failed", func() {
				Expect(results.SuccessfulLRPs).To(BeEmpty())
				Expect(results.FailedLRPs).To(HaveLen(1))
				failedLRP := results.FailedLRPs[0]
				Expect(failedLRP.Attempts).To(Equal(1))
				Expect(failedLRP.PlacementError).To(Equal("insufficient resources: memory"))
			})

			Context("when both cells have not enough memory and disk", func() {
				BeforeEach(func() {
					requestedDisk = 1000
				})

				It("should mark the start auction as failed", func() {
					Expect(results.SuccessfulLRPs).To(BeEmpty())
					Expect(results.FailedLRPs).To(HaveLen(1))
					failedLRP := results.FailedLRPs[0]
					Expect(failedLRP.Attempts).To(Equal(1))
					Expect(failedLRP.PlacementError).To(Equal("insufficient resources: disk, memory"))
				})
			})

			Context("when some cells have not enough memory and some have not enough disk", func() {
				BeforeEach(func() {
					clients["C-cell"] = &repfakes.FakeSimClient{}
					zones["C-zone"] = auctionrunner.Zone{
						auctionrunner.NewCell(
							logger,
							"C-cell",
							clients["C-cell"],
							BuildCellState("cellID", 0, "C-zone", 1200, 5, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{}, []string{}, []string{}, []string{}, 0),
						),
					}
				})

				It("should mark the start auction as failed", func() {
					Expect(results.SuccessfulLRPs).To(BeEmpty())
					Expect(results.FailedLRPs).To(HaveLen(1))
					failedLRP := results.FailedLRPs[0]
					Expect(failedLRP.Attempts).To(Equal(1))
					Expect(failedLRP.PlacementError).To(Equal("insufficient resources"))
				})
			})
		})
	})

	Describe("handling task auctions", func() {
		var taskAuction auctiontypes.TaskAuction

		BeforeEach(func() {
			clients["A-cell"] = &repfakes.FakeSimClient{}
			zones["A-zone"] = auctionrunner.Zone{auctionrunner.NewCell(logger, "A-cell", clients["A-cell"], BuildCellState("cellID", 0, "A-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
				*BuildLRP("does-not-matter", "domain", 0, "", 10, 10, 10, []string{}),
				*BuildLRP("does-not-matter", "domain", 0, "", 10, 10, 10, []string{}),
			}, []string{"driver-1", "driver-2"}, []string{}, []string{}, 0))}

			clients["B-cell"] = &repfakes.FakeSimClient{}
			zones["B-zone"] = auctionrunner.Zone{auctionrunner.NewCell(logger, "B-cell", clients["B-cell"], BuildCellState("cellID", 0, "B-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
				*BuildLRP("does-not-matter", "domain", 0, "", 10, 10, 10, []string{}),
			}, []string{"driver-3"}, []string{}, []string{}, 0))}

			taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
			clock.Increment(time.Minute)
		})

		Context("when only one of many zones supports a specific RootFS", func() {
			BeforeEach(func() {
				clients["C-cell"] = &repfakes.FakeSimClient{}
				zones["C-zone"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"C-cell",
						clients["C-cell"],
						BuildCellState("cellID", 0, "C-zone", 100, 100, 100, false, 0, windowsOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("tg-win-1", "domain", 0, "", 10, 10, 10, []string{}),
						}, []string{}, []string{}, []string{}, 0),
					),
				}
			})

			Context("with a new Task only supported in one of many zones", func() {
				BeforeEach(func() {
					taskAuction = BuildTaskAuction(BuildTask("tg-win-2", "domain", windowsRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
				})

				Context("when it picks a winner", func() {
					BeforeEach(func() {
						clock.Increment(time.Minute)
						s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
						results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
					})

					It("picks the best cell for the job", func() {
						Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
						Expect(clients["B-cell"].PerformCallCount()).To(Equal(0))
						Expect(clients["C-cell"].PerformCallCount()).To(Equal(1))

						_, startsToC := clients["C-cell"].PerformArgsForCall(0)
						Expect(startsToC.Tasks).To(ConsistOf(taskAuction.Task))
					})

					It("marks the start auction as succeeded", func() {
						setTaskWinner("C-cell", &taskAuction)
						taskAuction.WaitDuration = time.Minute
						Expect(results.SuccessfulTasks).To(ConsistOf(taskAuction))
						Expect(results.FailedTasks).To(BeEmpty())
					})
				})
			})
		})

		Context("when filtering on volume drivers", func() {
			Context("the cell does not have all of the volume drivers", func() {
				BeforeEach(func() {
					taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{"no-compatible-driver"}, []string{}), clock.Now())
					clock.Increment(time.Minute)

					s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
					results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
				})

				It("does not consider the cell for the auction", func() {
					Expect(len(results.FailedTasks)).To(Equal(1))
					Expect(results.FailedTasks[0].Task).To(Equal(taskAuction.Task))
					Expect(results.FailedTasks[0].AuctionRecord.PlacementError).To(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
				})
			})

			Context("a cell has the required volume drivers", func() {
				BeforeEach(func() {
					taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{"driver-1", "driver-2"}, []string{}), clock.Now())
					clock.Increment(time.Minute)

					s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
					results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
				})

				It("considers that cell for the auction", func() {
					Expect(len(results.SuccessfulTasks)).To(Equal(1))
					Expect(results.SuccessfulTasks[0].Task).To(Equal(taskAuction.Task))
					Expect(clients["A-cell"].PerformCallCount()).To(Equal(1))

					_, work := clients["A-cell"].PerformArgsForCall(0)
					Expect(len(work.Tasks)).To(Equal(1))
					Expect(work.Tasks[0]).To(Equal(taskAuction.Task))
				})
			})
		})

		Context("filtering on placement tags", func() {
			var (
				scheduler      *auctionrunner.Scheduler
				auctionRequest auctiontypes.AuctionRequest
			)

			BeforeEach(func() {
				clients["cell-z1-1"] = &repfakes.FakeSimClient{}
				clients["cell-z1-2"] = &repfakes.FakeSimClient{}
				zones["z1"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"cell-z1-1",
						clients["cell-z1-1"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{"quack", "moo"}),
						}, []string{}, []string{"quack", "moo"}, []string{}, 0),
					),
					auctionrunner.NewCell(
						logger,
						"cell-z1-2",
						clients["cell-z1-2"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{}),
						}, []string{}, []string{}, []string{}, 0),
					),
				}

				clients["cell-z2-1"] = &repfakes.FakeSimClient{}
				zones["z2"] = auctionrunner.Zone{
					auctionrunner.NewCell(
						logger,
						"cell-z2-1",
						clients["cell-z2-1"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{"quack"}),
						}, []string{}, []string{"quack"}, []string{}, 0),
					),
					auctionrunner.NewCell(
						logger,
						"cell-z2-2",
						clients["cell-z2-2"],
						BuildCellState("cellID", 0, "z1", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
							*BuildLRP("pg-5", "domain", 0, "", 10, 10, 10, []string{"quack", "moo", "oink"}),
						}, []string{}, []string{"quack", "moo", "oink"}, []string{}, 0),
					),
				}

				taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{"quack"}), clock.Now())
			})

			JustBeforeEach(func() {
				auctionRequest = auctiontypes.AuctionRequest{
					LRPs:  []auctiontypes.LRPAuction{},
					Tasks: []auctiontypes.TaskAuction{taskAuction},
				}
				scheduler = auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, defaultStartingContainerCountMaximum)
			})

			It("places the task on a cell with matching placement tags", func() {
				results := scheduler.Schedule(auctionRequest)
				Expect(len(results.SuccessfulTasks)).To(Equal(1))
				Expect(results.SuccessfulTasks[0].Task).To(Equal(taskAuction.Task))

				Expect(clients["cell-z2-1"].PerformCallCount()).To(Equal(1))
				_, work := clients["cell-z2-1"].PerformArgsForCall(0)
				Expect(len(work.Tasks)).To(Equal(1))
				Expect(work.Tasks[0]).To(Equal(taskAuction.Task))
			})

			Context("when no cells have the required placement tag", func() {
				BeforeEach(func() {
					taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{"oink"}), clock.Now())
				})

				It("does not place the task on a cell", func() {
					results := scheduler.Schedule(auctionRequest)
					Expect(len(results.SuccessfulTasks)).To(Equal(0))
					Expect(len(results.FailedTasks)).To(Equal(1))
					Expect(results.FailedTasks[0].Task).To(Equal(taskAuction.Task))
					Expect(results.FailedTasks[0].AuctionRecord.PlacementError).To(ContainSubstring("found no compatible cell with placement tag \"oink\""))
				})
			})
		})

		Context("when it picks a winner", func() {
			BeforeEach(func() {
				s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
				results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
			})

			It("picks the best cell for the job", func() {
				Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
				Expect(clients["B-cell"].PerformCallCount()).To(Equal(1))

				_, tasksToB := clients["B-cell"].PerformArgsForCall(0)
				Expect(tasksToB.Tasks).To(ConsistOf(taskAuction.Task))
			})

			It("marks the task auction as succeeded", func() {
				Expect(results.SuccessfulTasks).To(HaveLen(1))
				successfulTask := results.SuccessfulTasks[0]
				Expect(successfulTask.Winner).To(Equal("B-cell"))
				Expect(successfulTask.Attempts).To(Equal(1))
				Expect(successfulTask.WaitDuration).To(Equal(time.Minute))
				Expect(results.FailedTasks).To(BeEmpty())
			})
		})

		Context("when the cell rejects the task", func() {
			BeforeEach(func() {
				clients["B-cell"].PerformReturns(rep.Work{Tasks: []rep.Task{taskAuction.Task}}, nil)
				s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
				results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
			})

			It("marks the task auction as failed", func() {
				Expect(results.SuccessfulTasks).To(BeEmpty())

				Expect(results.FailedTasks).To(HaveLen(1))
				failedTask := results.FailedTasks[0]
				Expect(failedTask.Attempts).To(Equal(1))
			})
		})

		Context("when there is no room", func() {
			var requestedDisk int32

			BeforeEach(func() {
				requestedDisk = 50
			})

			JustBeforeEach(func() {
				taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 1000, requestedDisk, 10, []string{}, []string{}), clock.Now())
				clock.Increment(time.Minute)
				s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
				results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
			})

			It("should not attempt to start the task", func() {
				Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
				Expect(clients["B-cell"].PerformCallCount()).To(Equal(0))
			})

			It("should mark the start auction as failed", func() {
				Expect(results.SuccessfulTasks).To(BeEmpty())

				Expect(results.FailedTasks).To(HaveLen(1))
				failedTask := results.FailedTasks[0]
				Expect(failedTask.Attempts).To(Equal(1))
				Expect(failedTask.PlacementError).To(Equal("insufficient resources: memory"))
			})

			Context("when both cells have not enough memory and disk", func() {
				BeforeEach(func() {
					requestedDisk = 1000
				})

				It("should mark the start auction as failed", func() {
					Expect(results.SuccessfulTasks).To(BeEmpty())
					Expect(results.FailedTasks).To(HaveLen(1))
					failedTask := results.FailedTasks[0]
					Expect(failedTask.Attempts).To(Equal(1))
					Expect(failedTask.PlacementError).To(Equal("insufficient resources: disk, memory"))
				})
			})

			Context("when some cells have not enough memory and some have not enough disk", func() {
				BeforeEach(func() {
					clients["C-cell"] = &repfakes.FakeSimClient{}
					zones["C-zone"] = auctionrunner.Zone{
						auctionrunner.NewCell(
							logger,
							"C-cell",
							clients["C-cell"],
							BuildCellState("cellID", 0, "C-zone", 1200, 5, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{}, []string{}, []string{}, []string{}, 0),
						),
					}
				})

				It("should mark the start auction as failed", func() {
					Expect(results.SuccessfulTasks).To(BeEmpty())
					Expect(results.FailedTasks).To(HaveLen(1))
					failedTask := results.FailedTasks[0]
					Expect(failedTask.Attempts).To(Equal(1))
					Expect(failedTask.PlacementError).To(Equal("insufficient resources"))
				})
			})
		})

		Context("when auctioning more than the maximum inflight container creations", func() {
			var startAuctions []auctiontypes.TaskAuction
			var startingContainerCountMaximum int

			BeforeEach(func() {
				startingContainerCountMaximum = 2
				taskAuction1 := BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
				taskAuction2 := BuildTaskAuction(BuildTask("tg-2", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
				taskAuction3 := BuildTaskAuction(BuildTask("tg-3", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
				taskAuction4 := BuildTaskAuction(BuildTask("tg-4", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())

				scheduler = auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, startingContainerCountMaximum)
				startAuctions = []auctiontypes.TaskAuction{taskAuction1, taskAuction2, taskAuction3, taskAuction4}
			})

			It("only starts the maximum number of containers", func() {
				results = scheduler.Schedule(auctiontypes.AuctionRequest{Tasks: startAuctions})
				Expect(results.SuccessfulTasks).To(HaveLen(startingContainerCountMaximum))
				Expect(results.FailedTasks).To(HaveLen(len(startAuctions) - startingContainerCountMaximum))
			})
		})

		Context("when there is cell mismatch", func() {
			BeforeEach(func() {
				taskAuction = BuildTaskAuction(BuildTask("tg-1", "domain", "unsupported:rootfs", 100, 100, 10, []string{}, []string{}), clock.Now())
				clock.Increment(time.Minute)
				s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
				results = s.Schedule(auctiontypes.AuctionRequest{Tasks: []auctiontypes.TaskAuction{taskAuction}})
			})

			It("should not attempt to start the task", func() {
				Expect(clients["A-cell"].PerformCallCount()).To(Equal(0))
				Expect(clients["B-cell"].PerformCallCount()).To(Equal(0))
			})

			It("should mark the start auction as failed", func() {
				Expect(results.SuccessfulTasks).To(BeEmpty())

				Expect(results.FailedTasks).To(HaveLen(1))
				failedTask := results.FailedTasks[0]
				Expect(failedTask.Attempts).To(Equal(1))
				Expect(failedTask.PlacementError).To(Equal(auctiontypes.ErrorCellMismatch.Error()))
			})
		})
	})

	Describe("a comprehensive scenario", func() {
		BeforeEach(func() {
			clients["A-cell"] = &repfakes.FakeSimClient{}
			zones["A-zone"] = auctionrunner.Zone{auctionrunner.NewCell(logger, "A-cell", clients["A-cell"], BuildCellState("cellID", 0, "A-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
				*BuildLRP("pg-1", "domain", 0, "", 10, 10, 10, []string{}),
				*BuildLRP("pg-2", "domain", 0, "", 10, 10, 10, []string{}),
			}, []string{}, []string{}, []string{}, 0))}

			clients["B-cell"] = &repfakes.FakeSimClient{}
			zones["B-zone"] = auctionrunner.Zone{auctionrunner.NewCell(logger, "B-cell", clients["B-cell"], BuildCellState("cellID", 0, "B-zone", 100, 100, 100, false, 0, linuxOnlyRootFSProviders, []rep.LRP{
				*BuildLRP("pg-3", "domain", 0, "", 10, 10, 10, []string{}),
				*BuildLRP("pg-4", "domain", 0, "", 20, 20, 10, []string{}),
			}, []string{}, []string{}, []string{}, 0))}
		})

		It("should optimize the distribution", func() {
			startPG3 := BuildLRPAuction(
				"pg-3", "domain", 1, linuxRootFSURL, 40, 40, 10,
				clock.Now(),
				nil,
				[]string{},
			)
			startPG2 := BuildLRPAuction(
				"pg-2", "domain", 1, linuxRootFSURL, 5, 5, 10,
				clock.Now(),
				nil,
				[]string{},
			)
			startPGNope := BuildLRPAuctionWithPlacementError(
				"pg-nope", "domain", 1, ".net", 10, 10, 10,
				clock.Now(),
				auctiontypes.ErrorCellMismatch.Error(),
				[]string{},
				[]string{},
			)

			taskAuction1 := BuildTaskAuction(
				BuildTask("tg-1", "domain", linuxRootFSURL, 40, 40, 10, []string{}, []string{}),
				clock.Now(),
			)
			taskAuction2 := BuildTaskAuction(
				BuildTask("tg-2", "domain", linuxRootFSURL, 5, 5, 10, []string{}, []string{}),
				clock.Now(),
			)
			taskAuctionNope := BuildTaskAuction(
				BuildTask("tg-nope", "domain", ".net", 1, 1, 10, []string{}, []string{}),
				clock.Now(),
			)

			auctionRequest := auctiontypes.AuctionRequest{
				LRPs:  []auctiontypes.LRPAuction{startPG3, startPG2, startPGNope},
				Tasks: []auctiontypes.TaskAuction{taskAuction1, taskAuction2, taskAuctionNope},
			}

			s := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
			results = s.Schedule(auctionRequest)

			Expect(clients["A-cell"].PerformCallCount()).To(Equal(1))
			Expect(clients["B-cell"].PerformCallCount()).To(Equal(1))

			_, aWork := clients["A-cell"].PerformArgsForCall(0)
			_, bWork := clients["B-cell"].PerformArgsForCall(0)

			Expect(aWork.LRPs).To(ConsistOf(startPG3.LRP))
			Expect(bWork.LRPs).To(ConsistOf(startPG2.LRP))

			Expect(aWork.Tasks).To(ConsistOf(taskAuction1.Task))
			Expect(bWork.Tasks).To(ConsistOf(taskAuction2.Task))

			setLRPWinner("A-cell", &startPG3)
			setLRPWinner("B-cell", &startPG2)
			Expect(results.SuccessfulLRPs).To(ConsistOf(startPG3, startPG2))

			Expect(results.SuccessfulTasks).To(HaveLen(2))
			var successfulTaskAuction1, successfulTaskAuction2 auctiontypes.TaskAuction
			for _, ta := range results.SuccessfulTasks {
				if ta.Identifier() == taskAuction1.Identifier() {
					successfulTaskAuction1 = ta
				} else if ta.Identifier() == taskAuction2.Identifier() {
					successfulTaskAuction2 = ta
				}
			}
			Expect(successfulTaskAuction1).NotTo(BeNil())
			Expect(successfulTaskAuction1.Attempts).To(Equal(1))
			Expect(successfulTaskAuction1.Winner).To(Equal("A-cell"))
			Expect(successfulTaskAuction2).NotTo(BeNil())
			Expect(successfulTaskAuction2.Attempts).To(Equal(1))
			Expect(successfulTaskAuction2.Winner).To(Equal("B-cell"))

			startPGNope.Attempts = 1
			Expect(results.FailedLRPs).To(ConsistOf(startPGNope))
			Expect(results.FailedTasks).To(HaveLen(1))

			failedTask := results.FailedTasks[0]
			Expect(failedTask.Identifier()).To(Equal(taskAuctionNope.Identifier()))
			Expect(failedTask.Attempts).To(Equal(1))
		})
	})

	Describe("ordering work", func() {
		var (
			pg70, pg71, pg81, pg82 auctiontypes.LRPAuction
			tg1, tg2               auctiontypes.TaskAuction
			memory                 int32

			lrps  []auctiontypes.LRPAuction
			tasks []auctiontypes.TaskAuction
		)

		BeforeEach(func() {
			clients["cell"] = &repfakes.FakeSimClient{}

			pg70 = BuildLRPAuction("pg-7", "domain", 0, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
			pg71 = BuildLRPAuction("pg-7", "domain", 1, linuxRootFSURL, 10, 10, 10, clock.Now(), nil, []string{})
			pg81 = BuildLRPAuction("pg-8", "domain", 1, linuxRootFSURL, 40, 40, 10, clock.Now(), nil, []string{})
			pg82 = BuildLRPAuction("pg-8", "domain", 2, linuxRootFSURL, 40, 40, 10, clock.Now(), nil, []string{})
			lrps = []auctiontypes.LRPAuction{pg70, pg71, pg81, pg82}

			tg1 = BuildTaskAuction(BuildTask("tg-1", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}), clock.Now())
			tg2 = BuildTaskAuction(BuildTask("tg-2", "domain", linuxRootFSURL, 20, 20, 10, []string{}, []string{}), clock.Now())
			tasks = []auctiontypes.TaskAuction{tg1, tg2}

			memory = 100
		})

		JustBeforeEach(func() {
			zones["zone"] = auctionrunner.Zone{
				auctionrunner.NewCell(logger, "cell", clients["cell"], BuildCellState("cellID", 0, "zone", memory, 1000, 1000, false, 0, linuxOnlyRootFSProviders, []rep.LRP{}, []string{}, []string{}, []string{}, 0)),
			}

			auctionRequest := auctiontypes.AuctionRequest{
				LRPs:  lrps,
				Tasks: tasks,
			}

			scheduler := auctionrunner.NewScheduler(workPool, zones, clock, logger, 0.0, 0.0, 0)
			results = scheduler.Schedule(auctionRequest)
		})

		Context("where there are sufficient resources", func() {
			BeforeEach(func() {
				memory = 130
			})

			It("schedules all LPRs and tasks", func() {
				setLRPWinner("cell", &pg70, &pg71, &pg81, &pg82)
				setTaskWinner("cell", &tg1, &tg2)

				Expect(results.SuccessfulLRPs).To(ConsistOf(pg70, pg71, pg81, pg82))
				Expect(results.SuccessfulTasks).To(ConsistOf(tg1, tg2))
			})
		})

		Context("when there are insufficient resources", func() {
			BeforeEach(func() {
				memory = 10
			})

			It("schedules LRP instances with index 0 first", func() {
				setLRPWinner("cell", &pg70)

				Expect(results.SuccessfulLRPs).To(ConsistOf(pg70))
				Expect(results.SuccessfulTasks).To(BeEmpty())
			})

			Context("with just a bit more resources", func() {
				BeforeEach(func() {
					memory = 45
				})

				It("schedules tasks before LRP instances with index > 0", func() {
					setLRPWinner("cell", &pg70)
					setTaskWinner("cell", &tg1, &tg2)

					Expect(results.SuccessfulLRPs).To(ConsistOf(pg70))
					Expect(results.SuccessfulTasks).To(ConsistOf(tg1, tg2))
				})

				Context("with even more resources", func() {
					BeforeEach(func() {
						memory = 95
					})

					It("schedules LRPs with index > 0 after tasks, by index", func() {
						setLRPWinner("cell", &pg70, &pg71, &pg81)
						setTaskWinner("cell", &tg1, &tg2)

						Expect(results.SuccessfulLRPs).To(ConsistOf(pg70, pg71, pg81))
						Expect(results.SuccessfulTasks).To(ConsistOf(tg1, tg2))
					})
				})
			})
		})

		Context("when LRP indices match", func() {
			BeforeEach(func() {
				memory = 80
			})

			It("schedules boulders before pebbles", func() {
				setLRPWinner("cell", &pg70, &pg81)
				setTaskWinner("cell", &tg1, &tg2)

				Expect(results.SuccessfulLRPs).To(ConsistOf(pg70, pg81))
				Expect(results.SuccessfulTasks).To(ConsistOf(tg1, tg2))
			})
		})

		Context("when dealing with tasks", func() {
			var tg3 auctiontypes.TaskAuction

			BeforeEach(func() {
				tg3 = BuildTaskAuction(BuildTask("tg-3", "domain", linuxRootFSURL, 30, 30, 10, []string{}, []string{}), clock.Now())
				lrps = []auctiontypes.LRPAuction{}
				tasks = append(tasks, tg3)
				memory = tg3.MemoryMB + 1
			})

			It("schedules boulders before pebbles", func() {
				setTaskWinner("cell", &tg3)
				Expect(results.SuccessfulTasks).To(ConsistOf(tg3))
			})
		})
	})
})

func setLRPWinner(cellName string, lrps ...*auctiontypes.LRPAuction) {
	for _, l := range lrps {
		l.Winner = cellName
		l.Attempts++
	}
}

func setTaskWinner(cellName string, tasks ...*auctiontypes.TaskAuction) {
	for _, t := range tasks {
		t.Winner = cellName
		t.Attempts++
	}
}
