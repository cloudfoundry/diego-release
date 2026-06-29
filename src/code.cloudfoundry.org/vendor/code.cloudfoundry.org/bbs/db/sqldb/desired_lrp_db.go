package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) DesireLRP(ctx context.Context, logger lager.Logger, desiredLRP *models.DesiredLRP) error {
	logger = logger.Session("db-desire-lrp", lager.Data{"process_guid": desiredLRP.ProcessGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	return db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		routesData, err := db.encodeRouteData(logger, desiredLRP.Routes)
		if err != nil {
			logger.Error("failed-encoding-route-data", err)
			return err
		}

		metricTagsData, err := db.encodeDesiredMetricTagsData(logger, desiredLRP.MetricTags)
		if err != nil {
			logger.Error("failed-encoding-metric-tags-data", err)
			return err
		}

		runInfo := desiredLRP.DesiredLRPRunInfo(db.clock.Now())

		runInfoData, err := db.serializeModel(logger, &runInfo)
		if err != nil {
			logger.Error("failed-to-serialize-model", err)
			return err
		}

		volumePlacement := &models.VolumePlacement{}
		volumePlacement.DriverNames = []string{}
		for _, mount := range desiredLRP.VolumeMounts {
			volumePlacement.DriverNames = append(volumePlacement.DriverNames, mount.Driver)
		}

		volumePlacementData, err := db.serializeModel(logger, volumePlacement)
		if err != nil {
			logger.Error("failed-to-serialize-model", err)
			return err
		}

		guid, err := db.guidProvider.NextGUID()
		if err != nil {
			logger.Error("failed-to-generate-guid", err)
			return models.ErrGUIDGeneration
		}

		placementTagData, err := json.Marshal(desiredLRP.PlacementTags)
		if err != nil {
			logger.Error("failed-to-serialize-model", err)
			return err
		}

		desiredLRP.ModificationTag = &models.ModificationTag{Epoch: guid, Index: 0}

		_, err = db.insert(ctx, logger, tx, desiredLRPsTable,
			helpers.SQLAttributes{
				"process_guid":           desiredLRP.ProcessGuid,
				"domain":                 desiredLRP.Domain,
				"log_guid":               desiredLRP.LogGuid,
				"annotation":             desiredLRP.Annotation,
				"instances":              desiredLRP.Instances,
				"memory_mb":              desiredLRP.MemoryMb,
				"disk_mb":                desiredLRP.DiskMb,
				"max_pids":               desiredLRP.MaxPids,
				"rootfs":                 desiredLRP.RootFs,
				"volume_placement":       volumePlacementData,
				"modification_tag_epoch": desiredLRP.ModificationTag.Epoch,
				"modification_tag_index": desiredLRP.ModificationTag.Index,
				"routes":                 routesData,
				"run_info":               runInfoData,
				"placement_tags":         placementTagData,
				"metric_tags":            metricTagsData,
				"update_strategy":        desiredLRP.UpdateStrategy,
			},
		)
		if err != nil {
			logger.Error("failed-inserting-desired", err)
			return err
		}
		return nil
	})
}

func (db *SQLDB) DesiredLRPByProcessGuid(ctx context.Context, logger lager.Logger, processGuid string) (*models.DesiredLRP, error) {
	logger = logger.Session("db-desired-lrp-by-process-guid", lager.Data{"process_guid": processGuid})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var desiredLRP *models.DesiredLRP

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		row := db.one(ctx, logger, tx, desiredLRPsTable,
			desiredLRPColumns, helpers.NoLockRow,
			"process_guid = ?", processGuid,
		)

		desiredLRP, _, err = db.fetchDesiredLRP(ctx, logger, row, tx)
		return err
	})

	return desiredLRP, err
}

