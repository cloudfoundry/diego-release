package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-info/internalroutes"
)

func (sqldb *SQLDB) ConvergeLRPs(ctx context.Context, logger lager.Logger, cellSet models.CellSet) db.ConvergenceResult {
	logger = logger.Session("db-converge-lrps")
	logger.Info("starting")
	defer logger.Info("complete")

	now := sqldb.clock.Now()
	sqldb.pruneDomains(ctx, logger, now)
	events, instanceEvents := sqldb.pruneEvacuatingActualLRPs(ctx, logger, cellSet)
	domainSet, err := sqldb.domainSet(ctx, logger)
	if err != nil {
		return db.ConvergenceResult{}
	}

	converge := newConvergence(sqldb)
	converge.staleUnclaimedActualLRPs(ctx, logger, now)
	converge.actualLRPsWithMissingCells(ctx, logger, cellSet)
	converge.lrpInstanceCounts(ctx, logger, domainSet)
	converge.orphanedActualLRPs(ctx, logger)
	converge.orphanedSuspectActualLRPs(ctx, logger)
	converge.extraSuspectActualLRPs(ctx, logger)
	converge.suspectActualLRPsWithExistingCells(ctx, logger, cellSet)
	converge.suspectRunningActualLRPs(ctx, logger)
	converge.suspectClaimedActualLRPs(ctx, logger)
	converge.crashedActualLRPs(ctx, logger, now)
	converge.lrpsWithInternalRouteChanges(ctx, logger)
	converge.lrpsWithMetricTagChanges(ctx, logger)

	return db.ConvergenceResult{
		MissingLRPKeys:               converge.missingLRPKeys,
		UnstartedLRPKeys:             converge.unstartedLRPKeys,
		KeysToRetire:                 converge.keysToRetire,
		SuspectLRPKeysToRetire:       converge.suspectKeysToRetire,
		KeysWithMissingCells:         converge.ordinaryKeysWithMissingCells,
		MissingCellIds:               converge.missingCellIds,
		Events:                       events,
		InstanceEvents:               instanceEvents,
		SuspectKeysWithExistingCells: converge.suspectKeysWithExistingCells,
		SuspectRunningKeys:           converge.suspectRunningKeys,
		SuspectClaimedKeys:           converge.suspectClaimedKeys,
		KeysWithInternalRouteChanges: converge.keysWithInternalRouteChanges,
		KeysWithMetricTagChanges:     converge.keysWithMetricTagChanges,
	}
}

type convergence struct {
	*SQLDB

	ordinaryKeysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo
	missingCellIds               []string
	suspectKeysWithExistingCells []*models.ActualLRPKey

	suspectKeysToRetire []*models.ActualLRPKey

	suspectRunningKeys []*models.ActualLRPKey
	suspectClaimedKeys []*models.ActualLRPKey

	keysToRetire []*models.ActualLRPKey

	missingLRPKeys []*models.ActualLRPKeyWithSchedulingInfo

	unstartedLRPKeys []*models.ActualLRPKeyWithSchedulingInfo

	keysWithInternalRouteChanges []*db.ActualLRPKeyWithInternalRoutes
	keysWithMetricTagChanges     []*db.ActualLRPKeyWithMetricTags
}

func newConvergence(db *SQLDB) *convergence {
	return &convergence{
		SQLDB: db,
	}
}

// Adds stale UNCLAIMED Actual LRPs to the list of start requests.
func (c *convergence) staleUnclaimedActualLRPs(ctx context.Context, logger lager.Logger, now time.Time) {
	logger = logger.Session("stale-unclaimed-actual-lrps")

	rows, err := c.selectStaleUnclaimedLRPs(ctx, logger, c.db, now)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		var index int
		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index)
		if err != nil {
			continue
		}
		key := models.NewActualLRPKey(schedulingInfo.ProcessGuid, int32(index), schedulingInfo.Domain)
		c.unstartedLRPKeys = append(c.unstartedLRPKeys, &models.ActualLRPKeyWithSchedulingInfo{
			Key:            &key,
			SchedulingInfo: schedulingInfo,
		})
		logger.Info("creating-start-request",
			lager.Data{"reason": "stale-unclaimed-lrp", "process_guid": schedulingInfo.ProcessGuid, "index": index})
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

}

