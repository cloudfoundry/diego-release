package db

import (
	"context"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . SuspectDB

type SuspectDB interface {
	RemoveSuspectActualLRP(context.Context, lager.Logger, *models.ActualLRPKey) (*models.ActualLRP, error)
	PromoteSuspectActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32) (*models.ActualLRP, *models.ActualLRP, *models.ActualLRP, error)
}
