package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
)

const (
	migrationDuration = "MigrationDuration"
)

type Manager struct {
	logger         lager.Logger
	sqlDB          db.DB
	rawSQLDB       *sql.DB
	cryptor        encryption.Cryptor
	migrations     []Migration
	migrationsDone chan<- struct{}
	clock          clock.Clock
	databaseDriver string
	metronClient   loggingclient.IngressClient
}

func NewManager(
	logger lager.Logger,
	sqlDB db.DB,
	rawSQLDB *sql.DB,
	cryptor encryption.Cryptor,
	migrations Migrations,
	migrationsDone chan<- struct{},
	clock clock.Clock,
	databaseDriver string,
	metronClient loggingclient.IngressClient,
) Manager {
	sort.Sort(migrations)

	return Manager{
		logger:         logger,
		sqlDB:          sqlDB,
		rawSQLDB:       rawSQLDB,
		cryptor:        cryptor,
		migrations:     migrations,
		migrationsDone: migrationsDone,
		clock:          clock,
		databaseDriver: databaseDriver,
		metronClient:   metronClient,
	}
}

func (m Manager) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := m.logger.Session("migration-manager")
	logger.Info("starting")

	if m.rawSQLDB == nil {
		err := errors.New("no database configured")
		logger.Error("no-database-configured", err)
		return err
	}

	var maxMigrationVersion int64
	if len(m.migrations) > 0 {
		maxMigrationVersion = m.migrations[len(m.migrations)-1].Version()
	}
	version, err := m.initializeVersion(logger)
	if err != nil {
		return err
	}

	if version > maxMigrationVersion {
		return fmt.Errorf(
			"existing DB version (%d) exceeds bbs version (%d)",
			version,
			maxMigrationVersion,
		)
	}

	errorChan := make(chan error)
	go m.performMigration(logger, version, maxMigrationVersion, errorChan, ready)
	defer logger.Info("exited")

	select {
	case err := <-errorChan:
		logger.Error("migration-failed", err)
		return err
	case <-signals:
		logger.Info("migration-interrupt")
		return nil
	}
}

func (m *Manager) performMigration(
	logger lager.Logger,
	version int64,
	maxMigrationVersion int64,
	errorChan chan error,
	readyChan chan<- struct{},
) {
	migrateStart := m.clock.Now()
	if version != maxMigrationVersion {
		lastVersion := version

		for _, currentMigration := range m.migrations {
			if maxMigrationVersion < currentMigration.Version() {
				break
			}

			if lastVersion < currentMigration.Version() {
				nextVersion := currentMigration.Version()
				logger.Info("running-migration", lager.Data{
					"current_version":   lastVersion,
					"migration_version": nextVersion,
				})

				tx, err := m.rawSQLDB.Begin()
				if err != nil {
					errorChan <- err
					return
				}
				defer tx.Rollback()

				currentMigration.SetCryptor(m.cryptor)
				currentMigration.SetClock(m.clock)
				currentMigration.SetDBFlavor(m.databaseDriver)

				err = currentMigration.Up(tx, m.logger.Session("migration"))
				if err != nil {
					errorChan <- err
					return
				}

				lastVersion = nextVersion

				err = m.writeVersion(tx, lastVersion)
				if err != nil {
					errorChan <- err
					return
				}

				err = tx.Commit()
				if err != nil {
					errorChan <- err
					return
				}

				logger.Info("completed-migration", lager.Data{
					"current_version": lastVersion,
					"target_version":  maxMigrationVersion,
				})
			}
		}
	}

	logger.Debug("migrations-finished")

	err := m.metronClient.SendDuration(migrationDuration, time.Since(migrateStart))
	if err != nil {
		logger.Error("failed-to-send-migration-duration-metric", err)
	}

	m.finish(logger, readyChan)
}

func (m Manager) initializeVersion(logger lager.Logger) (int64, error) {
	tx, err := m.rawSQLDB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	version, err := m.sqlDB.Version(&queryableTx{tx}, context.Background(), logger)
	if err == nil {
		return version.CurrentVersion, nil
	}

	if err != models.ErrResourceNotFound {
		return -1, err
	}

	err = m.writeVersion(tx, 0)
	if err != nil {
		return -1, err
	}

	err = tx.Commit()
	if err != nil {
		return -1, err
	}

	return 0, nil
}

func (m *Manager) finish(logger lager.Logger, ready chan<- struct{}) {
	close(ready)
	close(m.migrationsDone)
	logger.Info("finished-migrations")
}

func (m *Manager) writeVersion(tx *sql.Tx, currentVersion int64) error {
	return m.sqlDB.SetVersion(&queryableTx{tx}, context.Background(), m.logger, &models.Version{
		CurrentVersion: currentVersion,
	})
}

type Migrations []Migration

func (m Migrations) Len() int           { return len(m) }
func (m Migrations) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m Migrations) Less(i, j int) bool { return m[i].Version() < m[j].Version() }

type queryableTx struct {
	tx *sql.Tx
}

func (tx *queryableTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, query, args...)
}

func (tx *queryableTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return tx.tx.PrepareContext(ctx, query)
}

func (tx *queryableTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return tx.tx.QueryContext(ctx, query, args...)
}

func (tx *queryableTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) helpers.RowScanner {
	// meow - perhaps this is where the nil exception is? does tx.tx not exist?
	return tx.tx.QueryRowContext(ctx, query, args...)
}

func (tx *queryableTx) Commit() error {
	return tx.tx.Commit()
}

func (tx *queryableTx) Rollback() error {
	return tx.tx.Rollback()
}
