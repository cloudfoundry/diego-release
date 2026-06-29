package db

import (
	"context"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . DesiredLRPDB

type DesiredLRPDB interface {
	DesiredLRPs(ctx context.Context, logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error)
	DesiredLRPByProcessGuid(ctx context.Context, logger lager.Logger, processGuid string) (*models.DesiredLRP, error)

	DesiredLRPSchedulingInfos(ctx context.Context, logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error)
	DesiredLRPSchedulingInfoByProcessGuid(ctx context.Context, logger lager.Logger, processGuid string) (*models.DesiredLRPSchedulingInfo, error)
	DesiredLRPUpdateStrategyByProcessGuid(ctx context.Context, logger lager.Logger, processGuid string) (models.DesiredLRP_UpdateStrategy, int32, error)

	DesiredLRPRoutingInfos(ctx context.Context, logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error)

	DesireLRP(ctx context.Context, logger lager.Logger, desiredLRP *models.DesiredLRP) error
	UpdateDesiredLRP(ctx context.Context, logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate) (beforeDesiredLRP *models.DesiredLRP, err error)
	RemoveDesiredLRP(ctx context.Context, logger lager.Logger, processGuid string) error
}
