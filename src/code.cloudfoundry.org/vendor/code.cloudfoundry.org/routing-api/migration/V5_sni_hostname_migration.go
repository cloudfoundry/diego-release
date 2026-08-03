package migration

import (
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
)

type V5SniHostnameMigration struct{}

var _ Migration = new(V5SniHostnameMigration)

func NewV5SniHostnameMigration() *V5SniHostnameMigration {
	return &V5SniHostnameMigration{}
}

func (v *V5SniHostnameMigration) Version() int {
	return 5
}

func (v *V5SniHostnameMigration) Run(sqlDB *db.SqlDB) error {
	// FIXED: Drop index BEFORE AutoMigrate to avoid MySQL error 1170
	// when Gorm v2 tries to change VARCHAR columns to LONGTEXT
	dropIndex(sqlDB, "idx_tcp_route", "tcp_routes")

	// Run AutoMigrate to add the SniHostname column
	err := sqlDB.Client.AutoMigrate(&models.TcpRouteMapping{})
	if err != nil {
		return err
	}

	// Recreate unique index with proper MySQL prefix lengths for LONGTEXT columns
	var indexSQL string
	if sqlDB.Client.Dialect().Name() == "mysql" {
		// MySQL requires prefix lengths for TEXT/LONGTEXT columns in indexes
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid(191), host_port, host_ip(191), external_port, sni_hostname(191))"
	} else {
		// PostgreSQL doesn't require prefix lengths
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid, host_port, host_ip, external_port, sni_hostname)"
	}
	return sqlDB.Client.ExecWithError(indexSQL)
}
