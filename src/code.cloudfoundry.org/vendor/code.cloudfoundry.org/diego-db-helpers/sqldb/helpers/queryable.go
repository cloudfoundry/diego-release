package helpers

import (
	"context"
	"database/sql"
	"time"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers/monitor"
)

//counterfeiter:generate . RowScanner
type RowScanner interface {
	Scan(dest ...interface{}) error
}

type Queryable interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner
}

//go:generate counterfeiter -generate

//counterfeiter:generate . QueryableDB
type QueryableDB interface {
	Queryable
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	OpenConnections() int
	WaitDuration() time.Duration
	WaitCount() int64
}

//counterfeiter:generate . Tx
type Tx interface {
	Queryable
	Commit() error
	Rollback() error
}

type monitoredTx struct {
	tx      *sql.Tx
	monitor monitor.Monitor
}

type monitoredDB struct {
	db      *sql.DB
	monitor monitor.Monitor
}

func NewMonitoredDB(db *sql.DB, monitor monitor.Monitor) QueryableDB {
	return &monitoredDB{
		db:      db,
		monitor: monitor,
	}
}

func (db *monitoredDB) OpenConnections() int {
	return db.db.Stats().OpenConnections
}

func (db *monitoredDB) WaitDuration() time.Duration {
	return db.db.Stats().WaitDuration
}

func (db *monitoredDB) WaitCount() int64 {
	return db.db.Stats().WaitCount
}

func (q *monitoredDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	var innerTx *sql.Tx
	err := q.monitor.Monitor(func() error {
		var err error
		innerTx, err = q.db.BeginTx(ctx, opts)
		return err
	})

	tx := &monitoredTx{
		tx:      innerTx,
		monitor: q.monitor,
	}

	return tx, err
}

func (q *monitoredDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := q.monitor.Monitor(func() error {
		var err error
		result, err = q.db.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

func (q *monitoredDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return q.db.PrepareContext(ctx, query)
}

func (q *monitoredDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var result *sql.Rows
	err := q.monitor.Monitor(func() error {
		var err error
		result, err = q.db.QueryContext(ctx, query, args...)
		return err
	})
	return result, err
}

func (q *monitoredDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return NewRowScanner(q.monitor, q.db.QueryRowContext(ctx, query, args...))
}

func (tx *monitoredTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := tx.monitor.Monitor(func() error {
		var err error
		result, err = tx.tx.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

func (tx *monitoredTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return tx.tx.PrepareContext(ctx, query)
}

func (tx *monitoredTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var result *sql.Rows
	err := tx.monitor.Monitor(func() error {
		var err error
		result, err = tx.tx.QueryContext(ctx, query, args...)
		return err
	})
	return result, err
}

func (tx *monitoredTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return NewRowScanner(tx.monitor, tx.tx.QueryRowContext(ctx, query, args...))
}

func (tx *monitoredTx) Commit() error {
	return tx.monitor.Monitor(tx.tx.Commit)
}

func (tx *monitoredTx) Rollback() error {
	return tx.monitor.Monitor(tx.tx.Rollback)
}

type scannableRow struct {
	monitor monitor.Monitor
	scanner RowScanner
}

func NewRowScanner(monitor monitor.Monitor, scanner RowScanner) RowScanner {
	return &scannableRow{monitor: monitor, scanner: scanner}
}

func (r *scannableRow) Scan(dest ...interface{}) error {
	return r.monitor.Monitor(func() error {
		return r.scanner.Scan(dest...)
	})
}