func (db *SQLDB) DesiredLRPs(ctx context.Context, logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
	logger = logger.Session("db-desired-lrps", lager.Data{"filter": filter})
	logger.Debug("start")
	defer logger.Debug("complete")

	var wheres []string
	var values []interface{}

	if len(filter.AppGuids) > 0 {
		var appGuidWheres []string
		for _, g := range filter.AppGuids {
			appGuidWheres = append(appGuidWheres, "process_guid LIKE ?")
			values = append(values, g+"%")
		}
		if len(filter.AppGuids) == 1 {
			wheres = append(wheres, appGuidWheres[0])
		} else {
			wheres = append(wheres, "("+strings.Join(appGuidWheres, " OR ")+")")
		}
	}

	if filter.Domain != "" {
		wheres = append(wheres, "domain = ?")
		values = append(values, filter.Domain)
	}

	if len(filter.ProcessGuids) > 0 {
		wheres = append(wheres, whereClauseForProcessGuids(filter.ProcessGuids))

		for _, guid := range filter.ProcessGuids {
			values = append(values, guid)
		}
	}

	results := []*models.DesiredLRP{}

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		rows, err := db.all(ctx, logger, tx, desiredLRPsTable,
			desiredLRPColumns, helpers.NoLockRow,
			strings.Join(wheres, " AND "), values...,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}
		defer rows.Close()

		results, err = db.fetchDesiredLRPs(ctx, logger, rows, tx)
		if err != nil {
			logger.Error("failed-fetching-row", rows.Err())
			return db.convertSQLError(rows.Err())
		}

		return nil
	})

	return results, err
}

func (db *SQLDB) DesiredLRPSchedulingInfos(ctx context.Context, logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
	logger = logger.Session("db-desired-lrps-scheduling-infos", lager.Data{"filter": filter})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var wheres []string
	var values []interface{}

	if filter.Domain != "" {
		wheres = append(wheres, "domain = ?")
		values = append(values, filter.Domain)
	}

	if len(filter.ProcessGuids) > 0 {
		wheres = append(wheres, whereClauseForProcessGuids(filter.ProcessGuids))

		for _, guid := range filter.ProcessGuids {
			values = append(values, guid)
		}
	}

	results := []*models.DesiredLRPSchedulingInfo{}

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		rows, err := db.all(ctx, logger, tx, desiredLRPsTable,
			schedulingInfoColumns, helpers.NoLockRow,
			strings.Join(wheres, " AND "), values...,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			desiredLRPSchedulingInfo, err := db.fetchDesiredLRPSchedulingInfo(logger, rows)
			if err != nil {
				logger.Error("failed-reading-row", err)
				continue
			}
			results = append(results, desiredLRPSchedulingInfo)
		}

		if rows.Err() != nil {
			logger.Error("failed-fetching-row", rows.Err())
			return db.convertSQLError(rows.Err())
		}

		return nil
	})

	return results, err
}

func (db *SQLDB) DesiredLRPSchedulingInfoByProcessGuid(ctx context.Context, logger lager.Logger, processGuid string) (*models.DesiredLRPSchedulingInfo, error) {
	logger = logger.Session("db-desired-lrp-scheduling-info-by-process-guid", lager.Data{"process_guid": processGuid})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var desiredLRPSchedulingInfo *models.DesiredLRPSchedulingInfo
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		row := db.one(ctx, logger, tx, desiredLRPsTable,
			schedulingInfoColumns, helpers.NoLockRow,
			"process_guid = ?", processGuid,
		)

		desiredLRPSchedulingInfo, err = db.fetchDesiredLRPSchedulingInfo(logger, row)
		return err
	})

	return desiredLRPSchedulingInfo, err
}

func (db *SQLDB) DesiredLRPUpdateStrategyByProcessGuid(ctx context.Context, logger lager.Logger, processGuid string) (models.DesiredLRP_UpdateStrategy, int32, error) {
	logger = logger.Session("db-desired-lrp-update-strategy-by-process-guid", lager.Data{"process_guid": processGuid})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var updateStrategy models.DesiredLRP_UpdateStrategy
	var instances int32
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		row := db.one(ctx, logger, tx, desiredLRPsTable,
			helpers.ColumnList{"update_strategy", "instances"}, helpers.NoLockRow,
			"process_guid = ?", processGuid,
		)
		err := row.Scan(&updateStrategy, &instances)
		if err != nil {
			logger.Error("failed-scanning-row", err)
			return db.convertSQLError(err)
		}
		return nil
	})

	return updateStrategy, instances, err
}

func (db *SQLDB) DesiredLRPRoutingInfos(ctx context.Context, logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
	logger = logger.Session("db-desired-lrps-routing-infos", lager.Data{"filter": filter})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var wheres []string
	var values []interface{}

	if len(filter.ProcessGuids) > 0 {
		wheres = append(wheres, whereClauseForProcessGuids(filter.ProcessGuids))

		for _, guid := range filter.ProcessGuids {
			values = append(values, guid)
		}
	}

	results := []*models.DesiredLRP{}

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		rows, err := db.all(ctx, logger, tx, desiredLRPsTable,
			routingInfoColumns, helpers.NoLockRow,
			strings.Join(wheres, " AND "), values...,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			desiredLRPRoutingInfo, err := db.fetchDesiredLRPRoutingInfo(logger, rows)
			if err != nil {
				logger.Error("failed-reading-row", err)
				continue
			}
			results = append(results, desiredLRPRoutingInfo)
		}

		if rows.Err() != nil {
			logger.Error("failed-fetching-row", rows.Err())
			return db.convertSQLError(rows.Err())
		}

		return nil
	})

	return results, err
}

