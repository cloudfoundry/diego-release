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
	appendMigration(NewAddRoutableToActualLrps())
}

type AddRoutableToActualLrps struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddRoutableToActualLrps() migration.Migration {
	return new(AddRoutableToActualLrps)
}

func (e *AddRoutableToActualLrps) String() string {
	return migrationString(e)
}

func (e *AddRoutableToActualLrps) Version() int64 {
	return 1686692176
}

func (e *AddRoutableToActualLrps) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddRoutableToActualLrps) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddRoutableToActualLrps) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddRoutableToActualLrps) Up(tx *sql.Tx, logger lager.Logger) error {
	var alterActualLRPAddRoutableSQL string
	if e.dbFlavor == "mysql" {
		alterActualLRPAddRoutableSQL = `ALTER TABLE actual_lrps
ADD COLUMN routable BOOL DEFAULT false;`
	} else {
		alterActualLRPAddRoutableSQL = `ALTER TABLE actual_lrps
ADD COLUMN IF NOT EXISTS routable BOOL DEFAULT false;`
	}

	logger.Info("altering the table", lager.Data{"query": alterActualLRPAddRoutableSQL})
	_, err := tx.Exec(alterActualLRPAddRoutableSQL)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-table", err)
		return err
	}

	logger.Info("altered the table", lager.Data{"query": alterActualLRPAddRoutableSQL})

	logger.Info("altering the table", lager.Data{"query": alterActualLRPSetRoutableForRunningSQL})
	_, err = tx.Exec(alterActualLRPSetRoutableForRunningSQL)
	if err != nil {
		logger.Error("failed-altering-table", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterActualLRPSetRoutableForRunningSQL})

	return nil
}

const alterActualLRPSetRoutableForRunningSQL = `UPDATE actual_lrps
SET routable = true WHERE state = 'RUNNING';`
