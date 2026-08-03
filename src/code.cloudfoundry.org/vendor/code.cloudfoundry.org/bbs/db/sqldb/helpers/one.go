package helpers

import (
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager/v3"
)

// SELECT <columns> FROM <table> WHERE ... LIMIT 1 [FOR UPDATE]
func (h *sqlHelper) One(
	ctx context.Context,
	logger lager.Logger,
	q Queryable,
	table string,
	columns ColumnList,
	lockRow RowLock,
	wheres string,
	whereBindings ...interface{},
) RowScanner {
	query := fmt.Sprintf("SELECT %s FROM %s\n", strings.Join(columns, ", "), table)

	if len(wheres) > 0 {
		query += "WHERE " + wheres
	}

	query += "\nLIMIT 1"

	if lockRow {
		query += "\nFOR UPDATE"
	}

	// meow - I think q here is the tx. Do we need to mock this perhaps?
	// errors are deferred until rows.Scan occurs.
	return q.QueryRowContext(ctx, h.Rebind(query), whereBindings...)
}