func (db *SQLDB) UpdateDesiredLRP(ctx context.Context, logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate) (*models.DesiredLRP, error) {
	logger = logger.Session("db-update-desired-lrp", lager.Data{"process_guid": processGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeDesiredLRP *models.DesiredLRP
	var originalRunInfo *models.DesiredLRPRunInfo
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		row := db.one(ctx, logger, tx, desiredLRPsTable,
			desiredLRPColumns, helpers.LockRow,
			"process_guid = ?", processGuid,
		)
		beforeDesiredLRP, originalRunInfo, err = db.fetchDesiredLRP(ctx, logger, row, tx)

		if err != nil {
			logger.Error("failed-lock-desired", err)
			return err
		}

		updateAttributes := helpers.SQLAttributes{"modification_tag_index": beforeDesiredLRP.ModificationTag.Index + 1}

		if update.AnnotationExists() {
			updateAttributes["annotation"] = update.GetAnnotation()
		}

		if update.InstancesExists() {
			updateAttributes["instances"] = update.GetInstances()
		}

		if update.Routes != nil {
			encodedData, err := db.encodeRouteData(logger, update.Routes)
			if err != nil {
				return err
			}
			updateAttributes["routes"] = encodedData
		}

		if update.MetricTags != nil {
			encodedData, err := db.encodeDesiredMetricTagsData(logger, update.MetricTags)
			if err != nil {
				return err
			}
			updateAttributes["metric_tags"] = encodedData
		}

		if update.ImageUsernameExists() || update.ImagePasswordExists() {
			runInfo := *originalRunInfo

			if update.ImageUsernameExists() {
				runInfo.ImageUsername = update.GetImageUsername()
			}
			if update.ImagePasswordExists() {
				runInfo.ImagePassword = update.GetImagePassword()
			}

			updatedRunInfoData, err := db.serializeModel(logger, &runInfo)
			if err != nil {
				logger.Error("failed-serializing-run-info", err)
				return err
			}
			updateAttributes["run_info"] = updatedRunInfoData
		}

		_, err = db.update(ctx, logger, tx, desiredLRPsTable, updateAttributes, `process_guid = ?`, processGuid)
		if err != nil {
			logger.Error("failed-executing-query", err)
			return err
		}

		return nil
	})

	return beforeDesiredLRP, err
}

