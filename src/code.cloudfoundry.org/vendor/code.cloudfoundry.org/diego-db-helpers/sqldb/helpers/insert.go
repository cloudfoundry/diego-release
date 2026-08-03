package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager/v3"
)

// INSERT INTO <table> (...) VALUES ...
func (h *sqlHelper) Insert(
	ctx context.Context,
	logger lager.Logger,
	q Queryable,
	table string,
	attributes SQLAttributes,
) (sql.Result, error) {
	attributeCount := len(attributes)
	if attributeCount == 0 {
		return nil, nil
	}

	query := fmt.Sprintf("INSERT INTO %s\n", table)
	attributeNames := make([]string, 0, attributeCount)
	attributeBindings := make([]string, 0, attributeCount)
	bindings := make([]interface{}, 0, attributeCount)

	for column, value := range attributes {
		attributeNames = append(attributeNames, column)
		attributeBindings = append(attributeBindings, "?")
		bindings = append(bindings, value)
	}
	query += fmt.Sprintf("(%s)", strings.Join(attributeNames, ", "))
	query += fmt.Sprintf("VALUES (%s)", strings.Join(attributeBindings, ", "))

	return q.ExecContext(ctx, h.Rebind(query), bindings...)
}
