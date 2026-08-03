package metrics

import (
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	logging "code.cloudfoundry.org/diego-logging-client"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

const (
	DefaultTaskEmitMetricsFrequency = 15 * time.Second

	ConvergenceTaskRunsMetric     = "ConvergenceTaskRuns"
	ConvergenceTaskDurationMetric = "ConvergenceTaskDuration"

	TasksStartedMetric   = "TasksStarted"
	TasksSucceededMetric = "TasksSucceeded"
	TasksFailedMetric    = "TasksFailed"

	TasksPendingMetric   = "TasksPending"
	TasksRunningMetric   = "TasksRunning"
	TasksCompletedMetric = "TasksCompleted"
	TasksResolvingMetric = "TasksResolving"

	ConvergenceTasksPrunedMetric = "ConvergenceTasksPruned"
	ConvergenceTasksKickedMetric = "ConvergenceTasksKicked"
)

//counterfeiter:generate -o fakes/fake_task_stat_metron_notifier.go . TaskStatMetronNotifier
type TaskStatMetronNotifier interface {
	ifrit.Runner

	RecordConvergenceDuration(duration time.Duration)
	RecordTaskStarted(cellID string)
	RecordTaskSucceeded(cellID string)
	RecordTaskFailed(cellID string)
	RecordTaskCounts(pending, running, completed, resolved int, pruned, kicked uint64)
}

type taskStatMetronNotifier struct {
	mutex        sync.Mutex
	clock        clock.Clock
	metricSender loggingMetricSender

	perCellMetrics map[string]perCellMetrics
	globalMetrics  globalMetrics
}

type perCellMetrics struct {
	tasksStarted, tasksFailed, tasksSucceeded int
}

type globalMetrics struct {
	convergenceTaskRuns     uint64
	convergenceTaskDuration time.Duration

	tasksPending, tasksRunning, tasksCompleted, tasksResolving int
	convergenceTasksPruned, convergenceTasksKicked             uint64
}

func NewTaskStatMetronNotifier(logger lager.Logger, clock clock.Clock, metronClient logging.IngressClient) TaskStatMetronNotifier {
	return &taskStatMetronNotifier{
		clock: clock,
		metricSender: loggingMetricSender{
			logger:       logger,
			metronClient: metronClient,
		},
		perCellMetrics: make(map[string]perCellMetrics),
	}
}

func (t *taskStatMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ticker := t.clock.NewTicker(DefaultEmitMetricsFrequency)
	close(ready)
	for {
		select {
		case <-ticker.C():
			t.emitMetrics()
		case <-signals:
			return nil
		}
	}
}

func (t *taskStatMetronNotifier) RecordConvergenceDuration(duration time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.globalMetrics.convergenceTaskRuns += 1
	t.globalMetrics.convergenceTaskDuration = duration
}

func (t *taskStatMetronNotifier) RecordTaskStarted(cellID string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	stats := t.perCellMetrics[cellID]
	stats.tasksStarted += 1
	t.perCellMetrics[cellID] = stats
}

func (t *taskStatMetronNotifier) RecordTaskSucceeded(cellID string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	stats := t.perCellMetrics[cellID]
	stats.tasksSucceeded += 1
	t.perCellMetrics[cellID] = stats
}

func (t *taskStatMetronNotifier) RecordTaskFailed(cellID string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	stats := t.perCellMetrics[cellID]
	stats.tasksFailed += 1
	t.perCellMetrics[cellID] = stats
}

func (t *taskStatMetronNotifier) RecordTaskCounts(pending, running, completed, resolving int, pruned, kicked uint64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.globalMetrics.tasksPending = pending
	t.globalMetrics.tasksRunning = running
	t.globalMetrics.tasksCompleted = completed
	t.globalMetrics.tasksResolving = resolving
	t.globalMetrics.convergenceTasksPruned = pruned
	t.globalMetrics.convergenceTasksKicked = kicked
}

func (t *taskStatMetronNotifier) emitMetrics() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	for cell, stats := range t.perCellMetrics {
		opt := loggregator.WithEnvelopeTag("cell-id", cell)
		t.metricSender.SendMetric(TasksStartedMetric, stats.tasksStarted, opt)
		t.metricSender.SendMetric(TasksFailedMetric, stats.tasksFailed, opt)
		t.metricSender.SendMetric(TasksSucceededMetric, stats.tasksSucceeded, opt)
	}

	t.metricSender.SendMetric(TasksPendingMetric, t.globalMetrics.tasksPending)
	t.metricSender.SendMetric(TasksRunningMetric, t.globalMetrics.tasksRunning)
	t.metricSender.SendMetric(TasksCompletedMetric, t.globalMetrics.tasksCompleted)
	t.metricSender.SendMetric(TasksResolvingMetric, t.globalMetrics.tasksResolving)
	t.metricSender.IncrementCounterWithDelta(ConvergenceTasksPrunedMetric, t.globalMetrics.convergenceTasksPruned)
	t.metricSender.IncrementCounterWithDelta(ConvergenceTasksKickedMetric, t.globalMetrics.convergenceTasksKicked)

	if t.globalMetrics.convergenceTaskRuns > 0 {
		t.metricSender.IncrementCounterWithDelta(ConvergenceTaskRunsMetric, t.globalMetrics.convergenceTaskRuns)
		t.globalMetrics.convergenceTaskRuns = 0
	}

	t.metricSender.SendDuration(ConvergenceTaskDurationMetric, t.globalMetrics.convergenceTaskDuration)
}