// Adds CRASHED Actual LRPs that can be restarted to the list of start requests
// and transitions them to UNCLAIMED.
func (c *convergence) crashedActualLRPs(ctx context.Context, logger lager.Logger, now time.Time) {
	logger = logger.Session("crashed-actual-lrps")
	restartCalculator := models.NewDefaultRestartCalculator()

	rows, err := c.selectCrashedLRPs(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		var index int
		actual := &models.ActualLRP{}

		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index, &actual.Since, &actual.CrashCount)
		if err != nil {
			continue
		}

		actual.ActualLRPKey = models.NewActualLRPKey(schedulingInfo.ProcessGuid, int32(index), schedulingInfo.Domain)
		actual.State = models.ActualLRPStateCrashed

		if actual.ShouldRestartCrash(now, restartCalculator) {
			c.unstartedLRPKeys = append(c.unstartedLRPKeys, &models.ActualLRPKeyWithSchedulingInfo{
				Key:            &actual.ActualLRPKey,
				SchedulingInfo: schedulingInfo,
			})
			logger.Info("creating-start-request",
				lager.Data{"reason": "crashed-instance", "process_guid": actual.ProcessGuid, "index": index})
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

}

func (c *convergence) lrpsWithInternalRouteChanges(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("lrps-with-internal-route-changes")
	rows, err := c.selectLRPsWithRoutes(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		actualLRPKey := &models.ActualLRPKey{}
		actualLRPInstanceKey := &models.ActualLRPInstanceKey{}
		var actualRouteData []byte
		var desiredRouteData []byte

		values := []interface{}{
			&actualLRPKey.ProcessGuid,
			&actualLRPKey.Index,
			&actualLRPKey.Domain,
			&actualLRPInstanceKey.InstanceGuid,
			&actualLRPInstanceKey.CellId,
			&actualRouteData,
			&desiredRouteData,
		}

		err := rows.Scan(values...)
		if err == sql.ErrNoRows {
			continue
		}

		if err != nil {
			logger.Error("failed-scanning", err)
			continue
		}

		var desiredRoutes models.Routes
		decodedDesiredData, err := c.encoder.Decode(desiredRouteData)
		if err != nil {
			logger.Error("failed-decrypting-desired-routes", err)
			continue
		}
		err = json.Unmarshal(decodedDesiredData, &desiredRoutes)
		if err != nil {
			logger.Error("failed-parsing-desired-routes", err)
			continue
		}

		actualInternalRoutes := internalroutes.InternalRoutes{}
		if len(actualRouteData) > 0 {
			decodedActualData, err := c.encoder.Decode(actualRouteData)
			if err != nil {
				logger.Error("failed-decrypting-actual-routes", err)
				continue
			}
			err = json.Unmarshal(decodedActualData, &actualInternalRoutes)
			if err != nil {
				logger.Error("failed-parsing-actual-routes", err)
				continue
			}
		}

		desiredInternalRoutes, err := internalroutes.InternalRoutesFromRoutingInfo(desiredRoutes)
		if err != nil {
			logger.Error("failed-getting-internal-routes-from-desired", err)
			continue
		}

		if !actualInternalRoutes.Equal(desiredInternalRoutes) {
			c.keysWithInternalRouteChanges = append(c.keysWithInternalRouteChanges, &db.ActualLRPKeyWithInternalRoutes{
				Key:                   actualLRPKey,
				InstanceKey:           actualLRPInstanceKey,
				DesiredInternalRoutes: desiredInternalRoutes,
			})
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

}

func (c *convergence) lrpsWithMetricTagChanges(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("lrps-with-metric-tag-changes")
	rows, err := c.selectLRPsWithMetricTags(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		actualLRPKey := &models.ActualLRPKey{}
		actualLRPInstanceKey := &models.ActualLRPInstanceKey{}
		var actualMetricTagData []byte
		var desiredMetricTagData []byte

		values := []interface{}{
			&actualLRPKey.ProcessGuid,
			&actualLRPKey.Index,
			&actualLRPKey.Domain,
			&actualLRPInstanceKey.InstanceGuid,
			&actualLRPInstanceKey.CellId,
			&actualMetricTagData,
			&desiredMetricTagData,
		}

		err := rows.Scan(values...)
		if err == sql.ErrNoRows {
			continue
		}

		if err != nil {
			logger.Error("failed-scanning", err)
			continue
		}

		var metricTags map[string]*models.MetricTagValue
		decodedDesiredData, err := c.encoder.Decode(desiredMetricTagData)
		if err != nil {
			logger.Error("failed-decrypting-desired-metric-tags", err)
			continue
		}
		err = json.Unmarshal(decodedDesiredData, &metricTags)
		if err != nil {
			logger.Error("failed-parsing-desired-metric-tags", err)
			continue
		}
		desiredMetricTags, err := models.ConvertMetricTags(metricTags, map[models.MetricTagValue_DynamicValue]interface{}{
			models.MetricTagDynamicValueIndex:        actualLRPKey.Index,
			models.MetricTagDynamicValueInstanceGuid: actualLRPInstanceKey.InstanceGuid,
		})
		if err != nil {
			logger.Error("converting-metric-tags-failed", err)
			continue
		}

		var actualMetricTags map[string]string
		if len(actualMetricTagData) > 0 {
			decodedActualData, err := c.encoder.Decode(actualMetricTagData)
			if err != nil {
				logger.Error("failed-decrypting-actual-metric-tags", err)
				continue
			}
			err = json.Unmarshal(decodedActualData, &actualMetricTags)
			if err != nil {
				logger.Error("failed-parsing-actual-metric-tags", err)
				continue
			}
		}

		if actualMetricTags != nil && !reflect.DeepEqual(desiredMetricTags, actualMetricTags) {
			c.keysWithMetricTagChanges = append(c.keysWithMetricTagChanges, &db.ActualLRPKeyWithMetricTags{
				Key:               actualLRPKey,
				InstanceKey:       actualLRPInstanceKey,
				DesiredMetricTags: desiredMetricTags,
			})
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

}

func scanActualLRPs(logger lager.Logger, rows *sql.Rows) []*models.ActualLRPKey {
	var actualLRPKeys []*models.ActualLRPKey
	for rows.Next() {
		actualLRPKey := &models.ActualLRPKey{}

		err := rows.Scan(
			&actualLRPKey.ProcessGuid,
			&actualLRPKey.Index,
			&actualLRPKey.Domain,
		)
		if err != nil {
			logger.Error("failed-scanning", err)
			continue
		}

		actualLRPKeys = append(actualLRPKeys, actualLRPKey)
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}
	return actualLRPKeys
}

// Adds orphaned Actual LRPs (ones with no corresponding Desired LRP) to the
// list of keys to retire.
func (c *convergence) orphanedActualLRPs(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("orphaned-actual-lrps")

	rows, err := c.selectOrphanedActualLRPs(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.keysToRetire = append(c.keysToRetire, scanActualLRPs(logger, rows)...)
}

func (c *convergence) extraSuspectActualLRPs(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("extra-suspect-lrps")

	rows, err := c.selectExtraSuspectActualLRPs(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectKeysToRetire = append(c.suspectKeysToRetire, scanActualLRPs(logger, rows)...)
}

func (c *convergence) orphanedSuspectActualLRPs(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("orphaned-suspect-lrps")

	rows, err := c.selectOrphanedSuspectActualLRPs(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectKeysToRetire = append(c.suspectKeysToRetire, scanActualLRPs(logger, rows)...)
}

func (c *convergence) suspectRunningActualLRPs(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("suspect-running-lrps")

	rows, err := c.selectSuspectRunningActualLRPs(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectRunningKeys = scanActualLRPs(logger, rows)
}

func (c *convergence) suspectClaimedActualLRPs(ctx context.Context, logger lager.Logger) {
	logger = logger.Session("suspect-running-lrps")

	rows, err := c.selectSuspectClaimedActualLRPs(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectClaimedKeys = scanActualLRPs(logger, rows)
}

// Creates and adds missing Actual LRPs to the list of start requests.
// Adds extra Actual LRPs  to the list of keys to retire.
func (c *convergence) lrpInstanceCounts(ctx context.Context, logger lager.Logger, domainSet map[string]struct{}) {
	logger = logger.Session("lrp-instance-counts")

	rows, err := c.selectLRPInstanceCounts(ctx, logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		var existingIndicesStr sql.NullString
		var actualInstances int

		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &actualInstances, &existingIndicesStr)
		if err != nil {
			continue
		}

		existingIndices := make(map[int]struct{})
		if existingIndicesStr.String != "" {
			for _, indexStr := range strings.Split(existingIndicesStr.String, ",") {
				index, err := strconv.Atoi(indexStr)
				if err != nil {
					logger.Error("cannot-parse-index", err, lager.Data{
						"index":                indexStr,
						"existing-indices-str": existingIndicesStr,
					})
					return
				}
				existingIndices[index] = struct{}{}
			}
		}

		for i := 0; i < int(schedulingInfo.Instances); i++ {
			_, found := existingIndices[i]
			if found {
				continue
			}

			index := int32(i)
			c.missingLRPKeys = append(c.missingLRPKeys, &models.ActualLRPKeyWithSchedulingInfo{
				Key: &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Domain:      schedulingInfo.Domain,
					Index:       index,
				},
				SchedulingInfo: schedulingInfo,
			})
			logger.Info("creating-start-request",
				lager.Data{"reason": "missing-instance", "process_guid": schedulingInfo.ProcessGuid, "index": index})
		}

		for index := range existingIndices {
			if index < int(schedulingInfo.Instances) {
				continue
			}

			// only take destructive actions for fresh domains
			if _, ok := domainSet[schedulingInfo.Domain]; ok {
				c.keysToRetire = append(c.keysToRetire, &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Index:       int32(index),
					Domain:      schedulingInfo.Domain,
				})
			}
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}
}

// Unclaim Actual LRPs that have missing cells (not in the cell set passed to
// convergence) and add them to the list of start requests.
func (c *convergence) suspectActualLRPsWithExistingCells(ctx context.Context, logger lager.Logger, cellSet models.CellSet) {
	logger = logger.Session("suspect-lrps-with-existing-cells")

	if len(cellSet) == 0 {
		return
	}

	rows, err := c.selectSuspectLRPsWithExistingCells(ctx, logger, c.db, cellSet)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectKeysWithExistingCells = scanActualLRPs(logger, rows)
}

// Unclaim Actual LRPs that have missing cells (not in the cell set passed to
// convergence) and add them to the list of start requests.
func (c *convergence) actualLRPsWithMissingCells(ctx context.Context, logger lager.Logger, cellSet models.CellSet) {
	logger = logger.Session("actual-lrps-with-missing-cells")

	var ordinaryKeysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo

	rows, err := c.selectLRPsWithMissingCells(ctx, logger, c.db, cellSet)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	missingCellSet := make(map[string]struct{})
	for rows.Next() {
		var index int32
		var cellID string
		var presence models.ActualLRP_Presence
		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index, &cellID, &presence)
		if err == nil && presence == models.ActualLRP_Ordinary {
			ordinaryKeysWithMissingCells = append(ordinaryKeysWithMissingCells, &models.ActualLRPKeyWithSchedulingInfo{
				Key: &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Domain:      schedulingInfo.Domain,
					Index:       index,
				},
				SchedulingInfo: schedulingInfo,
			})
		}
		missingCellSet[cellID] = struct{}{}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

	for key := range missingCellSet {
		c.missingCellIds = append(c.missingCellIds, key)
	}

	if len(c.missingCellIds) > 0 {
		logger.Info("detected-missing-cells", lager.Data{"cell_ids": c.missingCellIds})
	}

	c.ordinaryKeysWithMissingCells = ordinaryKeysWithMissingCells
}

func (db *SQLDB) pruneDomains(ctx context.Context, logger lager.Logger, now time.Time) {
	logger = logger.Session("prune-domains")

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		domains, err := db.domains(ctx, logger, tx, time.Time{})
		if err != nil {
			return err
		}

		for _, d := range domains {
			if d.expiresAt.After(now) {
				continue
			}

			logger.Info("pruning-domain", lager.Data{"domain": d.name, "expire-at": d.expiresAt})
			_, err := db.delete(ctx, logger, tx, domainsTable, "domain = ? ", d.name)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		logger.Error("cannot-prune-domains", err)
	}
}

func (db *SQLDB) pruneEvacuatingActualLRPs(ctx context.Context, logger lager.Logger, cellSet models.CellSet) ([]models.Event, []models.Event) {
	logger = logger.Session("prune-evacuating-actual-lrps")

	wheres := []string{"presence = ?"}
	bindings := []interface{}{models.ActualLRP_Evacuating}

	if len(cellSet) > 0 {
		wheres = append(wheres, fmt.Sprintf("actual_lrps.cell_id NOT IN (%s)", helpers.QuestionMarks(len(cellSet))))

		for cellID := range cellSet {
			bindings = append(bindings, cellID)
		}
	}

	lrpsToDelete, err := db.getActualLRPs(ctx, logger, strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-fetching-evacuating-lrps-with-missing-cells", err)
	}

	_, err = db.delete(ctx, logger, db.db, actualLRPsTable, strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-query", err)
	}

	var events []models.Event
	var instanceEvents []models.Event
	for _, lrp := range lrpsToDelete {
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		events = append(events, models.NewActualLRPRemovedEvent(lrp.ToActualLRPGroup()))
		instanceEvents = append(instanceEvents, models.NewActualLRPInstanceRemovedEvent(lrp, trace.RequestIdFromContext(ctx)))
	}
	return events, instanceEvents
}

func (db *SQLDB) domainSet(ctx context.Context, logger lager.Logger) (map[string]struct{}, error) {
	logger.Debug("listing-domains")
	domains, err := db.FreshDomains(ctx, logger)
	if err != nil {
		logger.Error("failed-listing-domains", err)
		return nil, err
	}
	logger.Debug("succeeded-listing-domains")
	m := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		m[domain] = struct{}{}
	}
	return m, nil
}
