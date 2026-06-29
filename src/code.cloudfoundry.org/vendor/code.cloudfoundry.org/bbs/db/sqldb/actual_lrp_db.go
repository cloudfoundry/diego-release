package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

const (
	Truncated = "(truncated)"
)

func (db *SQLDB) getActualLRPs(ctx context.Context, logger lager.Logger, wheres string, whereBindings ...interface{}) ([]*models.ActualLRP, error) {
	var actualLRPs []*models.ActualLRP
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		rows, err := db.all(ctx, logger, tx, actualLRPsTable,
			actualLRPColumns, helpers.NoLockRow,
			wheres, whereBindings...,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}
		defer rows.Close()
		actualLRPs, err = db.scanAndCleanupActualLRPs(ctx, logger, tx, rows)
		return err
	})

	return actualLRPs, err
}

func (db *SQLDB) ChangeActualLRPPresence(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, from, to models.ActualLRP_Presence) (before *models.ActualLRP, after *models.ActualLRP, err error) {
	logger = logger.Session("db-change-actual-lrp-presence", lager.Data{"key": key, "from": from, "to": to})
	logger.Info("starting")
	defer logger.Info("finished")

	var beforeLRP *models.ActualLRP
	var afterLRP models.ActualLRP
	err = db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		beforeLRP, err = db.fetchActualLRPForUpdate(ctx, logger, key.ProcessGuid, key.Index, from, tx)
		if err != nil {
			logger.Error("failed-fetching-lrp", err)
			return err
		}

		afterLRP = *beforeLRP
		afterLRP.Presence = to
		wheres := "process_guid = ? AND instance_index = ? AND presence = ?"
		_, err = db.update(ctx, logger, tx, actualLRPsTable, helpers.SQLAttributes{
			"presence": afterLRP.Presence,
		}, wheres, key.ProcessGuid, key.Index, beforeLRP.Presence)
		if err != nil {
			logger.Error("failed-updating-lrp", err)
		}
		return err
	})

	return beforeLRP, &afterLRP, err
}

func (db *SQLDB) ActualLRPs(ctx context.Context, logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRP, error) {
	logger = logger.Session("db-actual-lrps", lager.Data{"filter": filter})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var wheres []string
	var values []interface{}

	if filter.Domain != "" {
		wheres = append(wheres, "domain = ?")
		values = append(values, filter.Domain)
	}

	if filter.CellID != "" {
		wheres = append(wheres, "cell_id = ?")
		values = append(values, filter.CellID)
	}

	if filter.ProcessGuid != "" {
		wheres = append(wheres, "process_guid = ?")
		values = append(values, filter.ProcessGuid)
	}

	if filter.Index != nil {
		wheres = append(wheres, "instance_index = ?")
		values = append(values, *filter.Index)
	}

	lrps, err := db.getActualLRPs(ctx, logger, strings.Join(wheres, " AND "), values...)
	if err != nil {
		return nil, err
	}

	return lrps, nil
}

func (db *SQLDB) ActualLRPsByProcessGuids(ctx context.Context, logger lager.Logger, filter models.ActualLRPsByProcessGuidsFilter) ([]*models.ActualLRP, error) {
	logger = logger.Session("db-multiple-actual-lrps", lager.Data{"filter": filter})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var wheres []string
	var values []interface{}

	if len(filter.ProcessGuids) > 0 {
		placeholders := make([]string, len(filter.ProcessGuids))
		for i, guid := range filter.ProcessGuids {
			placeholders[i] = "?"
			values = append(values, guid)
		}
		wheres = append(wheres, fmt.Sprintf("process_guid IN (%s)", strings.Join(placeholders, ",")))
	}

	lrps, err := db.getActualLRPs(ctx, logger, strings.Join(wheres, " AND "), values...)
	if err != nil {
		return nil, err
	}

	return lrps, nil
}

