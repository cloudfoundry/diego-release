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
	appendMigration(NewSetRunInfoLongtext())
}

type SetRunInfoLongtext struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewSetRunInfoLongtext() migration.Migration {
	return &SetRunInfoLongtext{}
}

func (e *SetRunInfoLongtext) String() string {
	return migrationString(e)
}

func (e *SetRunInfoLongtext) Version() int64 {
	return 1674146125
}

func (e *SetRunInfoLongtext) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *SetRunInfoLongtext) SetClock(c clock.Clock)    { e.clock = c }
func (e *SetRunInfoLongtext) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *SetRunInfoLongtext) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("set-run-info-longtext")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTables(tx, logger)
}

func (e *SetRunInfoLongtext) alterTables(tx *sql.Tx, logger lager.Logger) error {
	if e.dbFlavor != "mysql" {
		return nil
	}

	alterDesiredLRPsSQL := `ALTER TABLE desired_lrps
	MODIFY annotation LONGTEXT,
	MODIFY routes LONGTEXT NOT NULL,
	MODIFY volume_placement LONGTEXT NOT NULL,
	MODIFY run_info LONGTEXT NOT NULL;`

	alterActualLRPsSQL := `ALTER TABLE actual_lrps
	MODIFY net_info LONGTEXT NOT NULL;`

	alterTasksSQL := `ALTER TABLE tasks
	MODIFY result LONGTEXT,
	MODIFY task_definition LONGTEXT NOT NULL;`

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
