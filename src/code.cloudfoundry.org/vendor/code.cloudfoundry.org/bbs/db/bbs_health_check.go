package db

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . BBSHealthCheckDB

type BBSHealthCheckDB interface {
	PerformBBSHealthCheck(ctx context.Context, logger lager.Logger, t time.Time) error
}
