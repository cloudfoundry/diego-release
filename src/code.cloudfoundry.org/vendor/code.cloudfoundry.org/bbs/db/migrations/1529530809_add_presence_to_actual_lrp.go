package migrations

import (
	"database/sql"
	"fmt"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func init() {
	appendMigration(NewAddPresenceToActualLrp())
}

type AddPresenceToActualLrp struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewAddPresenceToActualLrp() migration.Migration {
	return new(AddPresenceToActualLrp)
}

func (e *AddPresenceToActualLrp) String() string {
	return migrationString(e)
}

func (e *AddPresenceToActualLrp) Version() int64 {
	return 1529530809
}

func (e *AddPresenceToActualLrp) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddPresenceToActualLrp) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddPresenceToActualLrp) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddPresenceToActualLrp) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("add-presence")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTable(tx, logger)
}

func (e *AddPresenceToActualLrp) alterTable(tx *sql.Tx, logger lager.Logger) error {
	var addColumnSQL string
	if e.dbFlavor == "mysql" {
		addColumnSQL = "ALTER TABLE actual_lrps ADD COLUMN presence INT NOT NULL DEFAULT 0;"
	} else {
		addColumnSQL = "ALTER TABLE actual_lrps ADD COLUMN IF NOT EXISTS presence INT NOT NULL DEFAULT 0;"
	}

	logger.Info("altering-table")
	_, err := tx.Exec(helpers.RebindForFlavor(addColumnSQL, e.dbFlavor))
	if err != nil && !isDuplicateColumnError(err) {
		logger.Error("failed-altering-table", err)
		return err
	}

	alterTablesSQL := []string{}
	alterTablesSQL = append(alterTablesSQL, fmt.Sprintf("UPDATE actual_lrps SET presence = %d WHERE evacuating = true;", models.ActualLRP_Evacuating))

	if e.dbFlavor == "mysql" {
		alterTablesSQL = append(alterTablesSQL,
			"ALTER TABLE actual_lrps DROP primary key, ADD PRIMARY KEY (process_guid, instance_index, presence);",
		)
	} else {
		alterTablesSQL = append(alterTablesSQL,
			"ALTER TABLE actual_lrps DROP CONSTRAINT actual_lrps_pkey, ADD PRIMARY KEY (process_guid, instance_index, presence);",
		)
	}

	for _, query := range alterTablesSQL {
		logger.Info("altering the table", lager.Data{"query": query})
		_, err := tx.Exec(helpers.RebindForFlavor(query, e.dbFlavor))
		if err != nil {
			logger.Error("failed-altering-table", err)
			return err
		}
		logger.Info("altered the table", lager.Data{"query": query})
	}

	return nil
}
