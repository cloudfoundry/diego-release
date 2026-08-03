package sqldb

import (
	"context"
	"database/sql"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) DesireTask(ctx context.Context, logger lager.Logger, taskDef *models.TaskDefinition, taskGuid, domain string) (*models.Task, error) {
	logger = logger.Session("db-desire-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	taskDefData, err := db.serializeModel(logger, taskDef)
	if err != nil {
		logger.Error("failed-serializing-task-definition", err)
		return nil, err
	}

	now := db.clock.Now().UnixNano()
	err = db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		_, err = db.insert(ctx, logger, tx, tasksTable,
			helpers.SQLAttributes{
				"guid":               taskGuid,
				"domain":             domain,
				"created_at":         now,
				"updated_at":         now,
				"first_completed_at": 0,
				"state":              models.Task_Pending,
				"task_definition":    taskDefData,
			},
		)

		return err
	})

	if err != nil {
		logger.Error("failed-inserting-task", err)
		return nil, err
	}

	return &models.Task{
		TaskDefinition:   taskDef,
		TaskGuid:         taskGuid,
		Domain:           domain,
		CreatedAt:        now,
		UpdatedAt:        now,
		FirstCompletedAt: 0,
		State:            models.Task_Pending,
	}, nil
}

func (db *SQLDB) Tasks(ctx context.Context, logger lager.Logger, filter models.TaskFilter) ([]*models.Task, error) {
	logger = logger.Session("db-tasks", lager.Data{"filter": filter})
	logger.Debug("starting")
	defer logger.Debug("complete")

	wheres := []string{}
	values := []interface{}{}

	if filter.Domain != "" {
		wheres = append(wheres, "domain = ?")
		values = append(values, filter.Domain)
	}

	if filter.CellID != "" {
		wheres = append(wheres, "cell_id = ?")
		values = append(values, filter.CellID)
	}

	results := []*models.Task{}

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		rows, err := db.all(ctx, logger, tx, tasksTable,
			taskColumns, helpers.NoLockRow,
			strings.Join(wheres, " AND "), values...,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}
		defer rows.Close()

		results, _, _, err = db.fetchTasks(ctx, logger, rows, tx, true)
		if err != nil {
			logger.Error("failed-fetch", err)
			return err
		}

		return nil
	})

	return results, err
}

func (db *SQLDB) TaskByGuid(ctx context.Context, logger lager.Logger, taskGuid string) (*models.Task, error) {
	logger = logger.Session("db-task-by-guid", lager.Data{"task_guid": taskGuid})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var task *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		row := db.one(ctx, logger, tx, tasksTable,
			taskColumns, helpers.NoLockRow,
			"guid = ?", taskGuid,
		)

		task, err = db.fetchTask(ctx, logger, row, tx)
		return err
	})

	return task, err
}

func (db *SQLDB) StartTask(ctx context.Context, logger lager.Logger, taskGuid, cellId string) (*models.Task, *models.Task, bool, error) {
	logger = logger.Session("db-start-task", lager.Data{"task_guid": taskGuid, "cell_id": cellId})
	logger.Info("starting")
	defer logger.Info("complete")

	var started bool
	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask
		if afterTask.State == models.Task_Running && afterTask.CellId == cellId {
			logger.Debug("task-already-running-on-cell")
			return nil
		}

		if err = afterTask.ValidateTransitionTo(models.Task_Running); err != nil {
			logger.Error("failed-to-transition-task-to-running", err)
			return err
		}

		now := db.clock.Now().UnixNano()
		_, err = db.update(ctx, logger, tx, tasksTable,
			helpers.SQLAttributes{
				"state":      models.Task_Running,
				"updated_at": now,
				"cell_id":    cellId,
			},
			"guid = ?", taskGuid,
		)
		if err != nil {
			return err
		}

		afterTask.State = models.Task_Running
		afterTask.UpdatedAt = now
		afterTask.CellId = cellId

		started = true
		return nil
	})

	return &beforeTask, afterTask, started, err
}

func (db *SQLDB) CancelTask(ctx context.Context, logger lager.Logger, taskGuid string) (*models.Task, *models.Task, string, error) {
	logger = logger.Session("db-cancel-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task
	var cellID string

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask
		cellID = afterTask.CellId

		if err = afterTask.ValidateTransitionTo(models.Task_Completed); err != nil {
			if afterTask.State != models.Task_Pending {
				logger.Error("failed-to-transition-task-to-completed", err)
				return err
			}
		}
		err = db.completeTask(ctx, logger, afterTask, true, "task was cancelled", "", tx)
		if err != nil {
			return err
		}

		return nil
	})

	return &beforeTask, afterTask, cellID, err
}

