package sqlrunner

import (
	"database/sql"
	"fmt"
	"os"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/go-sql-driver/mysql"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// BBSDBParam is a backward-compatible alias for helpers.ConnectParams
type BBSDBParam = helpers.ConnectParams

// MySQLRunner is responsible for creating and tearing down a test database in
// a local MySQL instance. This runner assumes mysql is already running
// locally, and does not start or stop the mysql service.  mysql must be set up
// on localhost as described in the CONTRIBUTING.md doc in diego-release.
type MySQLRunner struct {
	logger    lager.Logger
	sqlDBName string
	db        *sql.DB
}

func NewMySQLRunner(sqlDBName string) *MySQLRunner {
	return &MySQLRunner{
		logger:    lagertest.NewTestLogger("mysql-runner"),
		sqlDBName: sqlDBName,
	}
}

func (m *MySQLRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	defer ginkgo.GinkgoRecover()
	logger := m.logger.Session("run")
	logger.Info("starting")
	defer logger.Info("completed")

	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		password = "diego_password"
	}
	baseConnString := fmt.Sprintf("%s:%s@/", user, password)

	var err error
	dbParams := &BBSDBParam{
		DriverName:                    "mysql",
		DatabaseConnectionString:      baseConnString,
		SqlCACertFile:                 "",
		SqlEnableIdentityVerification: false,
	}
	m.db, err = helpers.Connect(logger, dbParams)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(m.db.Ping()).To(gomega.Succeed())

	_, err = m.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", m.sqlDBName))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = m.db.Exec(fmt.Sprintf("CREATE DATABASE %s", m.sqlDBName))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	gomega.Expect(m.db.Close()).To(gomega.Succeed())

	connStringWithDB := fmt.Sprintf("%s%s", baseConnString, m.sqlDBName)
	dbParams.DatabaseConnectionString = connStringWithDB
	m.db, err = helpers.Connect(logger, dbParams)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(m.db.Ping()).NotTo(gomega.HaveOccurred())

	close(ready)

	<-signals

	logger.Info("signaled")

	_, err = m.db.Exec(fmt.Sprintf("DROP DATABASE %s", m.sqlDBName))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	logger.Info("closing-connection")
	gomega.Expect(m.db.Close()).To(gomega.Succeed())
	m.db = nil

	return nil
}

func (m *MySQLRunner) ConnectionString() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		password = "diego_password"
	}
	return fmt.Sprintf("%s:%s@/%s", user, password, m.sqlDBName)
}

func (p *MySQLRunner) Port() int {
	return 3306
}

func (p *MySQLRunner) DBName() string {
	return p.sqlDBName
}

func (p *MySQLRunner) Password() string {
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		password = "diego_password"
	}
	return password
}

func (p *MySQLRunner) Username() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}
	return user
}

func (m *MySQLRunner) DriverName() string {
	return "mysql"
}

func (m *MySQLRunner) DB() *sql.DB {
	return m.db
}

func (m *MySQLRunner) ResetTables(tables []string) {
	logger := m.logger.Session("reset-tables")
	logger.Info("starting")
	defer logger.Info("completed")

	for _, name := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s", name)
		result, err := m.db.Exec(query)
		switch err := err.(type) {
		case *mysql.MySQLError:
			if err.Number == 1146 {
				// missing table error, it's fine because we're trying to truncate it
				continue
			}
		}

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(result.RowsAffected()).To(gomega.BeEquivalentTo(0))
	}
}

func (m *MySQLRunner) Reset() {
	m.ResetTables([]string{"domains", "configurations", "tasks", "desired_lrps", "actual_lrps", "locks"})
}
