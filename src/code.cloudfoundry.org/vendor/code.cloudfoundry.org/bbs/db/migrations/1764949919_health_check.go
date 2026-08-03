package migrations

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func init() {
	appendMigration(NewBBSHealthCheckTable())
}

type BBSHealthCheckTable struct {
	encoder    format.Encoder
	serializer format.Serializer
	clock      clock.Clock
	rawSQLDB   *sql.DB
	dbFlavor   string
}

func NewBBSHealthCheckTable() migration.Migration {
	return new(BBSHealthCheckTable)
}

func (e *BBSHealthCheckTable) String() string {
	return migrationString(e)
}

func (e *BBSHealthCheckTable) Version() int64 {
	return 1764949919
}

func (e *BBSHealthCheckTable) SetCryptor(cryptor encryption.Cryptor) {
	e.encoder = format.NewEncoder(cryptor)
	e.serializer = format.NewSerializer(cryptor)
}

func (e *BBSHealthCheckTable) SetRawSQLDB(db *sql.DB)    { e.rawSQLDB = db }
func (e *BBSHealthCheckTable) SetClock(c clock.Clock)    { e.clock = c }
func (e *BBSHealthCheckTable) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *BBSHealthCheckTable) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("bbs-health-check-table")
	logger.Info("starting")
	defer logger.Info("completed")

	var addTableSQL string
	if e.dbFlavor == "mysql" {
		addTableSQL = "CREATE TABLE IF NOT EXISTS bbs_health_check (id int NOT NULL AUTO_INCREMENT, PRIMARY KEY (id), time bigint NOT NULL)"
	} else {
		addTableSQL = "CREATE TABLE IF NOT EXISTS bbs_health_check (id SERIAL PRIMARY KEY, time bigint NOT NULL)"
	}

	logger.Info("creating-table")
	_, err := tx.Exec(helpers.RebindForFlavor(addTableSQL, e.dbFlavor))
	if err != nil {
		logger.Error("failed-creating-table", err)
		return err
	}

	return nil
}
