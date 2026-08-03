package sqldb

import (
	"context"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

const configurationsTable = "configurations"

func (db *SQLDB) setConfigurationValue(tx helpers.Tx, ctx context.Context, logger lager.Logger, key, value string) error {
	_, err := db.upsert(
		ctx,
		logger,
		tx,
		configurationsTable,
		helpers.SQLAttributes{"value": value, "id": key},
		"id = ?", key,
	)
	if err != nil {
		logger.Error("failed-setting-config-value", err, lager.Data{"key": key})
		return err
	}

	return nil
}

func (db *SQLDB) getConfigurationValue(tx helpers.Tx, ctx context.Context, logger lager.Logger, key string) (string, error) {
	var value string

	err := db.one(ctx, logger, tx, "configurations",
		helpers.ColumnList{"value"}, helpers.NoLockRow,
		"id = ?", key,
	).Scan(&value)
	if err != nil {
		return "", err
	}

	return value, nil
}
