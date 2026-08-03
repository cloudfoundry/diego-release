package migration

import (
	"fmt"

	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
)

type V2UpdateRgMigration struct{}

var _ Migration = new(V2UpdateRgMigration)

func NewV2UpdateRgMigration() *V2UpdateRgMigration {
	return &V2UpdateRgMigration{}
}

func (v *V2UpdateRgMigration) Version() int {
	return 2
}

func (v *V2UpdateRgMigration) Run(sqlDB *db.SqlDB) error {
	// Check for duplicate router group names before creating the unique index
	var rgs []models.RouterGroupDB
	err := sqlDB.Client.Find(&rgs)
	if err != nil {
		return err
	}

	nameMap := make(map[string]int)
	for _, rg := range rgs {
		nameMap[rg.Name]++
	}

	for name, count := range nameMap {
		if count > 1 {
			return fmt.Errorf("cannot create unique index: router group name '%s' appears %d times", name, count)
		}
	}

	dropIndex(sqlDB, "idx_rg_name", "router_groups")

	// Create unique index - MySQL requires length prefix for text columns
	var indexSQL string
	if sqlDB.Client.Dialect().Name() == "mysql" {
		indexSQL = "CREATE UNIQUE INDEX idx_rg_name ON router_groups (name(191))"
	} else {
		indexSQL = "CREATE UNIQUE INDEX idx_rg_name ON router_groups (name)"
	}
	return sqlDB.Client.ExecWithError(indexSQL)
}
