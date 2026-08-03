package helpers

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager/v3"
)

// SELECT COUNT(*) FROM <table> WHERE ...
func (h *sqlHelper) Count(
	ctx context.Context,
	logger lager.Logger,
	q Queryable,
	table string,
	wheres string,
	whereBindings ...interface{},
) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s\n", table)

	if len(wheres) > 0 {
		query += "WHERE " + wheres
	}

	var count int
	err := q.QueryRowContext(ctx, h.Rebind(query), whereBindings...).Scan(&count)
	return count, err
}
