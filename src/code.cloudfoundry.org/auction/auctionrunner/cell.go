package auctionrunner

import (
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

const LocalityOffset = 1000

type Cell struct {
	logger lager.Logger
	Guid   string
	client rep.Client
	state  rep.CellState
	Index  int

	workToCommit rep.Work
}

func NewCell(logger lager.Logger, guid string, client rep.Client, state rep.CellState) *Cell {
	return &Cell{
		logger:       logger,
		Guid:         guid,
		client:       client,
		state:        state,
		Index:        state.CellIndex,
		workToCommit: rep.Work{CellID: guid},
	}
}

func (c *Cell) StartingContainerCount() int {
	return c.state.StartingContainerCount
}

func (c *Cell) MatchRootFS(rootFS string) bool {
	return c.state.MatchRootFS(rootFS)
}

func (c *Cell) MatchVolumeDrivers(volumeDrivers []string) bool {
	return c.state.MatchVolumeDrivers(volumeDrivers)
}

func (c *Cell) MatchPlacementTags(placementTags []string) bool {
	return c.state.MatchPlacementTags(placementTags)
}

func (c *Cell) State() rep.CellState {
	return c.state
}

func (c *Cell) ScoreForLRP(lrp *rep.LRP, startingContainerWeight, binPackFirstFitWeight float64) (float64, error) {
	proxiedLRP := rep.Resource{
		MemoryMB: lrp.Resource.MemoryMB + int32(c.state.ProxyMemoryAllocationMB),
		DiskMB:   lrp.Resource.DiskMB,
		MaxPids:  lrp.Resource.MaxPids,
	}

	err := c.state.ResourceMatch(&proxiedLRP)
	if err != nil {
		return 0, err
	}

	numberOfInstancesWithMatchingProcessGuid := 0
	for i := range c.state.LRPs {
		if c.state.LRPs[i].ProcessGuid == lrp.ProcessGuid {
			numberOfInstancesWithMatchingProcessGuid++
		}
	}

	localityScore := LocalityOffset * numberOfInstancesWithMatchingProcessGuid

	resourceScore := c.state.ComputeScore(&proxiedLRP, startingContainerWeight)

	indexScore := float64(c.Index) * binPackFirstFitWeight

	c.logger.Debug("score-for-lrp", lager.Data{
		"cell-guid":      c.Guid,
		"cell-index":     c.state.CellIndex,
		"locality-score": localityScore,
		"resource-score": resourceScore,
		"index-score":    indexScore,
		"score":          resourceScore + float64(localityScore) + indexScore,
	})
	return resourceScore + float64(localityScore) + indexScore, nil
}

func (c *Cell) ScoreForTask(task *rep.Task, startingContainerWeight float64) (float64, error) {
	err := c.state.ResourceMatch(&task.Resource)
	if err != nil {
		return 0, err
	}

	localityScore := LocalityOffset * len(c.state.Tasks)
	resourceScore := c.state.ComputeScore(&task.Resource, startingContainerWeight)
	return resourceScore + float64(localityScore), nil
}

func (c *Cell) ReserveLRP(lrp *rep.LRP) error {
	err := c.state.ResourceMatch(&lrp.Resource)
	if err != nil {
		return err
	}

	c.state.AddLRP(lrp)
	c.workToCommit.LRPs = append(c.workToCommit.LRPs, *lrp)
	return nil
}

func (c *Cell) ReserveTask(task *rep.Task) error {
	err := c.state.ResourceMatch(&task.Resource)
	if err != nil {
		return err
	}

	c.state.AddTask(task)
	c.workToCommit.Tasks = append(c.workToCommit.Tasks, *task)
	return nil
}

func (c *Cell) Commit() rep.Work {
	if len(c.workToCommit.LRPs) == 0 && len(c.workToCommit.Tasks) == 0 {
		return rep.Work{}
	}

	failedWork, err := c.client.Perform(c.logger, c.workToCommit)
	if err != nil {
		c.logger.Error("failed-to-commit", err, lager.Data{"cell-guid": c.Guid})
		//an error may indicate partial failure
		//in this case we don't reschedule work in order to make sure we don't
		//create duplicates of things -- we'll let the converger figure things out for us later
		return rep.Work{}
	}
	return failedWork
}
