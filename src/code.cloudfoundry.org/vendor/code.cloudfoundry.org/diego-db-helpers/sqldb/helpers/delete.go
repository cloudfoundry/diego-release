package helpers

import (
	"context"
	"database/sql"
	"fmt"

	"code.cloudfoundry.org/lager/v3"
)

// DELETE FROM <table> WHERE ...
func (h *sqlHelper) Delete(
	ctx context.Context,
	logger lager.Logger,
	q Queryable,
	table string,
	wheres string,
	whereBindings ...interface{},
) (sql.Result, error) {
	query := fmt.Sprintf("DELETE FROM %s\n", table)

	if len(wheres) > 0 {
		query += "WHERE " + wheres
	}

	return q.ExecContext(ctx, h.Rebind(query), whereBindings...)
}