func (db *SQLDB) CreateUnclaimedActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey) (*models.ActualLRP, error) {
	logger = logger.Session("db-create-unclaimed-actual-lrps", lager.Data{"key": key})
	logger.Info("starting")
	defer logger.Info("complete")

	guid, err := db.guidProvider.NextGUID()
	if err != nil {
		logger.Error("failed-to-generate-guid", err)
		return nil, models.ErrGUIDGeneration
	}

	netInfoData, err := db.serializeModel(logger, &models.ActualLRPNetInfo{})
	if err != nil {
		logger.Error("failed-to-serialize-net-info", err)
		return nil, err
	}

	internalRoutesData, err := db.encodeInternalRouteData(logger, []*models.ActualLRPInternalRoute{})
	if err != nil {
		logger.Error("failed-to-serialize-internal-routes", err)
		return nil, err
	}

	metricTagsData, err := db.encodeMetricTagsData(logger, map[string]string{})
	if err != nil {
		logger.Error("failed-to-serialize-metric-tags", err)
		return nil, err
	}

	now := db.clock.Now().UnixNano()
	err = db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		_, err := db.insert(ctx, logger, tx, actualLRPsTable,
			helpers.SQLAttributes{
				"process_guid":           key.ProcessGuid,
				"instance_index":         key.Index,
				"domain":                 key.Domain,
				"state":                  models.ActualLRPStateUnclaimed,
				"since":                  now,
				"net_info":               netInfoData,
				"modification_tag_epoch": guid,
				"modification_tag_index": 0,
				"internal_routes":        internalRoutesData,
				"metric_tags":            metricTagsData,
				"routable":               false,
			},
		)

		return err
	})
	if err != nil {
		logger.Error("failed-to-create-unclaimed-actual-lrp", err)
		return nil, err
	}
	lrp := &models.ActualLRP{
		ActualLRPKey:            *key,
		State:                   models.ActualLRPStateUnclaimed,
		Since:                   now,
		ModificationTag:         models.ModificationTag{Epoch: guid, Index: 0},
		ActualLrpInternalRoutes: []*models.ActualLRPInternalRoute{},
		MetricTags:              map[string]string{},
	}
	lrp.SetRoutable(false)
	return lrp, nil
}

func (db *SQLDB) UnclaimActualLRP(ctx context.Context, logger lager.Logger, isStale bool, key *models.ActualLRPKey) (*models.ActualLRP, *models.ActualLRP, error) {
	logger = logger.Session("db-unclaim-actual-lrp", lager.Data{"key": key})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeActualLRP *models.ActualLRP
	var actualLRP *models.ActualLRP
	processGuid := key.ProcessGuid
	index := key.Index

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		beforeActualLRP, actualLRP, err = db.unclaimActualLRP(ctx, logger, tx, processGuid, index, isStale)
		return err
	})

	return beforeActualLRP, actualLRP, err
}

func (db *SQLDB) UnclaimActualLRPIfAllRunning(ctx context.Context, logger lager.Logger, isStale bool, key *models.ActualLRPKey, desiredInstances int32) (*models.ActualLRP, *models.ActualLRP, error) {
	logger = logger.Session("db-unclaim-actual-lrp-if-all-running", lager.Data{"key": key})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeActualLRP *models.ActualLRP
	var actualLRP *models.ActualLRP
	processGuid := key.ProcessGuid
	index := key.Index

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		err := db.lockDesiredLRPByGuidForUpdate(ctx, logger, processGuid, tx)
		if err != nil {
			logger.Error("failed-locking-desired-lrp-by-guid", err)
			return err
		}
		actualInstances, err := db.countActualLRPsForProcessGuidAndState(ctx, logger, processGuid, models.ActualLRPStateRunning, models.ActualLRP_Ordinary, tx)
		if err != nil {
			logger.Error("failed-to-count-actual-lrps-for-process-guid-and-state", err)
			return err
		}

		if actualInstances < int(desiredInstances) {
			logger.Debug("not-enough-running-actual-lrps", lager.Data{"actual": actualInstances, "desired": desiredInstances})
			return models.ErrActualLRPCannotBeUnclaimed
		}
		beforeActualLRP, actualLRP, err = db.unclaimActualLRP(ctx, logger, tx, processGuid, index, isStale)
		return err
	})

	return beforeActualLRP, actualLRP, err
}

