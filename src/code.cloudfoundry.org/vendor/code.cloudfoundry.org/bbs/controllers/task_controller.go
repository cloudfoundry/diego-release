package controllers

import (
	"context"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/taskworkpool"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

type TaskController struct {
	db                     db.TaskDB
	taskCompletionClient   taskworkpool.TaskCompletionClient
	auctioneerClient       auctioneer.Client
	serviceClient          serviceclient.ServiceClient
	repClientFactory       rep.ClientFactory
	taskHub                events.Hub
	taskStatMetronNotifier metrics.TaskStatMetronNotifier
	maxRetries             int
}

func NewTaskController(
	db db.TaskDB,
	taskCompletionClient taskworkpool.TaskCompletionClient,
	auctioneerClient auctioneer.Client,
	serviceClient serviceclient.ServiceClient,
	repClientFactory rep.ClientFactory,
	taskHub events.Hub,
	taskStatMetronNotifier metrics.TaskStatMetronNotifier,
	maxRetries int,
) *TaskController {
	return &TaskController{
		db:                     db,
		taskCompletionClient:   taskCompletionClient,
		auctioneerClient:       auctioneerClient,
		serviceClient:          serviceClient,
		repClientFactory:       repClientFactory,
		taskHub:                taskHub,
		taskStatMetronNotifier: taskStatMetronNotifier,
		maxRetries:             maxRetries,
	}
}

func (c *TaskController) Tasks(ctx context.Context, logger lager.Logger, domain, cellID string) ([]*models.Task, error) {
	logger = logger.Session("tasks")

	filter := models.TaskFilter{Domain: domain, CellID: cellID}
	return c.db.Tasks(ctx, logger, filter)
}

func (c *TaskController) TaskByGuid(ctx context.Context, logger lager.Logger, taskGUID string) (*models.Task, error) {
	logger = logger.Session("task-by-guid")

	return c.db.TaskByGuid(ctx, logger, taskGUID)
}

func (c *TaskController) DesireTask(ctx context.Context, logger lager.Logger, taskDefinition *models.TaskDefinition, taskGUID, domain string) error {
	var err error
	var task *models.Task
	logger = logger.Session("desire-task")

	logger = logger.WithData(lager.Data{"task_guid": taskGUID})

	task, err = c.db.DesireTask(ctx, logger, taskDefinition, taskGUID, domain)
	if err != nil {
		return err
	}
	go c.taskHub.Emit(models.NewTaskCreatedEvent(task))

	logger.Debug("start-task-auction-request")
	taskStartRequest := auctioneer.NewTaskStartRequestFromModel(taskGUID, domain, taskDefinition)
	err = c.auctioneerClient.RequestTaskAuctions(logger, trace.RequestIdFromContext(ctx), []*auctioneer.TaskStartRequest{&taskStartRequest})
	if err != nil {
		logger.Error("failed-requesting-task-auction", err)
		// The creation succeeded, the auction request error can be dropped
	} else {
		logger.Debug("succeeded-requesting-task-auction")
	}

	return nil
}

func (c *TaskController) StartTask(ctx context.Context, logger lager.Logger, taskGUID, cellID string) (shouldStart bool, err error) {
	logger = logger.Session("start-task", lager.Data{"task_guid": taskGUID, "cell_id": cellID})
	before, after, shouldStart, err := c.db.StartTask(ctx, logger, taskGUID, cellID)
	if err == nil && shouldStart {
		go c.taskHub.Emit(models.NewTaskChangedEvent(before, after))
		c.taskStatMetronNotifier.RecordTaskStarted(cellID)
	}
	return shouldStart, err
}

func (c *TaskController) CancelTask(ctx context.Context, logger lager.Logger, taskGUID string) error {
	logger = logger.Session("cancel-task")

	before, after, cellID, err := c.db.CancelTask(ctx, logger, taskGUID)
	if err != nil {
		return err
	}
	go c.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	if after.CompletionCallbackUrl != "" {
		logger.Info("task-client-completing-task")
		go c.taskCompletionClient.Submit(c.db, c.taskHub, after)
	}

	if cellID == "" {
		return nil
	}

	logger.Info("start-check-cell-presence", lager.Data{"cell_id": cellID})
	cellPresence, err := c.serviceClient.CellById(logger, cellID)
	if err != nil {
		logger.Error("failed-fetching-cell-presence", err)
		// don't return an error, the rep will converge later
		return nil
	}
	logger.Info("finished-check-cell-presence", lager.Data{"cell_id": cellID})

	repClient, err := c.repClientFactory.CreateClient(cellPresence.RepAddress, cellPresence.RepUrl, trace.RequestIdFromContext(ctx))
	if err != nil {
		logger.Error("create-rep-client-failed", err)
		return err
	}
	logger.Info("start-rep-cancel-task", lager.Data{"task_guid": taskGUID})
	err = repClient.CancelTask(logger, taskGUID)
	if err != nil {
		logger.Error("failed-rep-cancel-task", err)
		// don't return an error, the rep will converge later
		return nil
	}
	logger.Info("finished-rep-cancel-task", lager.Data{"task_guid": taskGUID})
	return nil
}

func (c *TaskController) FailTask(ctx context.Context, logger lager.Logger, taskGUID, failureReason string) error {
	var err error

	before, after, err := c.db.FailTask(ctx, logger, taskGUID, failureReason)
	if err != nil {
		return err
	}

	go c.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	if after.CompletionCallbackUrl != "" {
		logger.Info("task-client-completing-task")
		go c.taskCompletionClient.Submit(c.db, c.taskHub, after)
	}

	return nil
}

func (c *TaskController) RejectTask(ctx context.Context, logger lager.Logger, taskGUID, rejectionReason string) error {
	logger = logger.Session("reject-task", lager.Data{"guid": taskGUID})
	logger.Info("start")
	defer logger.Info("complete")

	task, err := c.db.TaskByGuid(ctx, logger, taskGUID)
	if err != nil {
		logger.Error("failed-to-fetch-task", err)
		return err
	}

	logger.Info("reject-task", lager.Data{"rejection-reason": rejectionReason})
	before, after, rejectTaskErr := c.db.RejectTask(ctx, logger, taskGUID, rejectionReason)
	if rejectTaskErr != nil {
		logger.Error("failed-to-reject-task", rejectTaskErr)
	}

	if int(task.RejectionCount) >= c.maxRetries {
		return c.FailTask(ctx, logger, taskGUID, rejectionReason)
	}

	go c.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	return rejectTaskErr
}

func (c *TaskController) CompleteTask(
	ctx context.Context,
	logger lager.Logger,
	taskGUID,
	cellID string,
	failed bool,
	failureReason,
	result string,
) error {
	var err error
	logger = logger.Session("complete-task")

	before, after, err := c.db.CompleteTask(ctx, logger, taskGUID, cellID, failed, failureReason, result)
	if err != nil {
		return err
	}
	go c.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	if failed {
		c.taskStatMetronNotifier.RecordTaskFailed(cellID)
	} else {
		c.taskStatMetronNotifier.RecordTaskSucceeded(cellID)
	}

	if after.CompletionCallbackUrl != "" {
		logger.Info("task-client-completing-task")
		go c.taskCompletionClient.Submit(c.db, c.taskHub, after)
	}

	return nil
}

func (c *TaskController) ResolvingTask(ctx context.Context, logger lager.Logger, taskGUID string) error {
	logger = logger.Session("resolving-task")

	before, after, err := c.db.ResolvingTask(ctx, logger, taskGUID)
	if err != nil {
		return err
	}
	go c.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	return nil
}

func (c *TaskController) DeleteTask(ctx context.Context, logger lager.Logger, taskGUID string) error {
	logger = logger.Session("delete-task")

	task, err := c.db.DeleteTask(ctx, logger, taskGUID)
	if err != nil {
		return err
	}
	go c.taskHub.Emit(models.NewTaskRemovedEvent(task))

	return nil
}

func (c *TaskController) ConvergeTasks(
	ctx context.Context,
	logger lager.Logger,
	kickTaskDuration,
	expirePendingTaskDuration,
	expireCompletedTaskDuration time.Duration,
) error {
	var err error
	logger = logger.Session("converge-tasks")

	logger.Debug("listing-cells")
	cellSet, err := c.serviceClient.Cells(logger)
	if err == models.ErrResourceNotFound {
		logger.Debug("no-cells-found")
		cellSet = models.CellSet{}
	} else if err != nil {
		logger.Debug("failed-listing-cells")
		return err
	}
	logger.Debug("succeeded-listing-cells")

	convergenceStartTime := time.Now()
	taskConvergenceResult := c.db.ConvergeTasks(
		ctx,
		logger,
		cellSet,
		kickTaskDuration,
		expirePendingTaskDuration,
		expireCompletedTaskDuration,
	)

	c.taskStatMetronNotifier.RecordTaskCounts(
		taskConvergenceResult.Metrics.TasksPending,
		taskConvergenceResult.Metrics.TasksRunning,
		taskConvergenceResult.Metrics.TasksCompleted,
		taskConvergenceResult.Metrics.TasksResolving,
		taskConvergenceResult.Metrics.TasksPruned,
		taskConvergenceResult.Metrics.TasksKicked,
	)

	c.taskStatMetronNotifier.RecordConvergenceDuration(time.Since(convergenceStartTime))

	logger.Debug("emitting-events-from-convergence", lager.Data{"num_tasks_to_complete": len(taskConvergenceResult.TasksToComplete)})
	for _, event := range taskConvergenceResult.Events {
		go c.taskHub.Emit(event)
	}

	if len(taskConvergenceResult.TasksToAuction) > 0 {
		logger.Debug("requesting-task-auctions", lager.Data{"num_tasks_to_auction": len(taskConvergenceResult.TasksToAuction)})
		err = c.auctioneerClient.RequestTaskAuctions(logger, trace.RequestIdFromContext(ctx), taskConvergenceResult.TasksToAuction)
		if err != nil {
			taskGuids := make([]string, len(taskConvergenceResult.TasksToAuction))
			for i, task := range taskConvergenceResult.TasksToAuction {
				taskGuids[i] = task.Task.TaskGuid
			}
			logger.Error("failed-to-request-auctions-for-pending-tasks", err,
				lager.Data{"task_guids": taskGuids})
		}
		logger.Debug("done-requesting-task-auctions", lager.Data{"num_tasks_to_auction": len(taskConvergenceResult.TasksToAuction)})
	}

	logger.Debug("submitting-tasks-to-be-completed", lager.Data{"num_tasks_to_complete": len(taskConvergenceResult.TasksToComplete)})
	for _, task := range taskConvergenceResult.TasksToComplete {
		c.taskCompletionClient.Submit(c.db, c.taskHub, task)
	}
	logger.Debug("done-submitting-tasks-to-be-completed", lager.Data{"num_tasks_to_complete": len(taskConvergenceResult.TasksToComplete)})

	return nil
}
