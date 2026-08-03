package migration

import (
	"code.cloudfoundry.org/routing-api/db"
)

type V4AddRgUniqIdxTCPRoute struct{}

var _ Migration = new(V4AddRgUniqIdxTCPRoute)

func NewV4AddRgUniqIdxTCPRouteMigration() *V4AddRgUniqIdxTCPRoute {
	return &V4AddRgUniqIdxTCPRoute{}
}

func (v *V4AddRgUniqIdxTCPRoute) Version() int {
	return 4
}

func (v *V4AddRgUniqIdxTCPRoute) Run(sqlDB *db.SqlDB) error {
	dropIndex(sqlDB, "idx_tcp_route", "tcp_routes")

	// Create unique index - MySQL requires length prefixes for text columns
	var indexSQL string
	if sqlDB.Client.Dialect().Name() == "mysql" {
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid(191), host_port, host_ip(191), external_port)"
	} else {
		indexSQL = "CREATE UNIQUE INDEX idx_tcp_route ON tcp_routes (router_group_guid, host_port, host_ip, external_port)"
	}
	return sqlDB.Client.ExecWithError(indexSQL)
}
