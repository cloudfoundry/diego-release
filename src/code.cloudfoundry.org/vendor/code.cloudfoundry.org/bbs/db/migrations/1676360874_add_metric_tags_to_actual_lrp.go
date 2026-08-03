package migrations

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func init() {
	appendMigration(NewAddMetricTagsToActualLrp())
}

type AddMetricTagsToActualLrp struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddMetricTagsToActualLrp() migration.Migration {
	return new(AddMetricTagsToActualLrp)
}

func (e *AddMetricTagsToActualLrp) String() string {
	return migrationString(e)
}

func (e *AddMetricTagsToActualLrp) Version() int64 {
	return 1676360874
}

func (e *AddMetricTagsToActualLrp) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddMetricTagsToActualLrp) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddMetricTagsToActualLrp) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddMetricTagsToActualLrp) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("add-metric-tags")
	logger.Info("starting")
	defer logger.Info("completed")

	var alterTableSQL string
	if e.dbFlavor == "mysql" {
		alterTableSQL = "ALTER TABLE actual_lrps ADD COLUMN metric_tags MEDIUMTEXT;"
	} else {
		alterTableSQL = "ALTER TABLE actual_lrps ADD COLUMN IF NOT EXISTS metric_tags MEDIUMTEXT;"
	}
	logger.Info("altering the table", lager.Data{"query": alterTableSQL})
	_, err := tx.Exec(helpers.RebindForFlavor(alterTableSQL, e.dbFlavor))
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-table", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterTableSQL})

	return nil
}
