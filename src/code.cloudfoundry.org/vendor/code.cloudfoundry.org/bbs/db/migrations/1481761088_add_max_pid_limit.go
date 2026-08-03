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
	appendMigration(NewAddMaxPidsToDesiredLRPs())
}

type AddMaxPidsToDesiredLRPs struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddMaxPidsToDesiredLRPs() migration.Migration {
	return &AddMaxPidsToDesiredLRPs{}
}

func (e *AddMaxPidsToDesiredLRPs) String() string {
	return migrationString(e)
}

func (e *AddMaxPidsToDesiredLRPs) Version() int64 {
	return 1481761088
}

func (e *AddMaxPidsToDesiredLRPs) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddMaxPidsToDesiredLRPs) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddMaxPidsToDesiredLRPs) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddMaxPidsToDesiredLRPs) Up(tx *sql.Tx, logger lager.Logger) error {
	var alterDesiredLRPAddMaxPidsSQL string
	if e.dbFlavor == "mysql" {
		alterDesiredLRPAddMaxPidsSQL = `ALTER TABLE desired_lrps
	ADD COLUMN max_pids INTEGER DEFAULT 0;`
	} else {
		alterDesiredLRPAddMaxPidsSQL = `ALTER TABLE desired_lrps
	ADD COLUMN IF NOT EXISTS max_pids INTEGER DEFAULT 0;`
	}
	logger.Info("altering the table", lager.Data{"query": alterDesiredLRPAddMaxPidsSQL})
	_, err := tx.Exec(alterDesiredLRPAddMaxPidsSQL)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterDesiredLRPAddMaxPidsSQL})

	return nil
}
