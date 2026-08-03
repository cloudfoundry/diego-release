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
	appendMigration(NewAddRejectionReasonToTask())
}

type AddRejectionReasonToTask struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddRejectionReasonToTask() migration.Migration {
	return new(AddRejectionReasonToTask)
}

func (e *AddRejectionReasonToTask) String() string {
	return migrationString(e)
}

func (e *AddRejectionReasonToTask) Version() int64 {
	return 1523050077
}

func (e *AddRejectionReasonToTask) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddRejectionReasonToTask) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddRejectionReasonToTask) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddRejectionReasonToTask) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("add-task-rejection-reason")
	logger.Info("starting")
	defer logger.Info("completed")

	var query string
	if e.dbFlavor == "mysql" {
		query = "ALTER TABLE tasks ADD COLUMN rejection_reason VARCHAR(255) NOT NULL DEFAULT '';"
	} else {
		query = "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS rejection_reason VARCHAR(255) NOT NULL DEFAULT '';"
	}
	_, err := tx.Exec(query)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-table", err)
		return err
	}
	return nil
}
