package db

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) CreateLockTable(ctx context.Context, logger lager.Logger) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS locks (
			path VARCHAR(255) PRIMARY KEY,
			owner VARCHAR(255),
			value VARCHAR(4096),
			type VARCHAR(255) DEFAULT '',
			modified_index BIGINT DEFAULT 0,
			modified_id varchar(255) DEFAULT '',
			ttl BIGINT DEFAULT 0
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) CreateHealthCheckTable(ctx context.Context, logger lager.Logger) error {
	logger = logger.Session("create-health-check-table")
	logger.Info("starting")
	defer logger.Info("completed")

	var createTableSQL string
	switch db.flavor {
	case helpers.MySQL:
		createTableSQL = "CREATE TABLE IF NOT EXISTS locket_health_check (id int NOT NULL AUTO_INCREMENT, PRIMARY KEY (id), time bigint NOT NULL)"
	case helpers.Postgres:
		createTableSQL = "CREATE TABLE IF NOT EXISTS locket_health_check (id SERIAL PRIMARY KEY, time bigint NOT NULL)"
	default:
		return fmt.Errorf("unsupported database flavor: %s", db.flavor)
	}

	logger.Info("creating-table")
	_, err := db.ExecContext(ctx, helpers.RebindForFlavor(createTableSQL, db.flavor))
	if err != nil {
		logger.Error("failed-creating-table", err)
		return fmt.Errorf("failed to create health check table: %w", err)
	}

	return nil
}
