package db

import (
	"context"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . ActualLRPDB

type ActualLRPDB interface {
	ActualLRPs(ctx context.Context, logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRP, error)
	ActualLRPsByProcessGuids(ctx context.Context, logger lager.Logger, filter models.ActualLRPsByProcessGuidsFilter) ([]*models.ActualLRP, error)
	CreateUnclaimedActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey) (after *models.ActualLRP, err error)
	UnclaimActualLRP(ctx context.Context, logger lager.Logger, isStale bool, key *models.ActualLRPKey) (before *models.ActualLRP, after *models.ActualLRP, err error)
	UnclaimActualLRPIfAllRunning(ctx context.Context, logger lager.Logger, isStale bool, key *models.ActualLRPKey, desiredInstances int32) (before *models.ActualLRP, after *models.ActualLRP, err error)

	ClaimActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) (before *models.ActualLRP, after *models.ActualLRP, err error)
	StartActualLRP(ctx context.Context,
		logger lager.Logger,
		key *models.ActualLRPKey,
		instanceKey *models.ActualLRPInstanceKey,
		netInfo *models.ActualLRPNetInfo,
		internalRoutes []*models.ActualLRPInternalRoute,
		metricTags map[string]string,
		routable bool,
		availabilityZone string,
		isCurrentlyRunning bool,
	) (before *models.ActualLRP, after *models.ActualLRP, err error)
	CrashActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, crashReason string) (before *models.ActualLRP, after *models.ActualLRP, shouldRestart bool, err error)
	FailActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, placementError string) (before *models.ActualLRP, after *models.ActualLRP, err error)
	RemoveActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) error

	ChangeActualLRPPresence(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, from, to models.ActualLRP_Presence) (before *models.ActualLRP, after *models.ActualLRP, err error)

	CountActualLRPsByState(ctx context.Context, logger lager.Logger) (int, int, int, int, int)
	CountDesiredInstances(ctx context.Context, logger lager.Logger) int
}