func (db *SQLDB) CompleteTask(ctx context.Context, logger lager.Logger, taskGuid, cellID string, failed bool, failureReason, taskResult string) (*models.Task, *models.Task, error) {
	logger = logger.Session("db-complete-task", lager.Data{"task_guid": taskGuid, "cell_id": cellID})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}
		beforeTask = *afterTask

		if afterTask.CellId != cellID && afterTask.State == models.Task_Running {
			logger.Error("failed-task-already-running-on-different-cell", err)
			return models.NewRunningOnDifferentCellError(cellID, afterTask.CellId)
		}

		if err = afterTask.ValidateTransitionTo(models.Task_Completed); err != nil {
			logger.Error("failed-to-transition-task-to-completed", err)
			return err
		}

		err = db.completeTask(ctx, logger, afterTask, failed, failureReason, taskResult, tx)
		if err != nil {
			return err
		}

		return nil
	})

	return &beforeTask, afterTask, err
}

func (db *SQLDB) FailTask(ctx context.Context, logger lager.Logger, taskGuid, failureReason string) (*models.Task, *models.Task, error) {
	logger = logger.Session("db-fail-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask

		if err = afterTask.ValidateTransitionTo(models.Task_Completed); err != nil {
			if afterTask.State != models.Task_Pending {
				logger.Error("failed-to-transition-task-to-completed", err)
				return err
			}
		}

		err = db.completeTask(ctx, logger, afterTask, true, failureReason, "", tx)
		if err != nil {
			return err
		}

		return nil
	})

	return &beforeTask, afterTask, err
}

func (db *SQLDB) RejectTask(ctx context.Context, logger lager.Logger, taskGuid, rejectionReason string) (*models.Task, *models.Task, error) {
	logger = logger.Session("db-reject-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")
	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		if afterTask.State != models.Task_Pending && afterTask.State != models.Task_Running {
			logger.Info("invalid-task-state", lager.Data{"task_state": afterTask.State})
			return models.ErrBadRequest
		}

		beforeTask = *afterTask

		now := db.clock.Now().UnixNano()

		afterTask.RejectionCount++
		afterTask.RejectionReason = truncateString(rejectionReason, 1024)
		afterTask.State = models.Task_Pending
		afterTask.UpdatedAt = now

		_, err = db.update(ctx, logger, tx, tasksTable,
			helpers.SQLAttributes{
				"rejection_count":  afterTask.RejectionCount,
				"rejection_reason": afterTask.RejectionReason,
				"updated_at":       afterTask.UpdatedAt,
				"state":            afterTask.State,
			},
			"guid = ?", taskGuid,
		)
		if err != nil {
			logger.Error("failed-updating-tasks", err)
			return err
		}

		return nil
	})

	return &beforeTask, afterTask, err
}

// The stager calls this when it wants to claim a completed task.  This ensures that only one
// stager ever attempts to handle a completed task
func (db *SQLDB) ResolvingTask(ctx context.Context, logger lager.Logger, taskGuid string) (*models.Task, *models.Task, error) {
	logger = logger.Session("db-resolving-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask

		if err = afterTask.ValidateTransitionTo(models.Task_Resolving); err != nil {
			logger.Error("invalid-state-transition", err)
			return err
		}

		now := db.clock.Now().UnixNano()
		_, err = db.update(ctx, logger, tx, tasksTable,
			helpers.SQLAttributes{
				"state":      models.Task_Resolving,
				"updated_at": now,
			},
			"guid = ?", taskGuid,
		)
		if err != nil {
			logger.Error("failed-updating-tasks", err)
			return err
		}

		afterTask.State = models.Task_Resolving
		afterTask.UpdatedAt = now

		return nil
	})

	return &beforeTask, afterTask, err
}

func (db *SQLDB) DeleteTask(ctx context.Context, logger lager.Logger, taskGuid string) (*models.Task, error) {
	logger = logger.Session("db-delete-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var task *models.Task

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		task, err = db.fetchTaskForUpdate(ctx, logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		if task.State != models.Task_Resolving {
			err = models.NewTaskTransitionError(task.State, models.Task_Resolving)
			logger.Error("invalid-state-transition", err)
			return err
		}

		_, err = db.delete(ctx, logger, tx, tasksTable, "guid = ?", taskGuid)
		if err != nil {
			logger.Error("failed-deleting-task", err)
			return err
		}

		return nil
	})
	return task, err
}

