package migration

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
	"gorm.io/gorm"
)

// dropIndex drops an index by name. MySQL requires "DROP INDEX name ON table",
// PostgreSQL uses "DROP INDEX IF EXISTS name".
func dropIndex(sqlDB *db.SqlDB, indexName, tableName string) {
	if sqlDB.Client.Dialect().Name() == "mysql" {
		_ = sqlDB.Client.ExecWithError(fmt.Sprintf("DROP INDEX %s ON %s", indexName, tableName))
	} else {
		_ = sqlDB.Client.ExecWithError(fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName))
	}
}

const MigrationKey = "routing-api-migration"

type MigrationData struct {
	MigrationKey   string `gorm:"primary_key"`
	CurrentVersion int
	TargetVersion  int
}

type Runner struct {
	sqlDB  *db.SqlDB
	logger lager.Logger
}

func NewRunner(
	sqlDB *db.SqlDB,
	logger lager.Logger,
) *Runner {
	return &Runner{
		sqlDB:  sqlDB,
		logger: logger,
	}
}
func (r *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err2 := RunAllMigration(r.sqlDB, r.logger)
	if err2 != nil {
		return err2
	}

	close(ready)

	sig := <-signals
	r.logger.Info("received signal", lager.Data{"signal": sig})

	return nil
}

func RunAllMigration(sqlDB *db.SqlDB, logger lager.Logger) error {
	migrations := InitializeMigrations()

	logger.Info("starting-migration")
	err := RunMigrations(sqlDB, migrations, logger)
	if err != nil {
		logger.Error("migrations-failed", err)
		return err
	}
	logger.Info("finished-migration")
	return nil
}

//go:generate counterfeiter -o fakes/fake_migration.go . Migration
type Migration interface {
	Run(*db.SqlDB) error
	Version() int
}

func InitializeMigrations() []Migration {
	migrations := make([]Migration, 0)
	var migration Migration

	migration = NewV0InitMigration()
	migrations = append(migrations, migration)

	migration = NewV2UpdateRgMigration()
	migrations = append(migrations, migration)

	migration = NewV3UpdateTcpRouteMigration()
	migrations = append(migrations, migration)

	migration = NewV4AddRgUniqIdxTCPRouteMigration()
	migrations = append(migrations, migration)

	migration = NewV5SniHostnameMigration()
	migrations = append(migrations, migration)

	migration = NewV6TCPTLSRoutes()
	migrations = append(migrations, migration)

	migration = NewV7TCPTLSRoutes()
	migrations = append(migrations, migration)

	migration = NewV8HostTLSPortTCPDefaultZero()
	migrations = append(migrations, migration)

	migration = NewV9TerminateFrontendTLS()
	migrations = append(migrations, migration)

	migration = NewV10SniRewriteHostname()
	migrations = append(migrations, migration)

	return migrations
}

func RunMigrations(sqlDB *db.SqlDB, migrations []Migration, logger lager.Logger) error {
	if len(migrations) == 0 {
		return nil
	}

	if sqlDB == nil {
		return nil
	}

	lastMigrationVersion := migrations[len(migrations)-1].Version()
	err := sqlDB.Client.AutoMigrate(&MigrationData{})
	if err != nil {
		return err
	}

	tx := sqlDB.Client.Begin()
	existingVersion := &MigrationData{}

	err = tx.Where("migration_key = ?", MigrationKey).First(existingVersion)
	if err != nil && err != gorm.ErrRecordNotFound {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			logger.Error("rollback-error", rollbackErr)
		}
		return err
	}

	// if no migration data found, seed it
	if err == gorm.ErrRecordNotFound {
		existingVersion = &MigrationData{
			MigrationKey:   MigrationKey,
			CurrentVersion: -1,
			TargetVersion:  lastMigrationVersion,
		}

		logger.Info("creating-migration-version", lager.Data{"version": existingVersion})
		_, err = tx.Create(existingVersion)
	} else {
		// if we're at the latest version, exit all migrations
		if existingVersion.CurrentVersion >= lastMigrationVersion {
			logger.Debug("already-at-latest-version", lager.Data{"version": existingVersion})
			return tx.Commit()
		}

		// we weren't at the latest version, so lets set the target version.
		existingVersion.TargetVersion = lastMigrationVersion
		logger.Info("updating-migration-version", lager.Data{"version": existingVersion})
		_, err = tx.Save(existingVersion)
	}

	// if saving the migration info failed, roll it back.
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			logger.Error("rollback-error", rollbackErr)
		}
		return err
	}

	// otherwise commit it
	err = tx.Commit()
	if err != nil {
		logger.Error("commit-error", err)
		return err
	}

	// now try to iterate over migrations until we're current.
	currentVersion := existingVersion.CurrentVersion
	for _, m := range migrations {
		if m != nil && m.Version() > currentVersion {
			err = m.Run(sqlDB)
			if err != nil {
				return err
			}
			currentVersion = m.Version()
			existingVersion.CurrentVersion = currentVersion
			_, err = sqlDB.Client.Save(existingVersion)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