func (db *SQLDB) ClaimActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) (*models.ActualLRP, *models.ActualLRP, error) {
	logger = logger.Session("db-claim-actual-lrp", lager.Data{"process_guid": processGuid, "index": index, "instance_key": instanceKey})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeActualLRP models.ActualLRP
	var actualLRP *models.ActualLRP
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		actualLRP, err = db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Ordinary, tx)
		if err != nil {
			logger.Error("failed-fetching-actual-lrp-for-share", err)
			return err
		}
		beforeActualLRP = *actualLRP

		if !actualLRP.AllowsTransitionTo(&actualLRP.ActualLRPKey, instanceKey, models.ActualLRPStateClaimed) {
			logger.Error("cannot-transition-to-claimed", nil, lager.Data{"from_state": actualLRP.State, "same_instance_key": actualLRP.ActualLRPInstanceKey.Equal(instanceKey)})
			return models.ErrActualLRPCannotBeClaimed
		}

		if actualLRP.State == models.ActualLRPStateClaimed && actualLRP.ActualLRPInstanceKey.Equal(instanceKey) {
			return nil
		}

		actualLRP.ModificationTag.Increment()
		actualLRP.State = models.ActualLRPStateClaimed
		actualLRP.ActualLRPInstanceKey = *instanceKey
		actualLRP.PlacementError = ""
		actualLRP.ActualLRPNetInfo = models.ActualLRPNetInfo{}
		actualLRP.Since = db.clock.Now().UnixNano()
		netInfoData, err := db.serializeModel(logger, &models.ActualLRPNetInfo{})
		if err != nil {
			logger.Error("failed-to-serialize-net-info", err)
			return err
		}

		_, err = db.update(ctx, logger, tx, actualLRPsTable,
			helpers.SQLAttributes{
				"state":                  actualLRP.State,
				"cell_id":                actualLRP.CellId,
				"instance_guid":          actualLRP.InstanceGuid,
				"modification_tag_index": actualLRP.ModificationTag.Index,
				"placement_error":        actualLRP.PlacementError,
				"since":                  actualLRP.Since,
				"net_info":               netInfoData,
			},
			"process_guid = ? AND instance_index = ? AND presence = ?",
			processGuid, index, models.ActualLRP_Ordinary,
		)
		if err != nil {
			logger.Error("failed-claiming-actual-lrp", err)
			return err
		}

		return nil
	})

	return &beforeActualLRP, actualLRP, err
}

