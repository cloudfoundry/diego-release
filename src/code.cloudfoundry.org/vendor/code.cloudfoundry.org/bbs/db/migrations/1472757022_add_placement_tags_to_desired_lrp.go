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
	appendMigration(NewAddPlacementTagsToDesiredLRPs())
}

type AddPlacementTagsToDesiredLRPs struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddPlacementTagsToDesiredLRPs() migration.Migration {
	return &AddPlacementTagsToDesiredLRPs{}
}

func (e *AddPlacementTagsToDesiredLRPs) String() string {
	return migrationString(e)
}

func (e *AddPlacementTagsToDesiredLRPs) Version() int64 {
	return 1472757022
}

func (e *AddPlacementTagsToDesiredLRPs) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddPlacementTagsToDesiredLRPs) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddPlacementTagsToDesiredLRPs) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddPlacementTagsToDesiredLRPs) Up(tx *sql.Tx, logger lager.Logger) error {
	var alterDesiredLRPAddPlacementTagSQL string
	if e.dbFlavor == "mysql" {
		alterDesiredLRPAddPlacementTagSQL = `ALTER TABLE desired_lrps
	ADD COLUMN placement_tags TEXT;`
	} else {
		alterDesiredLRPAddPlacementTagSQL = `ALTER TABLE desired_lrps
	ADD COLUMN IF NOT EXISTS placement_tags TEXT;`
	}
	logger.Info("altering the table", lager.Data{"query": alterDesiredLRPAddPlacementTagSQL})
	_, err := tx.Exec(alterDesiredLRPAddPlacementTagSQL)
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterDesiredLRPAddPlacementTagSQL})

	return nil
}
