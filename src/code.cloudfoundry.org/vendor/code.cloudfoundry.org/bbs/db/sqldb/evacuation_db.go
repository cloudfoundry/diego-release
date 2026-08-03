package sqldb

import (
	"context"
	"reflect"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) EvacuateActualLRP(
	ctx context.Context,
	logger lager.Logger,
	lrpKey *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
	netInfo *models.ActualLRPNetInfo,
	internalRoutes []*models.ActualLRPInternalRoute,
	metricTags map[string]string,
	routable bool,
	availabilityZone string,
) (*models.ActualLRP, error) {
	logger = logger.Session("db-evacuate-actual-lrp", lager.Data{"lrp_key": lrpKey, "instance_key": instanceKey, "net_info": netInfo, "routable": routable})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var actualLRP *models.ActualLRP

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		processGuid := lrpKey.ProcessGuid
		index := lrpKey.Index

		actualLRP, err = db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Evacuating, tx)
		if err == models.ErrResourceNotFound {
			logger.Debug("creating-evacuating-lrp")
			actualLRP, err = db.createEvacuatingActualLRP(ctx, logger, lrpKey, instanceKey, netInfo, internalRoutes, metricTags, routable, availabilityZone, tx)
			return err
		}

		if err != nil {
			logger.Error("failed-locking-lrp", err)
			return err
		}

		if actualLRP.ActualLRPKey.Equal(lrpKey) &&
			actualLRP.ActualLRPInstanceKey.Equal(instanceKey) &&
			reflect.DeepEqual(actualLRP.ActualLRPNetInfo, *netInfo) {
			logger.Debug("evacuating-lrp-already-exists")
			return models.ErrResourceExists
		}

		now := db.clock.Now().UnixNano()
		actualLRP.ModificationTag.Increment()
		actualLRP.ActualLRPKey = *lrpKey
		actualLRP.ActualLRPInstanceKey = *instanceKey
		actualLRP.Since = now
		actualLRP.ActualLRPNetInfo = *netInfo
		actualLRP.ActualLrpInternalRoutes = internalRoutes
		actualLRP.MetricTags = metricTags
		actualLRP.AvailabilityZone = availabilityZone
		actualLRP.Presence = models.ActualLRP_Evacuating

		netInfoData, err := db.serializeModel(logger, netInfo)
		if err != nil {
			logger.Error("failed-serializing-net-info", err)
			return err
		}

		internalRoutesData, err := db.encodeInternalRouteData(logger, internalRoutes)
		if err != nil {
			logger.Error("failed-to-serialize-internalroutes", err)
			return err
		}

		metricTagsData, err := db.encodeMetricTagsData(logger, metricTags)
		if err != nil {
			logger.Error("failed-to-serialize-metric-tags", err)
			return err
		}

		_, err = db.update(ctx, logger, tx, "actual_lrps",
			helpers.SQLAttributes{
				"domain":                 actualLRP.Domain,
				"instance_guid":          actualLRP.InstanceGuid,
				"cell_id":                actualLRP.CellId,
				"net_info":               netInfoData,
				"internal_routes":        internalRoutesData,
				"metric_tags":            metricTagsData,
				"state":                  actualLRP.State,
				"since":                  actualLRP.Since,
				"modification_tag_index": actualLRP.ModificationTag.Index,
			},
			"process_guid = ? AND instance_index = ? AND presence = ?",
			actualLRP.ProcessGuid, actualLRP.Index, models.ActualLRP_Evacuating,
		)
		if err != nil {
			logger.Error("failed-update-evacuating-lrp", err)
			return err
		}

		return nil
	})

	return actualLRP, err
}

