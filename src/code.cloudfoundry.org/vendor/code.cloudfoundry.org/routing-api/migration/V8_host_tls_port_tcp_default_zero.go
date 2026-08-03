package migration

import (
	"code.cloudfoundry.org/routing-api/db"
)

type V8HostTLSPortTCPDefaultZero struct{}

func NewV8HostTLSPortTCPDefaultZero() *V8HostTLSPortTCPDefaultZero {
	return &V8HostTLSPortTCPDefaultZero{}
}

func (v *V8HostTLSPortTCPDefaultZero) Version() int {
	return 8
}

func (v *V8HostTLSPortTCPDefaultZero) Run(sqlDB *db.SqlDB) error {
	// Update existing rows where host_tls_port is NULL to 0
	err := sqlDB.Client.ExecWithError("UPDATE tcp_routes SET host_tls_port = 0 WHERE host_tls_port IS NULL")
	if err != nil {
		return err
	}

	// Try to remove the old index if it exists
	dropIndex(sqlDB, "idx_tcp_route", "tcp_routes")

	// Set the DEFAULT 0 on the host_tls_port column - syntax differs by database
	if sqlDB.Client.Dialect().Name() == "mysql" {
		err = sqlDB.Client.ExecWithError("ALTER TABLE tcp_routes MODIFY COLUMN host_tls_port int DEFAULT 0")
	} else {
		err = sqlDB.Client.ExecWithError("ALTER TABLE tcp_routes ALTER COLUMN host_tls_port SET DEFAULT 0")
	}
	return err
}
