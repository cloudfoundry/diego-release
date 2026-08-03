package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

const (
	tasksTable       = "tasks"
	desiredLRPsTable = "desired_lrps"
	actualLRPsTable  = "actual_lrps"
	domainsTable     = "domains"
)

var (
	routingInfoColumns = helpers.ColumnList{
		desiredLRPsTable + ".process_guid",
		desiredLRPsTable + ".domain",
		desiredLRPsTable + ".log_guid",
		desiredLRPsTable + ".instances",
		desiredLRPsTable + ".routes",
		desiredLRPsTable + ".modification_tag_epoch",
		desiredLRPsTable + ".modification_tag_index",
		desiredLRPsTable + ".metric_tags",
	}

	schedulingInfoColumns = helpers.ColumnList{
		desiredLRPsTable + ".process_guid",
		desiredLRPsTable + ".domain",
		desiredLRPsTable + ".log_guid",
		desiredLRPsTable + ".annotation",
		desiredLRPsTable + ".instances",
		desiredLRPsTable + ".memory_mb",
		desiredLRPsTable + ".disk_mb",
		desiredLRPsTable + ".max_pids",
		desiredLRPsTable + ".rootfs",
		desiredLRPsTable + ".routes",
		desiredLRPsTable + ".volume_placement",
		desiredLRPsTable + ".modification_tag_epoch",
		desiredLRPsTable + ".modification_tag_index",
		desiredLRPsTable + ".placement_tags",
	}

	desiredLRPColumns = append(schedulingInfoColumns,
		desiredLRPsTable+".run_info",
		desiredLRPsTable+".metric_tags",
		desiredLRPsTable+".update_strategy",
	)

	taskColumns = helpers.ColumnList{
		tasksTable + ".guid",
		tasksTable + ".domain",
		tasksTable + ".updated_at",
		tasksTable + ".created_at",
		tasksTable + ".first_completed_at",
		tasksTable + ".state",
		tasksTable + ".cell_id",
		tasksTable + ".result",
		tasksTable + ".failed",
		tasksTable + ".failure_reason",
		tasksTable + ".task_definition",
		tasksTable + ".rejection_count",
		tasksTable + ".rejection_reason",
	}

	actualLRPColumns = helpers.ColumnList{
		actualLRPsTable + ".process_guid",
		actualLRPsTable + ".instance_index",
		actualLRPsTable + ".presence",
		actualLRPsTable + ".domain",
		actualLRPsTable + ".state",
		actualLRPsTable + ".instance_guid",
		actualLRPsTable + ".cell_id",
		actualLRPsTable + ".placement_error",
		actualLRPsTable + ".since",
		actualLRPsTable + ".net_info",
		actualLRPsTable + ".internal_routes",
		actualLRPsTable + ".metric_tags",
		actualLRPsTable + ".routable",
		actualLRPsTable + ".availability_zone",
		actualLRPsTable + ".modification_tag_epoch",
		actualLRPsTable + ".modification_tag_index",
		actualLRPsTable + ".crash_count",
		actualLRPsTable + ".crash_reason",
	}

	actualLRPIDColumns = helpers.ColumnList{
		actualLRPsTable + ".process_guid",
		actualLRPsTable + ".instance_index",
		actualLRPsTable + ".domain",
		actualLRPsTable + ".instance_guid",
		actualLRPsTable + ".cell_id",
	}

	domainColumns = helpers.ColumnList{
		domainsTable + ".domain",
		domainsTable + ".expire_time",
	}
)