func (db *SQLDB) StartActualLRP(
	ctx context.Context,
	logger lager.Logger,
	key *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
	netInfo *models.ActualLRPNetInfo,
	internalRoutes []*models.ActualLRPInternalRoute,
	metricTags map[string]string,
	routable bool,
	availabilityZone string,
	isCurrentlyRunning bool,
) (*models.ActualLRP, *models.ActualLRP, error) {
	logger = logger.Session("db-start-actual-lrp", lager.Data{"actual_lrp_key": key, "actual_lrp_instance_key": instanceKey, "net_info": netInfo, "routable": routable})
	if db.debugStartActualLRPHeartbeats && isCurrentlyRunning {
		logger.Debug("heartbeating-lrp")
		defer logger.Debug("heartbeat-complete")
	} else {
		logger.Info("starting")
		defer logger.Info("complete")
	}

	var beforeActualLRP models.ActualLRP
	var actualLRP *models.ActualLRP

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		actualLRP, err = db.fetchActualLRPForUpdate(ctx, logger, key.ProcessGuid, key.Index, models.ActualLRP_Ordinary, tx)
		if err == models.ErrResourceNotFound {
			actualLRP, err = db.createRunningActualLRP(ctx, logger, key, instanceKey, netInfo, internalRoutes, metricTags, routable, availabilityZone, tx)
			return err
		}

		if err != nil {
			logger.Error("failed-to-get-actual-lrp", err)
			return err
		}

		beforeActualLRP = *actualLRP

		if actualLRP.ActualLRPKey.Equal(key) &&
			actualLRP.ActualLRPInstanceKey.Equal(instanceKey) &&
			actualLRP.ActualLRPNetInfo.Equal(netInfo) &&
			reflect.DeepEqual(actualLRP.ActualLrpInternalRoutes, internalRoutes) &&
			reflect.DeepEqual(actualLRP.MetricTags, metricTags) &&
			actualLRP.GetRoutable() == routable &&
			actualLRP.AvailabilityZone == availabilityZone &&
			actualLRP.State == models.ActualLRPStateRunning {
			logger.Debug("nothing-to-change")
			return nil
		}

		if !actualLRP.AllowsTransitionTo(key, instanceKey, models.ActualLRPStateRunning) {
			logger.Error("failed-to-transition-actual-lrp-to-started", nil)
			return models.ErrActualLRPCannotBeStarted
		}

		now := db.clock.Now().UnixNano()

		actualLRP.ActualLRPInstanceKey = *instanceKey
		actualLRP.ActualLRPNetInfo = *netInfo
		actualLRP.ActualLrpInternalRoutes = internalRoutes
		actualLRP.MetricTags = metricTags
		actualLRP.AvailabilityZone = availabilityZone
		actualLRP.State = models.ActualLRPStateRunning
		actualLRP.Since = now
		actualLRP.ModificationTag.Increment()
		actualLRP.PlacementError = ""
		actualLRP.SetRoutable(routable)

		netInfoData, err := db.serializeModel(logger, &actualLRP.ActualLRPNetInfo)
		if err != nil {
			logger.Error("failed-to-serialize-net-info", err)
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

		_, err = db.update(ctx, logger, tx, actualLRPsTable,
			helpers.SQLAttributes{
				"state":                  actualLRP.State,
				"cell_id":                actualLRP.CellId,
				"instance_guid":          actualLRP.InstanceGuid,
				"modification_tag_index": actualLRP.ModificationTag.Index,
				"placement_error":        actualLRP.PlacementError,
				"since":                  actualLRP.Since,
				"net_info":               netInfoData,
				"internal_routes":        internalRoutesData,
				"metric_tags":            metricTagsData,
				"routable":               actualLRP.GetRoutable(),
				"availability_zone":      actualLRP.AvailabilityZone,
			},
			"process_guid = ? AND instance_index = ? AND presence = ?",
			key.ProcessGuid, key.Index, models.ActualLRP_Ordinary,
		)
		if err != nil {
			logger.Error("failed-starting-actual-lrp", err)
			return err
		}

		return nil
	})

	return &beforeActualLRP, actualLRP, err
}

func truncateString(s string, maxLen int) string {
	l := len(s)
	if l < maxLen {
		return s
	}
	return s[:maxLen-len(Truncated)] + Truncated
}

