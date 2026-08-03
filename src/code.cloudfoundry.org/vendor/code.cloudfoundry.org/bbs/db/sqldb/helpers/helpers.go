package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager/v3"
)

const (
	MySQL    = "mysql"
	Postgres = "postgres"

	LockRow   RowLock = true
	NoLockRow RowLock = false
)

type SQLHelper interface {
	Transact(ctx context.Context, logger lager.Logger, db QueryableDB, f func(logger lager.Logger, tx Tx) error) error
	RetryOnDeadlock(logger lager.Logger, f func() error) error
	One(ctx context.Context, logger lager.Logger, q Queryable, table string, columns ColumnList, lockRow RowLock, wheres string, whereBindings ...interface{}) RowScanner
	All(ctx context.Context, logger lager.Logger, q Queryable, table string, columns ColumnList, lockRow RowLock, wheres string, whereBindings ...interface{}) (*sql.Rows, error)
	Upsert(ctx context.Context, logger lager.Logger, q Queryable, table string, attributes SQLAttributes, wheres string, whereBindings ...interface{}) (bool, error)
	Insert(ctx context.Context, logger lager.Logger, q Queryable, table string, attributes SQLAttributes) (sql.Result, error)
	Update(ctx context.Context, logger lager.Logger, q Queryable, table string, updates SQLAttributes, wheres string, whereBindings ...interface{}) (sql.Result, error)
	Delete(ctx context.Context, logger lager.Logger, q Queryable, table string, wheres string, whereBindings ...interface{}) (sql.Result, error)
	Count(ctx context.Context, logger lager.Logger, q Queryable, table string, wheres string, whereBindings ...interface{}) (int, error)

	ConvertSQLError(err error) error
	Rebind(query string) string
}

type sqlHelper struct {
	flavor string
}

func NewSQLHelper(flavor string) *sqlHelper {
	return &sqlHelper{flavor: flavor}
}

type RowLock bool
type SQLAttributes map[string]interface{}
type ColumnList []string

func (h *sqlHelper) Rebind(query string) string {
	return RebindForFlavor(query, h.flavor)
}

func RebindForFlavor(query, flavor string) string {
	if flavor == MySQL {
		return query
	}
	if flavor != Postgres {
		panic(fmt.Sprintf("Unrecognized DB flavor '%s'", flavor))
	}

	strParts := strings.Split(query, "?")
	for i := 1; i < len(strParts); i++ {
		strParts[i-1] = fmt.Sprintf("%s$%d", strParts[i-1], i)
	}
	return strings.Replace(strings.Join(strParts, ""), "MEDIUMTEXT", "TEXT", -1)
}

func QuestionMarks(count int) string {
	if count == 0 {
		return ""
	}
	return strings.Repeat("?, ", count-1) + "?"
}
