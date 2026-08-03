package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager/v3"
)

// UPDATE <table> SET ... WHERE ...
func (h *sqlHelper) Update(
	ctx context.Context,
	logger lager.Logger,
	q Queryable,
	table string,
	updates SQLAttributes,
	wheres string,
	whereBindings ...interface{},
) (sql.Result, error) {
	updateCount := len(updates)
	if updateCount == 0 {
		return nil, nil
	}

	query := fmt.Sprintf("UPDATE %s SET\n", table)
	updateQueries := make([]string, 0, updateCount)
	bindings := make([]interface{}, 0, updateCount+len(whereBindings))

	for column, value := range updates {
		updateQueries = append(updateQueries, fmt.Sprintf("%s = ?", column))
		bindings = append(bindings, value)
	}
	query += strings.Join(updateQueries, ", ") + "\n"
	if len(wheres) > 0 {
		query += "WHERE " + wheres
		bindings = append(bindings, whereBindings...)
	}

	return q.ExecContext(ctx, h.Rebind(query), bindings...)
}