func (db *SQLDB) CrashActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, crashReason string) (*models.ActualLRP, *models.ActualLRP, bool, error) {
	logger = logger.Session("db-crash-actual-lrp", lager.Data{"key": key, "instance_key": instanceKey, "crash_reason": crashReason})
	logger.Info("starting")
	defer logger.Info("complete")

	immediateRestart := false
	var beforeActualLRP models.ActualLRP
	var actualLRP *models.ActualLRP

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		actualLRP, err = db.fetchActualLRPForUpdate(ctx, logger, key.ProcessGuid, key.Index, models.ActualLRP_Ordinary, tx)
		if err != nil {
			logger.Error("failed-to-get-actual-lrp", err)
			return err
		}
		beforeActualLRP = *actualLRP

		latestChangeTime := time.Duration(db.clock.Now().UnixNano() - actualLRP.Since)

		var newCrashCount int32
		if latestChangeTime > models.CrashResetTimeout && actualLRP.State == models.ActualLRPStateRunning {
			newCrashCount = 1
		} else {
			newCrashCount = actualLRP.CrashCount + 1
		}

		if !actualLRP.AllowsTransitionTo(&actualLRP.ActualLRPKey, instanceKey, models.ActualLRPStateCrashed) {
			logger.Error("failed-to-transition-to-crashed", nil, lager.Data{"from_state": actualLRP.State, "same_instance_key": actualLRP.ActualLRPInstanceKey.Equal(instanceKey)})
			return models.ErrActualLRPCannotBeCrashed
		}

		actualLRP.ModificationTag.Increment()
		actualLRP.State = models.ActualLRPStateCrashed

		actualLRP.ActualLRPInstanceKey.InstanceGuid = ""
		actualLRP.ActualLRPInstanceKey.CellId = ""
		actualLRP.ActualLRPNetInfo = models.ActualLRPNetInfo{}
		actualLRP.CrashCount = newCrashCount
		actualLRP.CrashReason = crashReason
		netInfoData, err := db.serializeModel(logger, &actualLRP.ActualLRPNetInfo)
		if err != nil {
			logger.Error("failed-to-serialize-net-info", err)
			return err
		}

		if actualLRP.ShouldRestartImmediately(models.NewDefaultRestartCalculator()) {
			actualLRP.State = models.ActualLRPStateUnclaimed
			immediateRestart = true
		}

		now := db.clock.Now().UnixNano()
		actualLRP.Since = now

		_, err = db.update(ctx, logger, tx, actualLRPsTable,
			helpers.SQLAttributes{
				"state":                  actualLRP.State,
				"cell_id":                actualLRP.CellId,
				"instance_guid":          actualLRP.InstanceGuid,
				"modification_tag_index": actualLRP.ModificationTag.Index,
				"crash_count":            actualLRP.CrashCount,
				"crash_reason":           truncateString(actualLRP.CrashReason, 1024),
				"since":                  actualLRP.Since,
				"net_info":               netInfoData,
			},
			"process_guid = ? AND instance_index = ? AND presence = ?",
			key.ProcessGuid, key.Index, models.ActualLRP_Ordinary,
		)
		if err != nil {
			logger.Error("failed-to-crash-actual-lrp", err)
			return err
		}

		return nil
	})

	return &beforeActualLRP, actualLRP, immediateRestart, err
}

func (db *SQLDB) FailActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, placementError string) (*models.ActualLRP, *models.ActualLRP, error) {
	logger = logger.Session("db-fail-actual-lrp", lager.Data{"actual_lrp_key": key, "placement_error": placementError})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeActualLRP models.ActualLRP
	var actualLRP *models.ActualLRP

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		actualLRP, err = db.fetchActualLRPForUpdate(ctx, logger, key.ProcessGuid, key.Index, models.ActualLRP_Ordinary, tx)
		if err != nil {
			logger.Error("failed-to-get-actual-lrp", err)
			return err
		}
		beforeActualLRP = *actualLRP

		if actualLRP.State != models.ActualLRPStateUnclaimed {
			logger.Error("cannot-fail-actual-lrp", nil, lager.Data{"from_state": actualLRP.State})
			return models.ErrActualLRPCannotBeFailed
		}

		now := db.clock.Now().UnixNano()
		actualLRP.ModificationTag.Increment()
		actualLRP.PlacementError = placementError
		actualLRP.Since = now

		_, err = db.update(ctx, logger, tx, actualLRPsTable,
			helpers.SQLAttributes{
				"modification_tag_index": actualLRP.ModificationTag.Index,
				"placement_error":        truncateString(actualLRP.PlacementError, 1024),
				"since":                  actualLRP.Since,
			},
			"process_guid = ? AND instance_index = ? AND presence = ?",
			key.ProcessGuid, key.Index, models.ActualLRP_Ordinary,
		)
		if err != nil {
			logger.Error("failed-failing-actual-lrp", err)
			return err
		}

		return nil
	})

	return &beforeActualLRP, actualLRP, err
}

