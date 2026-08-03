package sqldb

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

const (
	expiredFailureReason         = "not started within time limit"
	cellDisappearedFailureReason = "cell disappeared before completion"
)

func (sqldb *SQLDB) ConvergeTasks(ctx context.Context, logger lager.Logger, cellSet models.CellSet, kickTasksDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration) db.TaskConvergenceResult {
	logger = logger.Session("db-converge-tasks")
	logger.Info("starting")
	defer logger.Info("complete")

	convergenceResult := db.TaskConvergenceResult{}

	// failedEvents is a list of tasks that have transitioned from the pending to the completed state (but expired and failed)
	// failedFetches are tasks that failed to deserialize (invalid task def)
	// rowsAffected are the number of pending tasks that have expired
	failedEvents, failedFetches, rowsAffected := sqldb.failExpiredPendingTasks(ctx, logger, expirePendingTaskDuration)
	convergenceResult.Events = append(convergenceResult.Events, failedEvents...)
	convergenceResult.Metrics.TasksPruned += failedFetches
	convergenceResult.Metrics.TasksKicked += uint64(rowsAffected)

	// tasksToAuction is a list of tasks in the pending state that have not expired and are being auctioned
	tasksToAuction, failedFetches := sqldb.getTaskStartRequestsForKickablePendingTasks(ctx, logger, expirePendingTaskDuration)
	convergenceResult.TasksToAuction = tasksToAuction
	convergenceResult.Metrics.TasksPruned += failedFetches
	convergenceResult.Metrics.TasksKicked += uint64(len(tasksToAuction))

	// failedEvents is a list of tasks that have transitioned from the running to completed state (but cell dissappeared and failed)
	// rowsAffected is the number of running tasks that have lost their cells
	failedEvents, failedFetches, rowsAffected = sqldb.failTasksWithDisappearedCells(ctx, logger, cellSet)
	convergenceResult.Events = append(convergenceResult.Events, failedEvents...)
	convergenceResult.Metrics.TasksPruned += failedFetches
	convergenceResult.Metrics.TasksKicked += uint64(rowsAffected)

	// do this first so that we now have "Completed" tasks before cleaning up
	// or re-sending the completion callback
	// demotedEvents is a list of tasks transitioning from resolving back to completed state (bc they exceeded kickTasksDuration)
	demotedEvents, failedFetches := sqldb.demoteKickableResolvingTasks(ctx, logger, kickTasksDuration)
	convergenceResult.Events = append(convergenceResult.Events, demotedEvents...)
	convergenceResult.Metrics.TasksPruned += failedFetches

	// removedEvents is a list of tasks in the completed stated that have been deleted bc the time since they initially changed to completed exceeded expireCompleteTaskDuration
	removedEvents, rowsAffected := sqldb.deleteExpiredCompletedTasks(ctx, logger, expireCompletedTaskDuration)
	convergenceResult.Events = append(convergenceResult.Events, removedEvents...)
	convergenceResult.Metrics.TasksPruned += uint64(rowsAffected)

	// tasksToComplete is a list of tasks in the complete state that have exceeded kickTasksDuration
	tasksToComplete, failedFetches := sqldb.getKickableCompleteTasksForCompletion(ctx, logger, kickTasksDuration)
	convergenceResult.TasksToComplete = tasksToComplete
	convergenceResult.Metrics.TasksPruned += failedFetches
	convergenceResult.Metrics.TasksKicked += uint64(len(tasksToComplete))

	convergenceResult.Metrics.TasksPending, convergenceResult.Metrics.TasksRunning, convergenceResult.Metrics.TasksCompleted, convergenceResult.Metrics.TasksResolving = sqldb.getTaskCountByState(ctx, logger)

	return convergenceResult
}

func (db *SQLDB) failExpiredPendingTasks(ctx context.Context, logger lager.Logger, expirePendingTaskDuration time.Duration) ([]models.Event, uint64, int64) {
	logger = logger.Session("fail-expired-pending-tasks")

	now := db.clock.Now()

	rows, err := db.all(ctx, logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND created_at < ?", models.Task_Pending, now.Add(-expirePendingTaskDuration).UnixNano())
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0, 0
	}
	defer rows.Close()

	tasks, validTaskGuids, invalidTasksCount, err := db.fetchTasks(ctx, logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-some-tasks", err)
	}

	wheres := []string{"state = ?", "created_at < ?"}
	bindings := []interface{}{models.Task_Pending, now.Add(-expirePendingTaskDuration).UnixNano()}

	if len(validTaskGuids) == 0 {
		return nil, uint64(invalidTasksCount), 0
	}

	wheres = append(wheres, fmt.Sprintf("guid IN (%s)", helpers.QuestionMarks(len(validTaskGuids))))
	for _, guid := range validTaskGuids {
		bindings = append(bindings, guid)
	}

	result, err := db.update(ctx, logger, db.db, tasksTable,
		helpers.SQLAttributes{
			"failed":             true,
			"failure_reason":     expiredFailureReason,
			"result":             "",
			"state":              models.Task_Completed,
			"first_completed_at": now.UnixNano(),
			"updated_at":         now.UnixNano(),
		},
		strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, uint64(invalidTasksCount), 0
	}

	var events []models.Event
	for _, task := range tasks {
		afterTask := *task
		afterTask.Failed = true
		afterTask.FailureReason = expiredFailureReason
		afterTask.Result = ""
		afterTask.State = models.Task_Completed
		afterTask.FirstCompletedAt = now.UnixNano()
		afterTask.UpdatedAt = now.UnixNano()

		events = append(events, models.NewTaskChangedEvent(task, &afterTask))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("failed-rows-affected", err)
		return events, uint64(invalidTasksCount), 0
	}
	return events, uint64(invalidTasksCount), rowsAffected
}

