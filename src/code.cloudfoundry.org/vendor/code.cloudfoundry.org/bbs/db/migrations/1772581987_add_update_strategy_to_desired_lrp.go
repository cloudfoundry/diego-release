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
	appendMigration(NewAddUpdateStrategyToDesiredLRPs())
}

type AddUpdateStrategyToDesiredLRPs struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddUpdateStrategyToDesiredLRPs() migration.Migration {
	return &AddUpdateStrategyToDesiredLRPs{}
}

func (e *AddUpdateStrategyToDesiredLRPs) String() string {
	return migrationString(e)
}

func (e *AddUpdateStrategyToDesiredLRPs) Version() int64 {
	return 1772581987
}

func (e *AddUpdateStrategyToDesiredLRPs) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddUpdateStrategyToDesiredLRPs) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddUpdateStrategyToDesiredLRPs) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddUpdateStrategyToDesiredLRPs) Up(tx *sql.Tx, logger lager.Logger) error {
	var alterDesiredLRPAddUpdateStrategySQL string
	if e.dbFlavor == "mysql" {
		alterDesiredLRPAddUpdateStrategySQL = `ALTER TABLE desired_lrps
	ADD COLUMN update_strategy INT NOT NULL DEFAULT 0;`
	} else {
		alterDesiredLRPAddUpdateStrategySQL = `ALTER TABLE desired_lrps
	ADD COLUMN IF NOT EXISTS update_strategy INT NOT NULL DEFAULT 0;`
	}
	logger.Info("altering the table", lager.Data{"query": alterDesiredLRPAddUpdateStrategySQL})
	_, err := tx.Exec(alterDesiredLRPAddUpdateStrategySQL)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterDesiredLRPAddUpdateStrategySQL})

	return nil
}