func (db *SQLDB) RemoveActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) error {
	logger = logger.Session("db-remove-actual-lrp", lager.Data{"process_guid": processGuid, "index": index})
	logger.Info("starting")
	defer logger.Info("complete")

	return db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		var result sql.Result
		if instanceKey == nil {
			result, err = db.delete(ctx, logger, tx, actualLRPsTable,
				"process_guid = ? AND instance_index = ? AND presence = ?",
				processGuid, index, models.ActualLRP_Ordinary,
			)
		} else {
			result, err = db.delete(ctx, logger, tx, actualLRPsTable,
				"process_guid = ? AND instance_index = ? AND presence = ? AND instance_guid = ? AND cell_id = ?",
				processGuid, index, models.ActualLRP_Ordinary, instanceKey.InstanceGuid, instanceKey.CellId,
			)
		}
		if err != nil {
			logger.Error("failed-removing-actual-lrp", err)
			return err
		}

		numRows, err := result.RowsAffected()
		if err != nil {
			logger.Error("failed-getting-rows-affected", err)
			return err
		}
		if numRows == 0 {
			logger.Debug("not-found", lager.Data{"instance_key": instanceKey})
			return models.ErrResourceNotFound
		}

		return nil
	})
}

func (db *SQLDB) createRunningActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo, internalRoutes []*models.ActualLRPInternalRoute, metricTags map[string]string, routable bool, availabilityZone string, tx helpers.Tx) (*models.ActualLRP, error) {
	now := db.clock.Now().UnixNano()
	guid, err := db.guidProvider.NextGUID()
	if err != nil {
		return nil, models.ErrGUIDGeneration
	}

	actualLRP := &models.ActualLRP{}
	actualLRP.ModificationTag = models.NewModificationTag(guid, 0)
	actualLRP.ActualLRPKey = *key
	actualLRP.ActualLRPInstanceKey = *instanceKey
	actualLRP.ActualLRPNetInfo = *netInfo
	actualLRP.ActualLrpInternalRoutes = internalRoutes
	actualLRP.MetricTags = metricTags
	actualLRP.State = models.ActualLRPStateRunning
	actualLRP.Since = now
	actualLRP.SetRoutable(routable)
	actualLRP.AvailabilityZone = availabilityZone

	netInfoData, err := db.serializeModel(logger, &actualLRP.ActualLRPNetInfo)
	if err != nil {
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

	_, err = db.insert(ctx, logger, tx, actualLRPsTable,
		helpers.SQLAttributes{
			"process_guid":           actualLRP.ActualLRPKey.ProcessGuid,
			"instance_index":         actualLRP.ActualLRPKey.Index,
			"domain":                 actualLRP.ActualLRPKey.Domain,
			"instance_guid":          actualLRP.ActualLRPInstanceKey.InstanceGuid,
			"cell_id":                actualLRP.ActualLRPInstanceKey.CellId,
			"state":                  actualLRP.State,
			"net_info":               netInfoData,
			"internal_routes":        internalRoutesData,
			"metric_tags":            metricTagsData,
			"routable":               actualLRP.GetRoutable(),
			"availability_zone":      actualLRP.AvailabilityZone,
			"since":                  actualLRP.Since,
			"modification_tag_epoch": actualLRP.ModificationTag.Epoch,
			"modification_tag_index": actualLRP.ModificationTag.Index,
		},
	)
	if err != nil {
		logger.Error("failed-creating-running-actual-lrp", err)
		return nil, err
	}
	return actualLRP, nil
}

func (db *SQLDB) unclaimActualLRP(ctx context.Context, logger lager.Logger, tx helpers.Tx, processGuid string, index int32, isStale bool) (*models.ActualLRP, *models.ActualLRP, error) {
	var beforeActualLRP models.ActualLRP
	var actualLRP *models.ActualLRP
	var err error

	actualLRP, err = db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Ordinary, tx)
	if err != nil {
		logger.Error("failed-fetching-actual-lrp-for-share", err)
		return nil, nil, err
	}
	beforeActualLRP = *actualLRP

	if actualLRP.State == models.ActualLRPStateUnclaimed {
		logger.Debug("already-" + actualLRP.State)
		return nil, nil, models.ErrActualLRPCannotBeUnclaimed
	}
	if isStale && actualLRP.State == models.ActualLRPStateClaimed {
		logger.Debug("a stale unstarted claim already-" + actualLRP.State + " by another cell.")
		return nil, nil, models.ErrActualLRPCannotBeUnclaimed
	}

	now := db.clock.Now().UnixNano()
	actualLRP.ModificationTag.Increment()
	actualLRP.State = models.ActualLRPStateUnclaimed
	actualLRP.ActualLRPInstanceKey.CellId = ""
	actualLRP.ActualLRPInstanceKey.InstanceGuid = ""
	actualLRP.Since = now
	actualLRP.ActualLRPNetInfo = models.ActualLRPNetInfo{}
	netInfoData, err := db.serializeModel(logger, &models.ActualLRPNetInfo{})
	if err != nil {
		logger.Error("failed-to-serialize-net-info", err)
		return nil, nil, err
	}

	_, err = db.update(ctx, logger, tx, actualLRPsTable,
		helpers.SQLAttributes{
			"state":                  actualLRP.State,
			"cell_id":                actualLRP.CellId,
			"instance_guid":          actualLRP.InstanceGuid,
			"modification_tag_index": actualLRP.ModificationTag.Index,
			"since":                  actualLRP.Since,
			"net_info":               netInfoData,
		},
		"process_guid = ? AND instance_index = ? AND presence = ?",
		actualLRP.ProcessGuid, actualLRP.Index, models.ActualLRP_Ordinary,
	)
	if err != nil {
		logger.Error("failed-to-unclaim-actual-lrp", err)
		return nil, nil, err
	}

	return &beforeActualLRP, actualLRP, nil
}

