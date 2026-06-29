package db

import (
	"context"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

type CompleteTaskWork func(logger lager.Logger, taskDB TaskDB, task *models.Task) func()

type TaskConvergenceResult struct {
	TasksToAuction  []*auctioneer.TaskStartRequest
	TasksToComplete []*models.Task
	Events          []models.Event

	Metrics TaskMetrics
}

type TaskMetrics struct {
	TasksPending   int
	TasksRunning   int
	TasksCompleted int
	TasksResolving int
	TasksPruned    uint64
	TasksKicked    uint64
}

//counterfeiter:generate . TaskDB
type TaskDB interface {
	Tasks(ctx context.Context, logger lager.Logger, filter models.TaskFilter) ([]*models.Task, error)
	TaskByGuid(ctx context.Context, logger lager.Logger, taskGuid string) (*models.Task, error)

	DesireTask(ctx context.Context, logger lager.Logger, taskDefinition *models.TaskDefinition, taskGuid, domain string) (*models.Task, error)
	StartTask(ctx context.Context, logger lager.Logger, taskGuid, cellId string) (before *models.Task, after *models.Task, shouldStart bool, rr error)
	CancelTask(ctx context.Context, logger lager.Logger, taskGuid string) (before *models.Task, after *models.Task, cellID string, err error)
	FailTask(ctx context.Context, logger lager.Logger, taskGuid, failureReason string) (before *models.Task, after *models.Task, err error)
	RejectTask(ctx context.Context, logger lager.Logger, taskGuid, rejectionReason string) (before *models.Task, after *models.Task, err error)
	CompleteTask(ctx context.Context, logger lager.Logger, taskGuid, cellId string, failed bool, failureReason, result string) (before *models.Task, after *models.Task, err error)
	ResolvingTask(ctx context.Context, logger lager.Logger, taskGuid string) (before *models.Task, after *models.Task, err error)
	DeleteTask(ctx context.Context, logger lager.Logger, taskGuid string) (task *models.Task, err error)

	ConvergeTasks(ctx context.Context, logger lager.Logger, cellSet models.CellSet, kickTaskDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration) TaskConvergenceResult
}
