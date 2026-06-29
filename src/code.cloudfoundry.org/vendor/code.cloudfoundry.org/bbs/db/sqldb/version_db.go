package sqldb

import (
	"context"
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

const VersionID = "version"

func (db *SQLDB) SetVersion(tx helpers.Tx, ctx context.Context, logger lager.Logger, version *models.Version) error {
	logger = logger.Session("db-set-version", lager.Data{"version": version})
	logger.Debug("starting")
	defer logger.Debug("complete")

	versionJSON, err := json.Marshal(version)
	if err != nil {
		logger.Error("failed-marshalling-version", err)
		return err
	}
	err = db.helper.RetryOnDeadlock(logger, func() error {
		return db.setConfigurationValue(tx, ctx, logger, VersionID, string(versionJSON))
	})
	if err != nil {
		return db.convertSQLError(err)
	}
	return nil
}

func (db *SQLDB) Version(tx helpers.Tx, ctx context.Context, logger lager.Logger) (*models.Version, error) {
	logger = logger.Session("db-version")
	logger.Debug("starting")
	defer logger.Debug("complete")

	var versionJSON string

	err := db.helper.RetryOnDeadlock(logger, func() error {
		var err error
		versionJSON, err = db.getConfigurationValue(tx, ctx, logger, VersionID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, db.convertSQLError(err)
	}
	var version models.Version
	err = json.Unmarshal([]byte(versionJSON), &version)
	if err != nil {
		logger.Error("failed-to-deserialize-version", err)
		return nil, models.ErrDeserialize
	}

	return &version, nil
}