func (db *SQLDB) RemoveEvacuatingActualLRP(ctx context.Context, logger lager.Logger, lrpKey *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) error {
	logger = logger.Session("db-remove-evacuating-actual-lrp", lager.Data{"lrp_key": lrpKey, "instance_key": instanceKey})
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		processGuid := lrpKey.ProcessGuid
		index := lrpKey.Index

		lrp, err := db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Evacuating, tx)
		if err == models.ErrResourceNotFound {
			logger.Debug("evacuating-lrp-does-not-exist")
			return nil
		}

		if err != nil {
			logger.Error("failed-fetching-actual-lrp", err)
			return err
		}

		if !lrp.ActualLRPInstanceKey.Equal(instanceKey) {
			logger.Debug("actual-lrp-instance-key-mismatch", lager.Data{"instance_key_param": instanceKey, "instance_key_from_db": lrp.ActualLRPInstanceKey})
			return models.ErrActualLRPCannotBeRemoved
		}

		_, err = db.delete(ctx, logger, tx, "actual_lrps",
			"process_guid = ? AND instance_index = ? AND presence = ?",
			processGuid, index, models.ActualLRP_Evacuating,
		)
		if err != nil {
			logger.Error("failed-delete", err)
			return models.ErrActualLRPCannotBeRemoved
		}

		return nil
	})
}

func (db *SQLDB) createEvacuatingActualLRP(
	ctx context.Context,
	logger lager.Logger,
	lrpKey *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
	netInfo *models.ActualLRPNetInfo,
	internalRoutes []*models.ActualLRPInternalRoute,
	metricTags map[string]string,
	routable bool,
	availabilityZone string,
	tx helpers.Tx,
) (*models.ActualLRP, error) {
	netInfoData, err := db.serializeModel(logger, netInfo)
	if err != nil {
		logger.Error("failed-serializing-net-info", err)
		return nil, err
	}

	internalRoutesData, err := db.encodeInternalRouteData(logger, internalRoutes)
	if err != nil {
		logger.Error("failed-to-serialize-internalroutes", err)
		return nil, err
	}

	metricTagsData, err := db.encodeMetricTagsData(logger, metricTags)
	if err != nil {
		logger.Error("failed-to-serialize-metric-tags", err)
		return nil, err
	}

	now := db.clock.Now()
	guid, err := db.guidProvider.NextGUID()
	if err != nil {
		return nil, models.ErrGUIDGeneration
	}

	actualLRP := &models.ActualLRP{
		ActualLRPKey:            *lrpKey,
		ActualLRPInstanceKey:    *instanceKey,
		ActualLRPNetInfo:        *netInfo,
		ActualLrpInternalRoutes: internalRoutes,
		MetricTags:              metricTags,
		AvailabilityZone:        availabilityZone,
		State:                   models.ActualLRPStateRunning,
		Since:                   now.UnixNano(),
		ModificationTag:         models.ModificationTag{Epoch: guid, Index: 0},
		Presence:                models.ActualLRP_Evacuating,
	}
	actualLRP.SetRoutable(routable)

	sqlAttributes := helpers.SQLAttributes{
		"process_guid":           actualLRP.ProcessGuid,
		"instance_index":         actualLRP.Index,
		"presence":               models.ActualLRP_Evacuating,
		"domain":                 actualLRP.Domain,
		"instance_guid":          actualLRP.InstanceGuid,
		"cell_id":                actualLRP.CellId,
		"state":                  actualLRP.State,
		"net_info":               netInfoData,
		"internal_routes":        internalRoutesData,
		"metric_tags":            metricTagsData,
		"routable":               routable,
		"availability_zone":      availabilityZone,
		"since":                  actualLRP.Since,
		"modification_tag_epoch": actualLRP.ModificationTag.Epoch,
		"modification_tag_index": actualLRP.ModificationTag.Index,
	}

	_, err = db.upsert(ctx, logger, tx, "actual_lrps",
		sqlAttributes,
		"process_guid = ? AND instance_index = ? AND presence = ?",
		actualLRP.ProcessGuid, actualLRP.Index, models.ActualLRP_Evacuating,
	)
	if err != nil {
		logger.Error("failed-inserting-evacuating-lrp", err)
		return nil, err
	}

	return actualLRP, nil
}