func (db *SQLDB) getTaskStartRequestsForKickablePendingTasks(ctx context.Context, logger lager.Logger, expirePendingTaskDuration time.Duration) ([]*models.TaskStartRequest, uint64) {
	logger = logger.Session("get-task-start-requests-for-kickable-pending-tasks")

	rows, err := db.all(ctx, logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND created_at > ?",
		models.Task_Pending, db.clock.Now().Add(-expirePendingTaskDuration).UnixNano(),
	)

	if err != nil {
		logger.Error("failed-query", err)
		return []*models.TaskStartRequest{}, math.MaxUint64
	}

	defer rows.Close()

	tasksToAuction := []*models.TaskStartRequest{}
	tasks, _, invalidTasksCount, err := db.fetchTasks(ctx, logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-some-tasks", err)
	}

	for _, task := range tasks {
		taskStartRequest := models.NewTaskStartRequestFromModel(task.TaskGuid, task.Domain, task.TaskDefinition)
		tasksToAuction = append(tasksToAuction, &taskStartRequest)
	}

	return tasksToAuction, uint64(invalidTasksCount)
}

func (db *SQLDB) failTasksWithDisappearedCells(ctx context.Context, logger lager.Logger, cellSet models.CellSet) ([]models.Event, uint64, int64) {
	logger = logger.Session("fail-tasks-with-disappeared-cells")

	values := make([]interface{}, 0, 1+len(cellSet))
	values = append(values, models.Task_Running)

	for k := range cellSet {
		values = append(values, k)
	}

	wheres := "state = ?"
	if len(cellSet) != 0 {
		wheres += fmt.Sprintf(" AND cell_id NOT IN (%s)", helpers.QuestionMarks(len(cellSet)))
	}
	now := db.clock.Now().UnixNano()

	rows, err := db.all(ctx, logger, db.db, tasksTable, taskColumns, helpers.NoLockRow, wheres, values...)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0, 0
	}
	defer rows.Close()

	tasks, validTaskGuids, invalidTasksCount, err := db.fetchTasks(ctx, logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
	}

	if len(validTaskGuids) == 0 {
		return nil, uint64(invalidTasksCount), 0
	}

	wheres += fmt.Sprintf(" AND guid IN (%s)", helpers.QuestionMarks(len(validTaskGuids)))

	for _, guid := range validTaskGuids {
		values = append(values, guid)
	}

	result, err := db.update(ctx, logger, db.db, tasksTable,
		helpers.SQLAttributes{
			"failed":             true,
			"failure_reason":     cellDisappearedFailureReason,
			"result":             "",
			"state":              models.Task_Completed,
			"first_completed_at": now,
			"updated_at":         now,
		},
		wheres, values...,
	)
	if err != nil {
		logger.Error("failed-updating-tasks", err)
		return nil, uint64(invalidTasksCount), 0
	}

	var events []models.Event
	for _, task := range tasks {
		afterTask := *task
		afterTask.Failed = true
		afterTask.FailureReason = cellDisappearedFailureReason
		afterTask.Result = ""
		afterTask.State = models.Task_Completed
		afterTask.FirstCompletedAt = now
		afterTask.UpdatedAt = now

		events = append(events, models.NewTaskChangedEvent(task, &afterTask))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("failed-rows-affected", err)
		return events, uint64(invalidTasksCount), 0
	}

	return events, uint64(invalidTasksCount), rowsAffected
}

