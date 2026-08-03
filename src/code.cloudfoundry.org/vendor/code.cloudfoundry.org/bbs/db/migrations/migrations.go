package migrations

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"code.cloudfoundry.org/bbs/migration"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

var migrationsRegistry = migration.Migrations{}

func appendMigration(migrationTemplate migration.Migration) {
	migrationsRegistry = append(migrationsRegistry, migrationTemplate)
}

func migrationString(m migration.Migration) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return strconv.FormatInt(m.Version(), 10)
	}
	return strings.Split(filepath.Base(filename), ".")[0]
}

func AllMigrations() migration.Migrations {
	migs := make(migration.Migrations, len(migrationsRegistry))
	for i, mig := range migrationsRegistry {
		rt := reflect.TypeOf(mig)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		migs[i] = reflect.New(rt).Interface().(migration.Migration)
	}
	return migs
}

func isDuplicateColumnError(err error) bool {
	switch e := err.(type) {
	case *mysql.MySQLError:
		if e.Number == 1060 {
			return true
		}
	case *pgconn.PgError:
		if e.Code == "42701" {
			return true
		}
	}

	return false
}