func (db *SQLDB) CreateConfigurationsTable(ctx context.Context, logger lager.Logger) error {
	_, err := db.db.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS configurations(
				id VARCHAR(255) PRIMARY KEY,
				value VARCHAR(255)
			)
		`)
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) selectLRPInstanceCounts(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	var query string
	columns := schedulingInfoColumns
	columns = append(columns, "COUNT(actual_lrps.instance_index) AS actual_instances")

	switch db.flavor {
	case helpers.Postgres:
		columns = append(columns, "STRING_AGG(actual_lrps.instance_index::text, ',') AS existing_indices")
	case helpers.MySQL:
		columns = append(columns, "GROUP_CONCAT(actual_lrps.instance_index) AS existing_indices")
	default:
		// totally shouldn't happen
		panic("database flavor not implemented: " + db.flavor)
	}

	query = fmt.Sprintf(`
		SELECT %s
			FROM desired_lrps
			LEFT OUTER JOIN actual_lrps ON desired_lrps.process_guid = actual_lrps.process_guid AND actual_lrps.presence = %d
			GROUP BY desired_lrps.process_guid
			HAVING COUNT(actual_lrps.instance_index) <> desired_lrps.instances
		`,
		strings.Join(columns, ", "), models.ActualLRP_Ordinary,
	)

	return q.QueryContext(ctx, query)
}

func (db *SQLDB) selectOrphanedActualLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := fmt.Sprintf(`
    SELECT actual_lrps.process_guid, actual_lrps.instance_index, actual_lrps.domain
      FROM actual_lrps
      JOIN domains ON actual_lrps.domain = domains.domain
      LEFT JOIN desired_lrps ON actual_lrps.process_guid = desired_lrps.process_guid
      WHERE actual_lrps.presence = %d AND desired_lrps.process_guid IS NULL
		`, models.ActualLRP_Ordinary)

	return q.QueryContext(ctx, query)
}

func (db *SQLDB) selectOrphanedSuspectActualLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := fmt.Sprintf(`
    SELECT actual_lrps.process_guid, actual_lrps.instance_index, actual_lrps.domain
      FROM actual_lrps
      JOIN domains ON actual_lrps.domain = domains.domain
      LEFT JOIN desired_lrps ON actual_lrps.process_guid = desired_lrps.process_guid
      WHERE actual_lrps.presence = %d AND desired_lrps.process_guid IS NULL
		`, models.ActualLRP_Suspect)

	return q.QueryContext(ctx, query)
}

func (db *SQLDB) selectSuspectRunningActualLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := db.helper.Rebind(`SELECT process_guid, instance_index, domain
			FROM actual_lrps
			WHERE actual_lrps.presence = ? AND actual_lrps.state = ?`)

	return q.QueryContext(ctx, query, models.ActualLRP_Suspect, models.ActualLRPStateRunning)
}

func (db *SQLDB) selectSuspectClaimedActualLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := db.helper.Rebind(`SELECT process_guid, instance_index, domain
			FROM actual_lrps
			WHERE actual_lrps.presence = ? AND actual_lrps.state = ?`)

	return q.QueryContext(ctx, query, models.ActualLRP_Suspect, models.ActualLRPStateClaimed)
}

func (db *SQLDB) selectExtraSuspectActualLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := db.helper.Rebind(`SELECT process_guid, instance_index, domain
      FROM actual_lrps
      WHERE actual_lrps.presence IN (?, ?) AND actual_lrps.state = ?
			GROUP BY process_guid, instance_index, domain
			HAVING count(*) >= 2`)
	return q.QueryContext(ctx, query, models.ActualLRP_Ordinary, models.ActualLRP_Suspect, models.ActualLRPStateRunning)
}

func (db *SQLDB) selectSuspectLRPsWithExistingCells(ctx context.Context, logger lager.Logger, q helpers.Queryable, cellSet models.CellSet) (*sql.Rows, error) {
	wheres := []string{fmt.Sprintf("actual_lrps.presence = %d", models.ActualLRP_Suspect)}
	bindings := make([]interface{}, 0, len(cellSet))

	if len(cellSet) > 0 {
		wheres = append(wheres, fmt.Sprintf("actual_lrps.cell_id IN (%s)", helpers.QuestionMarks(len(cellSet))))
		for cellID := range cellSet {
			bindings = append(bindings, cellID)
		}
	}

	query := fmt.Sprintf(`
		SELECT process_guid, instance_index, domain
			FROM actual_lrps
			WHERE %s
		`,
		strings.Join(wheres, " AND "),
	)

	return q.QueryContext(ctx, db.helper.Rebind(query), bindings...)
}

func (db *SQLDB) selectLRPsWithMissingCells(ctx context.Context, logger lager.Logger, q helpers.Queryable, cellSet models.CellSet) (*sql.Rows, error) {
	wheres := []string{
		"(actual_lrps.state = ? OR actual_lrps.state = ?)",
	}

	bindings := []interface{}{}

	bindings = append(bindings, models.ActualLRPStateRunning, models.ActualLRPStateClaimed)

	if len(cellSet) > 0 {
		wheres = append(wheres, fmt.Sprintf("actual_lrps.cell_id NOT IN (%s)", helpers.QuestionMarks(len(cellSet))))
		wheres = append(wheres, "actual_lrps.cell_id <> ''")
		for cellID := range cellSet {
			bindings = append(bindings, cellID)
		}
	}

	query := fmt.Sprintf(`
		SELECT %s
			FROM desired_lrps
			JOIN actual_lrps ON desired_lrps.process_guid = actual_lrps.process_guid
			WHERE %s
		`,
		strings.Join(append(schedulingInfoColumns, "actual_lrps.instance_index", "actual_lrps.cell_id", "actual_lrps.presence"), ", "),
		strings.Join(wheres, " AND "),
	)

	return q.QueryContext(ctx, db.helper.Rebind(query), bindings...)
}

func (db *SQLDB) selectCrashedLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := fmt.Sprintf(`
		SELECT %s
			FROM desired_lrps
			JOIN actual_lrps ON desired_lrps.process_guid = actual_lrps.process_guid
			WHERE actual_lrps.state = ? AND actual_lrps.presence = ?
		`,
		strings.Join(
			append(schedulingInfoColumns, "actual_lrps.instance_index", "actual_lrps.since", "actual_lrps.crash_count"),
			", ",
		),
	)

	return q.QueryContext(ctx, db.helper.Rebind(query), models.ActualLRPStateCrashed, models.ActualLRP_Ordinary)
}

func (db *SQLDB) selectStaleUnclaimedLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable, now time.Time) (*sql.Rows, error) {
	query := fmt.Sprintf(`
		SELECT %s
			FROM desired_lrps
			JOIN actual_lrps ON desired_lrps.process_guid = actual_lrps.process_guid
			WHERE actual_lrps.state = ? AND actual_lrps.since < ? AND actual_lrps.presence = ?
		`,
		strings.Join(append(schedulingInfoColumns, "actual_lrps.instance_index"), ", "),
	)

	return q.QueryContext(ctx,
		db.helper.Rebind(query),
		models.ActualLRPStateUnclaimed,
		now.Add(-models.StaleUnclaimedActualLRPDuration).UnixNano(),
		models.ActualLRP_Ordinary,
	)
}

func (db *SQLDB) selectLRPsWithRoutes(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := fmt.Sprintf(`
		SELECT %s
			FROM desired_lrps
			JOIN actual_lrps ON desired_lrps.process_guid = actual_lrps.process_guid
			WHERE actual_lrps.state = ? AND actual_lrps.presence = ?
		`,
		strings.Join(
			append(actualLRPIDColumns, actualLRPsTable+".internal_routes", desiredLRPsTable+".routes"),
			", ",
		),
	)

	return q.QueryContext(ctx, db.helper.Rebind(query), models.ActualLRPStateRunning, models.ActualLRP_Ordinary)
}

func (db *SQLDB) selectLRPsWithMetricTags(ctx context.Context, logger lager.Logger, q helpers.Queryable) (*sql.Rows, error) {
	query := fmt.Sprintf(`
		SELECT %s
			FROM desired_lrps
			JOIN actual_lrps ON desired_lrps.process_guid = actual_lrps.process_guid
			WHERE actual_lrps.state = ? AND actual_lrps.presence = ?
		`,
		strings.Join(
			append(actualLRPIDColumns, actualLRPsTable+".metric_tags", desiredLRPsTable+".metric_tags"),
			", ",
		),
	)

	return q.QueryContext(ctx, db.helper.Rebind(query), models.ActualLRPStateRunning, models.ActualLRP_Ordinary)
}

func (db *SQLDB) countActualLRPsForProcessGuidAndState(ctx context.Context, logger lager.Logger, processGuid string, state string, presence models.ActualLRP_Presence, q helpers.Queryable) (int, error) {
	query := `
		SELECT COUNT(*) AS actual_instances
			FROM actual_lrps
			WHERE actual_lrps.process_guid = ? AND actual_lrps.state = ? AND actual_lrps.presence = ?
	`

	var actualInstances int
	row := q.QueryRowContext(ctx, db.helper.Rebind(query), processGuid, state, presence)
	err := row.Scan(&actualInstances)
	if err != nil {
		logger.Error("failed-actual-lrp-for-process-guid-query", err)
		return 0, err
	}
	return actualInstances, nil
}

func (db *SQLDB) CountDesiredInstances(ctx context.Context, logger lager.Logger) int {
	query := `
		SELECT COALESCE(SUM(desired_lrps.instances), 0) AS desired_instances
			FROM desired_lrps
	`

	var desiredInstances int
	row := db.db.QueryRowContext(ctx, db.helper.Rebind(query))
	err := row.Scan(&desiredInstances)
	if err != nil {
		logger.Error("failed-desired-instances-query", err)
	}
	return desiredInstances
}

func (db *SQLDB) CountActualLRPsByState(ctx context.Context, logger lager.Logger) (claimedCount, unclaimedCount, runningCount, crashedCount, crashingDesiredCount int) {
	var query string
	switch db.flavor {
	case helpers.Postgres:
		query = `
			SELECT
				COUNT(*) FILTER (WHERE actual_lrps.state = $1) AS claimed_instances,
				COUNT(*) FILTER (WHERE actual_lrps.state = $2) AS unclaimed_instances,
				COUNT(*) FILTER (WHERE actual_lrps.state = $3) AS running_instances,
				COUNT(*) FILTER (WHERE actual_lrps.state = $4) AS crashed_instances,
				COUNT(DISTINCT process_guid) FILTER (WHERE actual_lrps.state = $5) AS crashing_desireds
			FROM actual_lrps
			WHERE presence = $6
		`
	case helpers.MySQL:
		query = `
			SELECT
				COUNT(IF(actual_lrps.state = ?, 1, NULL)) AS claimed_instances,
				COUNT(IF(actual_lrps.state = ?, 1, NULL)) AS unclaimed_instances,
				COUNT(IF(actual_lrps.state = ?, 1, NULL)) AS running_instances,
				COUNT(IF(actual_lrps.state = ?, 1, NULL)) AS crashed_instances,
				COUNT(DISTINCT IF(state = ?, process_guid, NULL)) AS crashing_desireds
			FROM actual_lrps
			WHERE presence = ?
		`
	default:
		// totally shouldn't happen
		panic("database flavor not implemented: " + db.flavor)
	}

	row := db.db.QueryRowContext(ctx, query, models.ActualLRPStateClaimed, models.ActualLRPStateUnclaimed, models.ActualLRPStateRunning, models.ActualLRPStateCrashed, models.ActualLRPStateCrashed, models.ActualLRP_Ordinary)
	err := row.Scan(&claimedCount, &unclaimedCount, &runningCount, &crashedCount, &crashingDesiredCount)
	if err != nil {
		logger.Error("failed-counting-actual-lrps", err)
	}
	return
}

func (db *SQLDB) one(ctx context.Context, logger lager.Logger, q helpers.Queryable, table string,
	columns helpers.ColumnList, lockRow helpers.RowLock,
	wheres string, whereBindings ...interface{},
) helpers.RowScanner {
	return db.helper.One(ctx, logger, q, table, columns, lockRow, wheres, whereBindings...)
}

func (db *SQLDB) all(ctx context.Context, logger lager.Logger, q helpers.Queryable, table string,
	columns helpers.ColumnList, lockRow helpers.RowLock,
	wheres string, whereBindings ...interface{},
) (*sql.Rows, error) {
	return db.helper.All(ctx, logger, q, table, columns, lockRow, wheres, whereBindings...)
}

func (db *SQLDB) upsert(ctx context.Context, logger lager.Logger, q helpers.Queryable, table string, attributes helpers.SQLAttributes, wheres string, whereBindings ...interface{}) (bool, error) {
	return db.helper.Upsert(ctx, logger, q, table, attributes, wheres, whereBindings...)
}

func (db *SQLDB) insert(ctx context.Context, logger lager.Logger, q helpers.Queryable, table string, attributes helpers.SQLAttributes) (sql.Result, error) {
	return db.helper.Insert(ctx, logger, q, table, attributes)
}

func (db *SQLDB) update(ctx context.Context, logger lager.Logger, q helpers.Queryable, table string, updates helpers.SQLAttributes, wheres string, whereBindings ...interface{}) (sql.Result, error) {
	return db.helper.Update(ctx, logger, q, table, updates, wheres, whereBindings...)
}

func (db *SQLDB) delete(ctx context.Context, logger lager.Logger, q helpers.Queryable, table string, wheres string, whereBindings ...interface{}) (sql.Result, error) {
	return db.helper.Delete(ctx, logger, q, table, wheres, whereBindings...)
}
