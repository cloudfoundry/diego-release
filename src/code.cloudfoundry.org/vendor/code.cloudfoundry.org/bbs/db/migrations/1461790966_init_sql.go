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
	appendMigration(NewInitSQL())
}

type InitSQL struct {
	serializer format.Serializer
	clock      clock.Clock
	dbFlavor   string
}

func NewInitSQL() migration.Migration {
	return &InitSQL{}
}

func (e *InitSQL) String() string {
	return migrationString(e)
}

func (e *InitSQL) Version() int64 {
	return 1461790966
}

func (e *InitSQL) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *InitSQL) SetClock(c clock.Clock)    { e.clock = c }
func (e *InitSQL) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *InitSQL) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("init-sql")
	logger.Info("truncating-tables")

	// Ignore the error as the tables may not exist
	_ = e.dropTables(tx)

	err := e.createTables(tx, logger)
	if err != nil {
		return err
	}

	err = e.createIndices(tx, logger)
	if err != nil {
		return err
	}

	return nil
}

func (e *InitSQL) dropTables(tx *sql.Tx) error {
	tableNames := []string{
		"domains",
		"tasks",
		"desired_lrps",
		"actual_lrps",
	}
	for _, tableName := range tableNames {
		_, err := tx.Exec("DROP TABLE IF EXISTS " + tableName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *InitSQL) createTables(tx *sql.Tx, logger lager.Logger) error {
	var createTablesSQL = []string{
		helpers.RebindForFlavor(createDomainSQL, e.dbFlavor),
		helpers.RebindForFlavor(createDesiredLRPsSQL, e.dbFlavor),
		helpers.RebindForFlavor(createActualLRPsSQL, e.dbFlavor),
		helpers.RebindForFlavor(createTasksSQL, e.dbFlavor),
	}

	logger.Info("creating-tables")
	for _, query := range createTablesSQL {
		logger.Info("creating the table", lager.Data{"query": query})
		_, err := tx.Exec(query)
		if err != nil {
			logger.Error("failed-creating-tables", err)
			return err
		}
		logger.Info("created the table", lager.Data{"query": query})
	}

	return nil
}

func (e *InitSQL) createIndices(tx *sql.Tx, logger lager.Logger) error {
	logger.Info("creating-indices")
	createIndicesSQL := []string{}
	createIndicesSQL = append(createIndicesSQL, createDomainsIndices...)
	createIndicesSQL = append(createIndicesSQL, createDesiredLRPsIndices...)
	createIndicesSQL = append(createIndicesSQL, createActualLRPsIndices...)
	createIndicesSQL = append(createIndicesSQL, createTasksIndices...)

	for _, query := range createIndicesSQL {
		logger.Info("creating the index", lager.Data{"query": query})
		_, err := tx.Exec(query)
		if err != nil {
			logger.Error("failed-creating-index", err)
			return err
		}
		logger.Info("created the index", lager.Data{"query": query})
	}

	return nil
}

const createDomainSQL = `CREATE TABLE domains(
	domain VARCHAR(255) PRIMARY KEY,
	expire_time BIGINT DEFAULT 0
);`

const createDesiredLRPsSQL = `CREATE TABLE desired_lrps(
	process_guid VARCHAR(255) PRIMARY KEY,
	domain VARCHAR(255) NOT NULL,
	log_guid VARCHAR(255) NOT NULL,
	annotation MEDIUMTEXT,
	instances INT NOT NULL,
	memory_mb INT NOT NULL,
	disk_mb INT NOT NULL,
	rootfs VARCHAR(255) NOT NULL,
	routes MEDIUMTEXT NOT NULL,
	volume_placement MEDIUMTEXT NOT NULL,
	modification_tag_epoch VARCHAR(255) NOT NULL,
	modification_tag_index INT,
	run_info MEDIUMTEXT NOT NULL
);`

const createActualLRPsSQL = `CREATE TABLE actual_lrps(
	process_guid VARCHAR(255),
	instance_index INT,
	evacuating BOOL DEFAULT false,
	domain VARCHAR(255) NOT NULL,
	state VARCHAR(255) NOT NULL,
	instance_guid VARCHAR(255) NOT NULL DEFAULT '',
	cell_id VARCHAR(255) NOT NULL DEFAULT '',
	placement_error VARCHAR(255) NOT NULL DEFAULT '',
	since BIGINT DEFAULT 0,
	net_info MEDIUMTEXT NOT NULL,
	modification_tag_epoch VARCHAR(255) NOT NULL,
	modification_tag_index INT,
	crash_count INT NOT NULL DEFAULT 0,
	crash_reason VARCHAR(255) NOT NULL DEFAULT '',
	expire_time BIGINT DEFAULT 0,

	PRIMARY KEY(process_guid, instance_index, evacuating)
);`

const createTasksSQL = `CREATE TABLE tasks(
	guid VARCHAR(255) PRIMARY KEY,
	domain VARCHAR(255) NOT NULL,
	updated_at BIGINT DEFAULT 0,
	created_at BIGINT DEFAULT 0,
	first_completed_at BIGINT DEFAULT 0,
	state INT,
	cell_id VARCHAR(255) NOT NULL DEFAULT '',
	result MEDIUMTEXT,
	failed BOOL DEFAULT false,
	failure_reason VARCHAR(255) NOT NULL DEFAULT '',
	task_definition MEDIUMTEXT NOT NULL
);`

var createDomainsIndices = []string{
	`CREATE INDEX domains_expire_time_idx ON domains (expire_time)`,
}

var createDesiredLRPsIndices = []string{
	`CREATE INDEX desired_lrps_domain_idx ON desired_lrps (domain)`,
}

var createActualLRPsIndices = []string{
	`CREATE INDEX actual_lrps_domain_idx ON actual_lrps (domain)`,
	`CREATE INDEX actual_lrps_cell_id_idx ON actual_lrps (cell_id)`,
	`CREATE INDEX actual_lrps_since_idx ON actual_lrps (since)`,
	`CREATE INDEX actual_lrps_state_idx ON actual_lrps (state)`,
	`CREATE INDEX actual_lrps_expire_time_idx ON actual_lrps (expire_time)`,
}

var createTasksIndices = []string{
	`CREATE INDEX tasks_domain_idx ON tasks (domain)`,
	`CREATE INDEX tasks_state_idx ON tasks (state)`,
	`CREATE INDEX tasks_cell_id_idx ON tasks (cell_id)`,
	`CREATE INDEX tasks_updated_at_idx ON tasks (updated_at)`,
	`CREATE INDEX tasks_created_at_idx ON tasks (created_at)`,
	`CREATE INDEX tasks_first_completed_at_idx ON tasks (first_completed_at)`,
}
