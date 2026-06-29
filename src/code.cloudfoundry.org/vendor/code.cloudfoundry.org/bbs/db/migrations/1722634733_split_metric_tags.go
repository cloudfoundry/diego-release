package migrations

import (
	"database/sql"
	"encoding/json"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func init() {
	appendMigration(NewSplitMetricTags())
}

type SplitMetricTags struct {
	encoder    format.Encoder
	serializer format.Serializer
	clock      clock.Clock
	rawSQLDB   *sql.DB
	dbFlavor   string
}

func NewSplitMetricTags() migration.Migration {
	return new(SplitMetricTags)
}

func (e *SplitMetricTags) String() string {
	return migrationString(e)
}

func (e *SplitMetricTags) Version() int64 {
	return 1722634733
}

func (e *SplitMetricTags) SetCryptor(cryptor encryption.Cryptor) {
	e.encoder = format.NewEncoder(cryptor)
	e.serializer = format.NewSerializer(cryptor)
}

func (e *SplitMetricTags) SetRawSQLDB(db *sql.DB)    { e.rawSQLDB = db }
func (e *SplitMetricTags) SetClock(c clock.Clock)    { e.clock = c }
func (e *SplitMetricTags) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *SplitMetricTags) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("split-metric-tags-desired-lrps")
	logger.Info("starting")
	defer logger.Info("completed")

	var addColumnSQL string
	if e.dbFlavor == "mysql" {
		addColumnSQL = "ALTER TABLE desired_lrps ADD COLUMN metric_tags MEDIUMTEXT"
	} else {
		addColumnSQL = "ALTER TABLE desired_lrps ADD COLUMN IF NOT EXISTS metric_tags MEDIUMTEXT"
	}

	logger.Info("altering-table")
	_, err := tx.Exec(helpers.RebindForFlavor(addColumnSQL, e.dbFlavor))
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-table", err)
		return err
	}

	query := "SELECT process_guid, run_info FROM desired_lrps"

	rows, err := tx.Query(query)
	if err != nil {
		logger.Error("failed-query", err)
		return err
	}

	metricTagsMap := map[string]map[string]*models.MetricTagValue{}

	var processGuid string
	var runInfoData []byte

	if rows.Err() != nil {
		logger.Error("failed-fetching-row", rows.Err())
		return rows.Err()
	}

	for rows.Next() {
		err := rows.Scan(&processGuid, &runInfoData)
		if err != nil {
			logger.Error("failed-reading-row", err)
			continue
		}
		var runInfo models.DesiredLRPRunInfo
		err = e.serializer.Unmarshal(logger, runInfoData, &runInfo)
		if err != nil {
			logger.Error("failed-parsing-run-info", err)
			continue
		}
		metricTags := map[string]*models.MetricTagValue{}
		for k, v := range runInfo.MetricTags {
			metricTags[k] = v
		}
		metricTagsMap[processGuid] = metricTags
	}
	err = rows.Close()
	if err != nil {
		logger.Error("failed-to-close-row", err)
	}

	for pGuid, metricTags := range metricTagsMap {
		updateQuery := "UPDATE desired_lrps SET metric_tags = ? WHERE process_guid = ?"

		mData, err := json.Marshal(metricTags)
		if err != nil {
			logger.Error("failed-marshalling-metric-tags", err)
			continue
		}

		encodedData, err := e.encoder.Encode(mData)
		if err != nil {
			logger.Error("failed-encoding-metric-tags", err)
			continue
		}

		bindings := []interface{}{encodedData, pGuid}
		_, err = tx.Exec(helpers.RebindForFlavor(updateQuery, e.dbFlavor), bindings...)
		if err != nil {
			logger.Error("failed-updating-desired-lrp-record", err)
			return models.ErrBadRequest
		}
	}

	return nil
}
