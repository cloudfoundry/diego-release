package auctionrunnerdelegate

import (
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/rep"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/lager/v3"
)

type AuctionRunnerDelegate struct {
	repClientFactory rep.ClientFactory
	bbsClient        bbs.InternalClient
}

func New(
	repClientFactory rep.ClientFactory,
	bbsClient bbs.InternalClient,
) *AuctionRunnerDelegate {
	return &AuctionRunnerDelegate{
		repClientFactory: repClientFactory,
		bbsClient:        bbsClient,
	}
}

func (a *AuctionRunnerDelegate) FetchCellReps(logger lager.Logger, traceID string) (map[string]rep.Client, error) {
	cells, err := a.bbsClient.Cells(logger, traceID)
	cellReps := map[string]rep.Client{}
	if err != nil {
		return cellReps, err
	}

	for _, cell := range cells {
		client, err := a.repClientFactory.CreateClient(cell.RepAddress, cell.RepUrl, traceID)
		if err != nil {
			logger.Error("create-rep-client-failed", err)
			continue
		}
		cellReps[cell.CellId] = client
	}

	return cellReps, nil
}

func (a *AuctionRunnerDelegate) AuctionCompleted(logger lager.Logger, traceID string, results auctiontypes.AuctionResults) {
	for i := range results.FailedTasks {
		task := &results.FailedTasks[i]
		err := a.bbsClient.RejectTask(logger, traceID, task.TaskGuid, task.PlacementError)
		if err != nil {
			logger.Error("failed-to-reject-task", err, lager.Data{
				"task":           task,
				"auction-result": "failed",
			})
		}
	}

	for i := range results.FailedLRPs {
		lrp := &results.FailedLRPs[i]
		err := a.bbsClient.FailActualLRP(logger, traceID, &lrp.ActualLRPKey, lrp.PlacementError)
		if err != nil {
			logger.Error("failed-to-fail-LRP", err, lager.Data{
				"lrp":            lrp,
				"auction-result": "failed",
			})
		}
	}
}
