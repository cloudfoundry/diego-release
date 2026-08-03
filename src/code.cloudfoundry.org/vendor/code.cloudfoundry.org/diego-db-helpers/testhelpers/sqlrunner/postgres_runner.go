package sqlrunner

import (
	"database/sql"
	"fmt"
	"os"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/jackc/pgx/v5/pgconn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// PostgresRunner is responsible for creating and tearing down a test database in
// a local Postgres instance. This runner assumes mysql is already running
// locally, and does not start or stop the mysql service.  mysql must be set up
// on localhost as described in the CONTRIBUTING.md doc in diego-release.
type PostgresRunner struct {
	logger    lager.Logger
	db        *sql.DB
	sqlDBName string
}

func NewPostgresRunner(sqlDBName string) *PostgresRunner {
	return &PostgresRunner{
		logger:    lagertest.NewTestLogger("postgres-runner"),
		sqlDBName: sqlDBName,
	}
}

func (p *PostgresRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	defer GinkgoRecover()
	logger := p.logger.Session("run")
	logger.Info("starting")
	defer logger.Info("completed")

	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		password = "diego_pw"
	}
	baseConnString := fmt.Sprintf("postgres://%s:%s@localhost/", user, password)

	var err error
	dbParams := &helpers.ConnectParams{
		DriverName:                    "postgres",
		DatabaseConnectionString:      baseConnString,
		SqlCACertFile:                 "",
		SqlEnableIdentityVerification: false,
	}
	p.db, err = helpers.Connect(logger, dbParams)
	Expect(err).NotTo(HaveOccurred())
	Expect(p.db.Ping()).To(Succeed())

	_, err = p.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())

	_, err = p.db.Exec(fmt.Sprintf("CREATE DATABASE %s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())

	Expect(p.db.Close()).To(Succeed())

	connStringWithDB := fmt.Sprintf("%s/%s", baseConnString, p.sqlDBName)
	dbParams = &helpers.ConnectParams{
		DriverName:                    "postgres",
		DatabaseConnectionString:      connStringWithDB,
		SqlCACertFile:                 "",
		SqlEnableIdentityVerification: false,
	}
	p.db, err = helpers.Connect(logger, dbParams)
	Expect(err).NotTo(HaveOccurred())
	Expect(p.db.Ping()).To(Succeed())

	close(ready)

	<-signals

	logger.Info("signaled")

	// We need to close the connection to the database we want to drop before dropping it.
	Expect(p.db.Close()).To(Succeed())

	logger.Info("openning-connection-to-database")
	dbParams = &helpers.ConnectParams{
		DriverName:                    "postgres",
		DatabaseConnectionString:      baseConnString,
		SqlCACertFile:                 "",
		SqlEnableIdentityVerification: false,
	}
	p.db, err = helpers.Connect(logger, dbParams)
	Expect(err).NotTo(HaveOccurred())

	logger.Info("dropping-database")
	_, err = p.db.Exec(fmt.Sprintf("DROP DATABASE %s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())

	logger.Info("closing-connection")
	Expect(p.db.Close()).To(Succeed())

	return nil
}

func (p *PostgresRunner) ConnectionString() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		password = "diego_pw"
	}
	return fmt.Sprintf("user=%s password=%s host=localhost dbname=%s", user, password, p.sqlDBName)
}

func (p *PostgresRunner) Port() int {
	return 5432
}

func (p *PostgresRunner) DBName() string {
	return p.sqlDBName
}

func (p *PostgresRunner) DriverName() string {
	return "postgres"
}

func (p *PostgresRunner) Password() string {
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		password = "diego_pw"
	}
	return password
}

func (p *PostgresRunner) Username() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}
	return user
}

func (p *PostgresRunner) DB() *sql.DB {
	return p.db
}

func (p *PostgresRunner) ResetTables(tables []string) {
	logger := p.logger.Session("reset-tables")
	logger.Info("starting")
	defer logger.Info("completed")

	for _, name := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s", name)
		result, err := p.db.Exec(query)

		switch err := err.(type) {
		case *pgconn.PgError:
			if err.Code == "42P01" {
				// missing table error, it's fine because we're trying to truncate it
				continue
			}
		}

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RowsAffected()).To(BeEquivalentTo(0))
	}
}

func (p *PostgresRunner) Reset() {
}
