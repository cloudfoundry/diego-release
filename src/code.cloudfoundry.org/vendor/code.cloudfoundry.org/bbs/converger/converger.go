package converger

import (
	"context"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"
	uuid "github.com/nu7hatch/gouuid"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/clock"
)

//go:generate counterfeiter -generate

//counterfeiter:generate -o fake_controllers/fake_lrp_convergence_controller.go . LrpConvergenceController
type LrpConvergenceController interface {
	ConvergeLRPs(ctx context.Context)
}

//counterfeiter:generate -o fake_controllers/fake_task_controller.go . TaskController
type TaskController interface {
	ConvergeTasks(ctx context.Context, logger lager.Logger, kickTaskDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration) error
}

type Converger struct {
	id                          string
	serviceClient               serviceclient.ServiceClient
	lrpConvergenceController    LrpConvergenceController
	taskController              TaskController
	logger                      lager.Logger
	clock                       clock.Clock
	convergeRepeatInterval      time.Duration
	kickTaskDuration            time.Duration
	expirePendingTaskDuration   time.Duration
	expireCompletedTaskDuration time.Duration
	closeOnce                   *sync.Once
}

func New(
	logger lager.Logger,
	clock clock.Clock,
	lrpConvergenceController LrpConvergenceController,
	taskController TaskController,
	serviceClient serviceclient.ServiceClient,
	convergeRepeatInterval,
	kickTaskDuration,
	expirePendingTaskDuration,
	expireCompletedTaskDuration time.Duration,
) *Converger {

	uuid, err := uuid.NewV4()
	if err != nil {
		panic("Failed to generate a random guid....:" + err.Error())
	}

	return &Converger{
		id:                          uuid.String(),
		logger:                      logger,
		clock:                       clock,
		serviceClient:               serviceClient,
		lrpConvergenceController:    lrpConvergenceController,
		taskController:              taskController,
		convergeRepeatInterval:      convergeRepeatInterval,
		kickTaskDuration:            kickTaskDuration,
		expirePendingTaskDuration:   expirePendingTaskDuration,
		expireCompletedTaskDuration: expireCompletedTaskDuration,
		closeOnce:                   &sync.Once{},
	}
}

func (c *Converger) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("converger-process")
	logger.Info("started")

	convergeTimer := c.clock.NewTimer(c.convergeRepeatInterval)
	defer func() {
		logger.Info("done")
		convergeTimer.Stop()
	}()

	cellEvents := c.serviceClient.CellEvents(logger)
	convergeChan := make(chan struct{}, 2)

	close(ready)

	for {
		select {
		case <-signals:
			return nil

		case event := <-cellEvents:
			// Stopping the timer in order to avoid a race condition in the tests.
			// Executing Stop() removes the timer from the list of watchers on the clock
			// which allows us to use WaitForWatcherAndIncrement on the fake clock.
			convergeTimer.Stop()
			switch event.EventType() {
			case models.EventTypeCellDisappeared:
				logger.Info("received-cell-disappeared-event", lager.Data{"cell-id": event.CellIDs()})
				c.converge(convergeChan)
			}

		case <-convergeTimer.C():
			convergeTimer.Stop()
			c.converge(convergeChan)
		}

		select {
		case <-signals:
			return nil
		case <-convergeChan:
		}
		select {
		case <-signals:
			return nil
		case <-convergeChan:
		}

		convergeTimer.Reset(c.convergeRepeatInterval)
	}
}

func (c *Converger) converge(convergeChan chan struct{}) {
	logger := c.logger.Session("executing-convergence")

	go func() {
		logger.Info("converge-tasks-started")
		defer logger.Info("converge-tasks-done")

		err := c.taskController.ConvergeTasks(
			context.Background(),
			c.logger,
			c.kickTaskDuration,
			c.expirePendingTaskDuration,
			c.expireCompletedTaskDuration,
		)
		if err != nil {
			logger.Error("failed-to-converge-tasks", err)
		}

		convergeChan <- struct{}{}
	}()

	go func() {
		logger.Info("converge-lrps-started")
		defer logger.Info("converge-lrps-done")

		c.lrpConvergenceController.ConvergeLRPs(context.Background())

		convergeChan <- struct{}{}
	}()
}