func (db *SQLDB) scanToActualLRP(logger lager.Logger, row helpers.RowScanner) (*models.ActualLRP, error) {
	var netInfoData []byte
	var internalRoutesData []byte
	var metricTagsData []byte
	var routable bool
	var actualLRP models.ActualLRP

	err := row.Scan(
		&actualLRP.ProcessGuid,
		&actualLRP.Index,
		&actualLRP.Presence,
		&actualLRP.Domain,
		&actualLRP.State,
		&actualLRP.InstanceGuid,
		&actualLRP.CellId,
		&actualLRP.PlacementError,
		&actualLRP.Since,
		&netInfoData,
		&internalRoutesData,
		&metricTagsData,
		&routable,
		&actualLRP.AvailabilityZone,
		&actualLRP.ModificationTag.Epoch,
		&actualLRP.ModificationTag.Index,
		&actualLRP.CrashCount,
		&actualLRP.CrashReason,
	)
	if err != nil {
		logger.Error("failed-scanning-actual-lrp", err)
		return nil, err
	}

	if len(netInfoData) > 0 {
		err = db.deserializeModel(logger, netInfoData, &actualLRP.ActualLRPNetInfo)
		if err != nil {
			logger.Error("failed-unmarshaling-net-info-data", err)
			return &actualLRP, models.ErrDeserialize
		}
	}

	internalRoutes := []*models.ActualLRPInternalRoute{}
	if len(internalRoutesData) > 0 {
		decodedData, err := db.encoder.Decode(internalRoutesData)
		if err != nil {
			logger.Error("failed-decrypting-internal-routes", err)
			return &actualLRP, models.ErrDeserialize
		}
		err = json.Unmarshal(decodedData, &internalRoutes)
		if err != nil {
			logger.Error("failed-parsing-internal-routes", err)
			return &actualLRP, models.ErrDeserialize
		}
	}
	actualLRP.ActualLrpInternalRoutes = internalRoutes

	metricTags := map[string]string{}
	if len(metricTagsData) > 0 {
		decodedData, err := db.encoder.Decode(metricTagsData)
		if err != nil {
			logger.Error("failed-decrypting-metric-tags", err)
			return &actualLRP, models.ErrDeserialize
		}
		err = json.Unmarshal(decodedData, &metricTags)
		if err != nil {
			logger.Error("failed-parsing-metric-tags", err)
			return &actualLRP, models.ErrDeserialize
		}
	}
	actualLRP.MetricTags = metricTags
	actualLRP.SetRoutable(routable)

	return &actualLRP, nil
}