func (db *SQLDB) completeTask(ctx context.Context, logger lager.Logger, task *models.Task, failed bool, failureReason, result string, tx helpers.Tx) error {
	now := db.clock.Now().UnixNano()

	task.State = models.Task_Completed
	task.UpdatedAt = now
	task.FirstCompletedAt = now
	task.Failed = failed
	task.FailureReason = truncateString(failureReason, 1024)
	task.Result = result
	task.CellId = ""

	_, err := db.update(ctx, logger, tx, tasksTable,
		helpers.SQLAttributes{
			"failed":             task.Failed,
			"failure_reason":     task.FailureReason,
			"result":             task.Result,
			"state":              task.State,
			"first_completed_at": task.FirstCompletedAt,
			"updated_at":         task.UpdatedAt,
			"cell_id":            "",
		},
		"guid = ?", task.TaskGuid,
	)
	if err != nil {
		logger.Error("failed-updating-tasks", err)
		return err
	}

	return nil
}

func (db *SQLDB) fetchTaskForUpdate(ctx context.Context, logger lager.Logger, taskGuid string, queryable helpers.Queryable) (*models.Task, error) {
	row := db.one(ctx, logger, queryable, tasksTable,
		taskColumns, helpers.LockRow,
		"guid = ?", taskGuid,
	)
	return db.fetchTask(ctx, logger, row, queryable)
}

func (db *SQLDB) fetchTasks(ctx context.Context, logger lager.Logger, rows *sql.Rows, queryable helpers.Queryable, abortOnError bool) ([]*models.Task, []string, int, error) {
	tasks := []*models.Task{}
	invalidGuids := []string{}
	validGuids := []string{}
	var err error
	for rows.Next() {
		var task *models.Task
		var guid string

		task, guid, err = db.fetchTaskInternal(logger, rows)
		if err != nil {
			if err == models.ErrDeserialize {
				invalidGuids = append(invalidGuids, guid)
			}

			if abortOnError {
				break
			}

			continue
		}

		tasks = append(tasks, task)
		validGuids = append(validGuids, task.TaskGuid)
	}

	if err == nil {
		err = rows.Err()
	}

	closeErr := rows.Close()
	if closeErr != nil {
		logger.Debug("failed-to-close-row", lager.Data{"error": closeErr})
	}

	if len(invalidGuids) > 0 {
		deleteErr := db.deleteInvalidTasks(ctx, logger, queryable, invalidGuids...)
		if deleteErr != nil {
			logger.Error("failed-to-delete-invalid-task", err, lager.Data{"guids": invalidGuids})
		}
	}

	return tasks, validGuids, len(invalidGuids), err
}

func (db *SQLDB) fetchTask(ctx context.Context, logger lager.Logger, scanner helpers.RowScanner, queryable helpers.Queryable) (*models.Task, error) {
	task, guid, err := db.fetchTaskInternal(logger, scanner)
	if err == models.ErrDeserialize {
		deleteErr := db.deleteInvalidTasks(ctx, logger, queryable, guid)
		if deleteErr != nil {
			logger.Error("failed-to-delete-invalid-task", err, lager.Data{"guid": guid})
		}
	}
	return task, err
}

func (db *SQLDB) fetchTaskInternal(logger lager.Logger, scanner helpers.RowScanner) (*models.Task, string, error) {
	var guid, domain, cellID, failureReason, rejectionReason string
	var result sql.NullString
	var createdAt, updatedAt, firstCompletedAt int64
	var state, rejectionCount int32
	var failed bool
	var taskDefData []byte

	err := scanner.Scan(
		&guid,
		&domain,
		&updatedAt,
		&createdAt,
		&firstCompletedAt,
		&state,
		&cellID,
		&result,
		&failed,
		&failureReason,
		&taskDefData,
		&rejectionCount,
		&rejectionReason,
	)

	if err == sql.ErrNoRows {
		return nil, "", err
	}

	if err != nil {
		logger.Error("failed-scanning-row", err)
		return nil, "", err
	}

	var taskDef models.TaskDefinition
	err = db.deserializeModel(logger, taskDefData, &taskDef)
	if err != nil {
		return nil, guid, models.ErrDeserialize
	}

	task := &models.Task{
		TaskGuid:         guid,
		Domain:           domain,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		FirstCompletedAt: firstCompletedAt,
		State:            models.Task_State(state),
		CellId:           cellID,
		Result:           result.String,
		Failed:           failed,
		FailureReason:    failureReason,
		TaskDefinition:   &taskDef,
		RejectionCount:   rejectionCount,
		RejectionReason:  rejectionReason,
	}
	return task, guid, nil
}

func (db *SQLDB) deleteInvalidTasks(ctx context.Context, logger lager.Logger, queryable helpers.Queryable, guids ...string) error {
	for _, guid := range guids {
		logger.Info("deleting-invalid-task-from-db", lager.Data{"guid": guid})
		_, err := db.delete(ctx, logger, queryable, tasksTable, "guid = ?", guid)
		if err != nil {
			logger.Error("failed-deleting-task", err)
		}
	}
	return nil
}