func (db *SQLDB) encodeRouteData(logger lager.Logger, routes *models.Routes) ([]byte, error) {
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

func (db *SQLDB) encodeDesiredMetricTagsData(logger lager.Logger, metricTags map[string]*models.MetricTagValue) ([]byte, error) {
	metricTagsData, err := json.Marshal(metricTags)
	if err != nil {
		logger.Error("failed-marshalling-routes", err)
		return nil, models.ErrBadRequest
	}
	encodedData, err := db.encoder.Encode(metricTagsData)
	if err != nil {
		logger.Error("failed-encrypting-routes", err)
		return nil, models.ErrBadRequest
	}
	return encodedData, nil
}

func (db *SQLDB) RemoveDesiredLRP(ctx context.Context, logger lager.Logger, processGuid string) error {
	logger = logger.Session("db-remove-desired-lrp", lager.Data{"process_guid": processGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	return db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		err := db.lockDesiredLRPByGuidForUpdate(ctx, logger, processGuid, tx)
		if err != nil {
			logger.Error("failed-lock-desired", err)
			return err
		}

		_, err = db.delete(ctx, logger, tx, desiredLRPsTable, "process_guid = ?", processGuid)
		if err != nil {
			logger.Error("failed-deleting-from-db", err)
			return err
		}

		return nil
	})
}

// "rows" needs to have the columns defined in the schedulingInfoColumns constant
func (db *SQLDB) fetchDesiredLRPSchedulingInfoAndMore(logger lager.Logger, scanner helpers.RowScanner, dest ...interface{}) (*models.DesiredLRPSchedulingInfo, error) {
	schedulingInfo := &models.DesiredLRPSchedulingInfo{}
	var routeData, volumePlacementData, placementTagData []byte
	values := []interface{}{
		&schedulingInfo.ProcessGuid,
		&schedulingInfo.Domain,
		&schedulingInfo.LogGuid,
		&schedulingInfo.Annotation,
		&schedulingInfo.Instances,
		&schedulingInfo.MemoryMb,
		&schedulingInfo.DiskMb,
		&schedulingInfo.MaxPids,
		&schedulingInfo.RootFs,
		&routeData,
		&volumePlacementData,
		&schedulingInfo.ModificationTag.Epoch,
		&schedulingInfo.ModificationTag.Index,
		&placementTagData,
	}
	values = append(values, dest...)

	err := scanner.Scan(values...)
	if err == sql.ErrNoRows {
		return nil, err
	}

	if err != nil {
		logger.Error("failed-scanning", err)
		return nil, err
	}

	var routes models.Routes
	encodedData, err := db.encoder.Decode(routeData)
	if err != nil {
		logger.Error("failed-decrypting-routes", err)
		return nil, err
	}
	err = json.Unmarshal(encodedData, &routes)
	if err != nil {
		logger.Error("failed-parsing-routes", err)
		return nil, err
	}
	schedulingInfo.Routes = routes

	var volumePlacement models.VolumePlacement
	err = db.deserializeModel(logger, volumePlacementData, &volumePlacement)
	if err != nil {
		logger.Error("failed-parsing-volume-placement", err)
		return nil, err
	}
	schedulingInfo.VolumePlacement = &volumePlacement
	if placementTagData != nil {
		err = json.Unmarshal(placementTagData, &schedulingInfo.PlacementTags)
		if err != nil {
			logger.Error("failed-parsing-placement-tags", err)
			return nil, err
		}
	}

	return schedulingInfo, nil
}

func (db *SQLDB) fetchDesiredLRPRoutingInfo(logger lager.Logger, scanner helpers.RowScanner, dest ...interface{}) (*models.DesiredLRP, error) {
	routingInfo := &models.DesiredLRP{}
	var modificationTagEpoch string
	var modificationTagIndex uint32
	var routeData, metricTagsData []byte
	values := []interface{}{
		&routingInfo.ProcessGuid,
		&routingInfo.Domain,
		&routingInfo.LogGuid,
		&routingInfo.Instances,
		&routeData,
		&modificationTagEpoch,
		&modificationTagIndex,
		&metricTagsData,
	}

	err := scanner.Scan(values...)
	if err == sql.ErrNoRows {
		return nil, err
	}

	if err != nil {
		logger.Error("failed-scanning", err)
		return nil, err
	}
	var routes models.Routes
	encodedData, err := db.encoder.Decode(routeData)
	if err != nil {
		logger.Error("failed-decrypting-routes", err)
		return nil, err
	}
	err = json.Unmarshal(encodedData, &routes)
	if err != nil {
		logger.Error("failed-parsing-routes", err)
		return nil, err
	}
	routingInfo.Routes = &routes

	var metricTags map[string]*models.MetricTagValue
	decodedDesiredData, err := db.encoder.Decode(metricTagsData)
	if err != nil {
		logger.Error("failed-decrypting-metric-tags", err)
		return nil, err
	}
	err = json.Unmarshal(decodedDesiredData, &metricTags)
	if err != nil {
		logger.Error("failed-parsing-metric-tags", err)
		return nil, err
	}
	routingInfo.MetricTags = metricTags
	routingInfo.ModificationTag = &models.ModificationTag{Epoch: modificationTagEpoch, Index: modificationTagIndex}

	return routingInfo, nil
}

func (db *SQLDB) lockDesiredLRPByGuidForUpdate(ctx context.Context, logger lager.Logger, processGuid string, tx helpers.Tx) error {
	row := db.one(ctx, logger, tx, desiredLRPsTable,
		helpers.ColumnList{"1"}, helpers.LockRow,
		"process_guid = ?", processGuid,
	)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return err
	}
	return nil
}

