package sqldb

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) PerformBBSHealthCheck(ctx context.Context, logger lager.Logger, t time.Time) error {
	logger = logger.Session("db")
	logger.Debug("starting")
	defer logger.Debug("done")

	logger.Debug("upserting-time", lager.Data{"time": t})
	_, err := db.upsert(ctx, logger, db.db, "bbs_health_check", helpers.SQLAttributes{"id": 1, "time": t.UnixNano()}, "id = ?", 1)
	if err != nil {
		return fmt.Errorf("failed upserting health check time: %s", err)
	}

	logger.Debug("retrieving-upserted-time")
	scanner := db.db.QueryRowContext(ctx, helpers.RebindForFlavor("SELECT time from bbs_health_check where id = ?", db.flavor), 1)
	var insertedTime int64
	err = scanner.Scan(&insertedTime)
	if err != nil {
		return fmt.Errorf("failed querying for health check time: %s", err)
	}
	logger.Debug("upserted-and-retrieved-time")
	return nil
}