func (db *SQLDB) demoteKickableResolvingTasks(ctx context.Context, logger lager.Logger, kickTasksDuration time.Duration) ([]models.Event, uint64) {
	logger = logger.Session("demote-kickable-resolving-tasks")

	rows, err := db.all(ctx, logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND updated_at < ?", models.Task_Resolving, db.clock.Now().Add(-kickTasksDuration).UnixNano(),
	)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0
	}
	defer rows.Close()

	tasks, validTaskGuids, invalidTasksCount, err := db.fetchTasks(ctx, logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
	}

	wheres := []string{"state = ?", "updated_at < ?"}
	bindings := []interface{}{models.Task_Resolving, db.clock.Now().Add(-kickTasksDuration).UnixNano()}

	if len(validTaskGuids) == 0 {
		return nil, uint64(invalidTasksCount)
	}

	wheres = append(wheres, fmt.Sprintf("guid IN (%s)", helpers.QuestionMarks(len(validTaskGuids))))

	for _, guid := range validTaskGuids {
		bindings = append(bindings, guid)
	}

	_, err = db.update(ctx, logger, db.db, tasksTable,
		helpers.SQLAttributes{"state": models.Task_Completed},
		strings.Join(wheres, " AND "), bindings...,
	)
	if err != nil {
		logger.Error("failed-updating-tasks", err)
	}

	var events []models.Event
	for _, task := range tasks {
		afterTask := *task
		afterTask.State = models.Task_Completed
		events = append(events, models.NewTaskChangedEvent(task, &afterTask))
	}

	return events, uint64(invalidTasksCount)
}

func (db *SQLDB) deleteExpiredCompletedTasks(ctx context.Context, logger lager.Logger, expireCompletedTaskDuration time.Duration) ([]models.Event, int64) {
	logger = logger.Session("delete-expired-completed-tasks")
	wheres := "state = ? AND first_completed_at < ?"
	values := []interface{}{models.Task_Completed, db.clock.Now().Add(-expireCompletedTaskDuration).UnixNano()}

	rows, err := db.all(ctx, logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		wheres, values...,
	)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0
	}
	defer rows.Close()

	tasks, validTaskGuids, invalidTasksCount, err := db.fetchTasks(ctx, logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-tasks", err, lager.Data{"invalidTasksCound": int64(invalidTasksCount)})
	}

	if len(validTaskGuids) == 0 {
		return nil, int64(invalidTasksCount)
	}

	wheres += fmt.Sprintf(" AND guid IN (%s)", helpers.QuestionMarks(len(validTaskGuids)))

	for _, guid := range validTaskGuids {
		values = append(values, guid)
	}

	result, err := db.delete(ctx, logger, db.db, tasksTable, wheres, values...)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, int64(invalidTasksCount)
	}

	var events []models.Event
	for _, task := range tasks {
		events = append(events, models.NewTaskRemovedEvent(task))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("failed-rows-affected", err)
		return events, int64(invalidTasksCount)
	}
	rowsAffected += int64(invalidTasksCount)

	return events, rowsAffected
}

func (db *SQLDB) getKickableCompleteTasksForCompletion(ctx context.Context, logger lager.Logger, kickTasksDuration time.Duration) ([]*models.Task, uint64) {
	logger = logger.Session("get-kickable-complete-tasks-for-completion")

	rows, err := db.all(ctx, logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND updated_at < ?",
		models.Task_Completed, db.clock.Now().Add(-kickTasksDuration).UnixNano(),
	)

	if err != nil {
		logger.Error("failed-query", err)
		return []*models.Task{}, math.MaxUint64
	}

	defer rows.Close()

	tasksToComplete, _, failedFetches, err := db.fetchTasks(ctx, logger, rows, db.db, false)

	if err != nil {
		logger.Error("failed-fetching-some-tasks", err)
	}

	return tasksToComplete, uint64(failedFetches)
}

func (db *SQLDB) getTaskCountByState(ctx context.Context, logger lager.Logger) (pendingCount, runningCount, completedCount, resolvingCount int) {
	var query string
	switch db.flavor {
	case helpers.Postgres:
		query = `
			SELECT
				COUNT(*) FILTER (WHERE state = $1) AS pending_tasks,
				COUNT(*) FILTER (WHERE state = $2) AS running_tasks,
				COUNT(*) FILTER (WHERE state = $3) AS completed_tasks,
				COUNT(*) FILTER (WHERE state = $4) AS resolving_tasks
			FROM tasks
		`
	case helpers.MySQL:
		query = `
			SELECT
				COUNT(IF(state = ?, 1, NULL)) AS pending_tasks,
				COUNT(IF(state = ?, 1, NULL)) AS running_tasks,
				COUNT(IF(state = ?, 1, NULL)) AS completed_tasks,
				COUNT(IF(state = ?, 1, NULL)) AS resolving_tasks
			FROM tasks
		`
	default:
		// totally shouldn't happen
		panic("database flavor not implemented: " + db.flavor)
	}

	row := db.db.QueryRowContext(ctx, query, models.Task_Pending, models.Task_Running, models.Task_Completed, models.Task_Resolving)
	err := row.Scan(&pendingCount, &runningCount, &completedCount, &resolvingCount)
	if err != nil {
		logger.Error("failed-counting-tasks", err)
	}
	return
}
