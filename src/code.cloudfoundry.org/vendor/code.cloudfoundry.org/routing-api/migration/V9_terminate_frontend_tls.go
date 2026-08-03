package migration

import (
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
)

type V9TerminateFrontendTLS struct{}

var _ Migration = new(V9TerminateFrontendTLS)

func NewV9TerminateFrontendTLS() *V9TerminateFrontendTLS {
	return &V9TerminateFrontendTLS{}
}

func (v *V9TerminateFrontendTLS) Version() int {
	return 9
}

func (v *V9TerminateFrontendTLS) Run(sqlDB *db.SqlDB) error {
	// FIXED: Drop index BEFORE AutoMigrate to avoid MySQL error 1170
	// when Gorm v2 tries to change VARCHAR columns to LONGTEXT
	dropIndex(sqlDB, "idx_tcp_route", "tcp_routes")

	// Run AutoMigrate to add the TerminateFrontendTLS column
	err := sqlDB.Client.AutoMigrate(&models.TcpRouteMapping{})
	if err != nil {
		return err
	}

	// Recreate unique index with proper MySQL prefix lengths for LONGTEXT columns
	var indexSQL string
	if sqlDB.Client.Dialect().Name() == "mysql" {
		// MySQL requires prefix lengths for TEXT/LONGTEXT columns in indexes
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid(191), host_port, host_ip(191), external_port, sni_hostname(191), host_tls_port, terminate_frontend_tls)"
	} else {
		// PostgreSQL doesn't require prefix lengths
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid, host_port, host_ip, external_port, sni_hostname, host_tls_port, terminate_frontend_tls)"
	}
	return sqlDB.Client.ExecWithError(indexSQL)
}
