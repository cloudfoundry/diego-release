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
	appendMigration(NewIncreaseTaskErrorColumns())
}

type IncreaseTaskErrorColumns struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewIncreaseTaskErrorColumns() migration.Migration {
	return new(IncreaseTaskErrorColumns)
}

func (e *IncreaseTaskErrorColumns) String() string {
	return migrationString(e)
}

func (e *IncreaseTaskErrorColumns) Version() int64 {
	return 1528928964
}

func (e *IncreaseTaskErrorColumns) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *IncreaseTaskErrorColumns) SetClock(c clock.Clock)    { e.clock = c }
func (e *IncreaseTaskErrorColumns) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *IncreaseTaskErrorColumns) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("increase-failure-reason-column")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTables(tx, logger)
}

func (e *IncreaseTaskErrorColumns) alterTables(tx *sql.Tx, logger lager.Logger) error {
	var alterTaskTableSQL string

	if e.dbFlavor == "mysql" {
		alterTaskTableSQL = `ALTER TABLE tasks
	MODIFY rejection_reason VARCHAR(1024) NOT NULL DEFAULT '',
	MODIFY failure_reason VARCHAR(1024) NOT NULL DEFAULT ''`

	} else {
		alterTaskTableSQL = `ALTER TABLE tasks
	ALTER rejection_reason TYPE VARCHAR(1024),
	ALTER failure_reason TYPE VARCHAR(1024)`
	}

	logger.Info("altering-tables")
	logger.Info("altering the table", lager.Data{"query": alterTaskTableSQL})
	_, err := tx.Exec(alterTaskTableSQL)
	if err != nil {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterTaskTableSQL})

	return nil
}
