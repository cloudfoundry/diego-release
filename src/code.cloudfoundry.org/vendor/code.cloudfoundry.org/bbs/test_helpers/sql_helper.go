package test_helpers

import (
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/diego-db-helpers/testhelpers/sqlrunner"
)

const (
	mysqlFlavor    = "mysql"
	mysql8Flavor   = "mysql8"
	postgresFlavor = "postgres"
)

func UseSQL() bool {
	return true
}

func driver() string {
	flavor := os.Getenv("DB")
	if flavor == "" {
		flavor = postgresFlavor
	}
	return flavor
}

func UseMySQL() bool {
	return driver() == mysqlFlavor || driver() == mysql8Flavor
}

func UsePostgres() bool {
	return driver() == postgresFlavor
}

func NewSQLRunner(dbName string) sqlrunner.SQLRunner {
	var sqlRunner sqlrunner.SQLRunner

	if UseMySQL() {
		sqlRunner = sqlrunner.NewMySQLRunner(dbName)
	} else if UsePostgres() {
		sqlRunner = sqlrunner.NewPostgresRunner(dbName)
	} else {
		panic(fmt.Sprintf("driver '%s' is not supported", driver()))
	}

	return sqlRunner
}

func ReplaceQuestionMarks(queryString string) string {
	strParts := strings.Split(queryString, "?")
	for i := 1; i < len(strParts); i++ {
		strParts[i-1] = fmt.Sprintf("%s$%d", strParts[i-1], i)
	}
	return strings.Join(strParts, "")
}
