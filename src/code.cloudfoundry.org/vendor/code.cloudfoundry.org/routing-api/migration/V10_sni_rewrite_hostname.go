package migration

import (
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
)

type V10SniRewriteHostname struct{}

var _ Migration = new(V10SniRewriteHostname)

func NewV10SniRewriteHostname() *V10SniRewriteHostname {
	return &V10SniRewriteHostname{}
}

func (v *V10SniRewriteHostname) Version() int {
	return 10
}

func (v *V10SniRewriteHostname) Run(sqlDB *db.SqlDB) error {
	// CRITICAL FIX: Drop index BEFORE AutoMigrate to avoid MySQL error 1170
	// when Gorm v2 tries to change VARCHAR columns to LONGTEXT
	dropIndex(sqlDB, "idx_tcp_route", "tcp_routes")

	// Run AutoMigrate to add the SniRewriteHostname column
	err := sqlDB.Client.AutoMigrate(&models.TcpRouteMapping{})
	if err != nil {
		return err
	}

	// Recreate unique index with proper MySQL prefix lengths for LONGTEXT columns
	// Note: SniRewriteHostname is NOT part of the unique index (no unique_index tag in model)
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
