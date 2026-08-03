package migration

import (
	"code.cloudfoundry.org/routing-api/db"
)

type V7TCPTLSRoutes struct{}

func NewV7TCPTLSRoutes() *V7TCPTLSRoutes {
	return &V7TCPTLSRoutes{}
}

func (v *V7TCPTLSRoutes) Version() int {
	return 7
}

func (v *V7TCPTLSRoutes) Run(sqlDB *db.SqlDB) error {
	// Update the instance_id column to allow NULL values - syntax differs by database
	if sqlDB.Client.Dialect().Name() == "mysql" {
		err := sqlDB.Client.ExecWithError("ALTER TABLE tcp_routes MODIFY COLUMN instance_id varchar(255) DEFAULT NULL")
		if err != nil {
			return err
		}
	} else {
		err := sqlDB.Client.ExecWithError("ALTER TABLE tcp_routes ALTER COLUMN instance_id SET DEFAULT NULL")
		if err != nil {
			return err
		}
	}

	dropIndex(sqlDB, "idx_tcp_route", "tcp_routes")

	// Create unique index - MySQL requires length prefixes for text columns
	var indexSQL string
	if sqlDB.Client.Dialect().Name() == "mysql" {
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid(191), host_port, host_ip(191), external_port, sni_hostname(191), host_tls_port)"
	} else {
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid, host_port, host_ip, external_port, sni_hostname, host_tls_port)"
	}
	return sqlDB.Client.ExecWithError(indexSQL)
}
