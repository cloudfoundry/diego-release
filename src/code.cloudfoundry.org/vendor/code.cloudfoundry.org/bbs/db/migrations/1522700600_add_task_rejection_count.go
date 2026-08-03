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
	appendMigration(NewAddTaskRejectionCount())
}

type AddTaskRejectionCount struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddTaskRejectionCount() migration.Migration {
	return new(AddTaskRejectionCount)
}

func (e *AddTaskRejectionCount) String() string {
	return migrationString(e)
}

func (e *AddTaskRejectionCount) Version() int64 {
	return 1522700600
}

func (e *AddTaskRejectionCount) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddTaskRejectionCount) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddTaskRejectionCount) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddTaskRejectionCount) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("add-task-rejection-count")
	logger.Info("starting")
	defer logger.Info("completed")

	var stmt string
	if e.dbFlavor == "mysql" {
		stmt = "ALTER TABLE tasks ADD COLUMN rejection_count INTEGER NOT NULL DEFAULT 0;"
	} else {
		stmt = "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS rejection_count INTEGER NOT NULL DEFAULT 0;"
	}
	_, err := tx.Exec(stmt)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-table", err)
		return err
	}
	return nil
}