func (db *SQLDB) fetchActualLRPForUpdate(ctx context.Context, logger lager.Logger, processGuid string, index int32, presence models.ActualLRP_Presence, tx helpers.Tx) (*models.ActualLRP, error) {
	wheres := "process_guid = ? AND instance_index = ? AND presence = ?"
	bindings := []interface{}{processGuid, index, presence}

	rows, err := db.all(ctx, logger, tx, actualLRPsTable,
		actualLRPColumns, helpers.LockRow, wheres, bindings...)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, err
	}
	actualLRPs, err := db.scanAndCleanupActualLRPs(ctx, logger, tx, rows)
	if err != nil {
		return nil, err
	}

	if len(actualLRPs) == 0 {
		return nil, models.ErrResourceNotFound
	}

	if len(actualLRPs) > 1 {
		return nil, models.ErrResourceConflict
	}

	return actualLRPs[0], nil
}

func (db *SQLDB) scanAndCleanupActualLRPs(ctx context.Context, logger lager.Logger, q helpers.Queryable, rows *sql.Rows) ([]*models.ActualLRP, error) {
	result := []*models.ActualLRP{}
	actualsToDelete := []*models.ActualLRP{}

	for rows.Next() {
		actualLRP, err := db.scanToActualLRP(logger, rows)
		if err == models.ErrDeserialize {
			actualsToDelete = append(actualsToDelete, actualLRP)
			continue
		} else if err != nil {
			logger.Error("failed-scanning-actual-lrp", err)
			return nil, err
		}

		result = append(result, actualLRP)
	}
	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
		return nil, db.convertSQLError(rows.Err())
	}

	for _, actual := range actualsToDelete {
		_, err := db.delete(ctx, logger, q, actualLRPsTable,
			"process_guid = ? AND instance_index = ? AND presence = ?",
			actual.ProcessGuid, actual.Index, actual.Presence,
		)
		if err != nil {
			logger.Error("failed-cleaning-up-invalid-actual-lrp", err)
		}
	}

	return result, nil
}

func (db *SQLDB) encodeInternalRouteData(logger lager.Logger, routes []*models.ActualLRPInternalRoute) ([]byte, error) {
	routeData, err := json.Marshal(routes)
	if err != nil {
		logger.Error("failed-marshalling-routes", err)
		return nil, models.ErrBadRequest
	}
	encodedData, err := db.encoder.Encode(routeData)
	if err != nil {
		logger.Error("failed-encrypting-routes", err)
		return nil, models.ErrBadRequest
	}
	return encodedData, nil
}

func (db *SQLDB) encodeMetricTagsData(logger lager.Logger, tags map[string]string) ([]byte, error) {
	tagsData, err := json.Marshal(tags)
	if err != nil {
		logger.Error("failed-marshalling-metric-tags", err)
		return nil, models.ErrBadRequest
	}
	encodedData, err := db.encoder.Encode(tagsData)
	if err != nil {
		logger.Error("failed-encrypting-metric-tags", err)
		return nil, models.ErrBadRequest
	}
	return encodedData, nil
}