func (db *SQLDB) fetchDesiredLRPs(ctx context.Context, logger lager.Logger, rows *sql.Rows, queryable helpers.Queryable) ([]*models.DesiredLRP, error) {
	guids := []string{}
	lrps := []*models.DesiredLRP{}
	for rows.Next() {
		lrp, _, guid, err := db.fetchDesiredLRPInternal(logger, rows)
		if err == models.ErrDeserialize {
			guids = append(guids, guid)
		}
		if err != nil {
			logger.Error("failed-reading-row", err)
			continue
		}
		lrps = append(lrps, lrp)
	}

	if len(guids) > 0 {
		deleteErr := db.deleteInvalidLRPs(ctx, logger, queryable, guids...)
		if deleteErr != nil {
			logger.Error("failed-to-delete-invalid-lrps", deleteErr, lager.Data{"guid": guids})
		}
	}

	if err := rows.Err(); err != nil {
		return lrps, err
	}

	return lrps, nil
}

func (db *SQLDB) fetchDesiredLRP(ctx context.Context, logger lager.Logger, scanner helpers.RowScanner, queryable helpers.Queryable) (*models.DesiredLRP, *models.DesiredLRPRunInfo, error) {
	lrp, runInfo, guid, err := db.fetchDesiredLRPInternal(logger, scanner)
	if err == models.ErrDeserialize {
		deleteErr := db.deleteInvalidLRPs(ctx, logger, queryable, guid)
		if deleteErr != nil {
			logger.Error("failed-to-delete-invalid-lrp", deleteErr, lager.Data{"guid": guid})
		}
	}
	return lrp, runInfo, err
}

func (db *SQLDB) fetchDesiredLRPInternal(logger lager.Logger, scanner helpers.RowScanner) (*models.DesiredLRP, *models.DesiredLRPRunInfo, string, error) {
	var runInfoData, metricTagsData []byte
	var updateStrategy models.DesiredLRP_UpdateStrategy
	schedulingInfo, err := db.fetchDesiredLRPSchedulingInfoAndMore(logger, scanner, &runInfoData, &metricTagsData, &updateStrategy)
	if err != nil {
		return nil, nil, "", err
	}

	var runInfo models.DesiredLRPRunInfo
	err = db.deserializeModel(logger, runInfoData, &runInfo)
	if err != nil {
		return nil, nil, schedulingInfo.ProcessGuid, models.ErrDeserialize
	}
	// dedup the ports
	runInfo.Ports = dedupSlice(runInfo.Ports)

	var metricTags map[string]*models.MetricTagValue
	encodedData, err := db.encoder.Decode(metricTagsData)
	if err != nil {
		logger.Error("failed-decrypting-metric-tags", err)
		return nil, nil, "", err
	}
	err = json.Unmarshal(encodedData, &metricTags)
	if err != nil {
		logger.Error("failed-parsing-metric-tags", err)
		return nil, nil, "", err
	}
	desiredLRP := models.NewDesiredLRP(*schedulingInfo, runInfo, metricTags, updateStrategy)
	return &desiredLRP, &runInfo, "", nil
}

func (db *SQLDB) deleteInvalidLRPs(ctx context.Context, logger lager.Logger, queryable helpers.Queryable, guids ...string) error {
	for _, guid := range guids {
		logger.Info("deleting-invalid-desired-lrp-from-db", lager.Data{"guid": guid})
		_, err := db.delete(ctx, logger, queryable, desiredLRPsTable, "process_guid = ?", guid)
		if err != nil {
			logger.Error("failed-deleting-invalid-row", err)
			return err
		}
	}
	return nil
}

func (db *SQLDB) fetchDesiredLRPSchedulingInfo(logger lager.Logger, scanner helpers.RowScanner) (*models.DesiredLRPSchedulingInfo, error) {
	return db.fetchDesiredLRPSchedulingInfoAndMore(logger, scanner)
}

func whereClauseForProcessGuids(filter []string) string {
	var questionMarks []string

	where := "process_guid IN ("
	for range filter {
		questionMarks = append(questionMarks, "?")
	}

	where += strings.Join(questionMarks, ", ")
	return where + ")"
}

func dedupSlice(ints []uint32) []uint32 {
	if ints == nil {
		// this is really here to make some tests happy, otherwise we replace the
		// nil with an empty slice and they barf
		return nil
	}

	set := make(map[uint32]struct{})
	for _, i := range ints {
		set[i] = struct{}{}
	}
	if len(ints) == len(set) {
		// short circuit the copying if the set has the same number of elements as
		// the slice
		return ints
	}

	newIs := make([]uint32, 0, len(ints))
	for i := range set {
		newIs = append(newIs, i)
	}
	return newIs
}
