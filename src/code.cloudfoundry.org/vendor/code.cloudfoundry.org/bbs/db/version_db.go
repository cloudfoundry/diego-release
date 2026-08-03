package db

import (
	"context"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . VersionDB
type VersionDB interface {
	Version(tx helpers.Tx, ctx context.Context, logger lager.Logger) (*models.Version, error)
	SetVersion(tx helpers.Tx, ctx context.Context, logger lager.Logger, version *models.Version) error
}
