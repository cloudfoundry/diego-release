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
	appendMigration(NewAddAvailabilityZoneToActualLrps())
}

type AddAvailabilityZoneToActualLrps struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddAvailabilityZoneToActualLrps() migration.Migration {
	return new(AddAvailabilityZoneToActualLrps)
}

func (e *AddAvailabilityZoneToActualLrps) String() string {
	return migrationString(e)
}

func (e *AddAvailabilityZoneToActualLrps) Version() int64 {
	return 1698182853
}

func (e *AddAvailabilityZoneToActualLrps) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddAvailabilityZoneToActualLrps) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddAvailabilityZoneToActualLrps) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddAvailabilityZoneToActualLrps) Up(tx *sql.Tx, logger lager.Logger) error {
	var alterActualLRPAddAvailabilityZoneSQL string
	if e.dbFlavor == "mysql" {
		alterActualLRPAddAvailabilityZoneSQL = `ALTER TABLE actual_lrps
ADD COLUMN availability_zone VARCHAR(255) NOT NULL DEFAULT '';`
	} else {
		alterActualLRPAddAvailabilityZoneSQL = `ALTER TABLE actual_lrps
ADD COLUMN IF NOT EXISTS availability_zone VARCHAR(255) NOT NULL DEFAULT '';`
	}
	logger.Info("altering the table", lager.Data{"query": alterActualLRPAddAvailabilityZoneSQL})
	_, err := tx.Exec(alterActualLRPAddAvailabilityZoneSQL)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterActualLRPAddAvailabilityZoneSQL})

	return nil
}
