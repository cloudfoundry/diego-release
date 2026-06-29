package migrations

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
)

func init() {
	appendMigration(NewIncreaseRunInfoColumnSize())
}

type IncreaseRunInfoColumnSize struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewIncreaseRunInfoColumnSize() migration.Migration {
	return &IncreaseRunInfoColumnSize{}
}

func (e *IncreaseRunInfoColumnSize) String() string {
	return migrationString(e)
}

func (e *IncreaseRunInfoColumnSize) Version() int64 {
	return 1471030898
}

func (e *IncreaseRunInfoColumnSize) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *IncreaseRunInfoColumnSize) SetClock(c clock.Clock)    { e.clock = c }
func (e *IncreaseRunInfoColumnSize) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *IncreaseRunInfoColumnSize) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("increase-run-info-column")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTables(tx, logger)
}

func (e *IncreaseRunInfoColumnSize) alterTables(tx *sql.Tx, logger lager.Logger) error {
	if e.dbFlavor != "mysql" {
		return nil
	}

	var alterTablesSQL = []string{
		alterDesiredLRPsSQL,
		alterActualLRPsSQL,
		alterTasksSQL,
	}

	logger.Info("altering-tables")
	for _, query := range alterTablesSQL {
		logger.Info("altering the table", lager.Data{"query": query})
		_, err := tx.Exec(query)
		if err != nil {
			logger.Error("failed-altering-tables", err)
			return err
		}
		logger.Info("altered the table", lager.Data{"query": query})
	}

	return nil
}

const alterDesiredLRPsSQL = `ALTER TABLE desired_lrps
	MODIFY annotation MEDIUMTEXT,
	MODIFY routes MEDIUMTEXT NOT NULL,
	MODIFY volume_placement MEDIUMTEXT NOT NULL,
	MODIFY run_info MEDIUMTEXT NOT NULL;`

const alterActualLRPsSQL = `ALTER TABLE actual_lrps
	MODIFY net_info MEDIUMTEXT NOT NULL;`

const alterTasksSQL = `ALTER TABLE tasks
	MODIFY result MEDIUMTEXT,
	MODIFY task_definition MEDIUMTEXT NOT NULL;`
