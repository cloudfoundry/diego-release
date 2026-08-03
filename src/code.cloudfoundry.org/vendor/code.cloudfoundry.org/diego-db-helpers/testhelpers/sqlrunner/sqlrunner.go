package sqlrunner

import (
	"database/sql"

	"github.com/tedsuo/ifrit"
)

type SQLRunner interface {
	ifrit.Runner
	ConnectionString() string
	// Reset truncates all tables; callers should call ResetTables with the
	// schema-specific table list rather than relying on this no-op default.
	Reset()
	ResetTables(tables []string)
	DriverName() string
	Port() int
	DBName() string
	Username() string
	Password() string
	DB() *sql.DB
}
