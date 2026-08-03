package helpers

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrResourceExists     = errors.New("sql-resource-exists")
	ErrDeadlock           = errors.New("sql-deadlock")
	ErrBadRequest         = errors.New("sql-bad-request")
	ErrUnrecoverableError = errors.New("sql-unrecoverable")
	ErrResourceNotFound   = errors.New("sql-resource-not-found")
)

type ErrUnknownError struct {
	errorCode string
	flavor    string
}

func (e *ErrUnknownError) Error() string {
	return fmt.Sprintf("sql-unknown, error code: %s, flavor: %s", e.errorCode, e.flavor)
}

func (h *sqlHelper) ConvertSQLError(err error) error {
	if err != nil {
		switch e := err.(type) {
		case *mysql.MySQLError:
			return h.convertMySQLError(e)
		case *pgconn.PgError:
			return h.convertPostgresError(e)
		case *pgconn.PrepareError:
			subError := e.Unwrap()
			if errors.Is(subError, &pgconn.PrepareError{}) {
				// don't recurse if someone has mistakenly wrapped a PrepareError in a PrepareError
				// to ensure we avoid invinite looping
				return err
			}
			return h.ConvertSQLError(e.Unwrap())
		}

		if err == sql.ErrNoRows {
			return ErrResourceNotFound
		}
	}

	return err
}

func (h *sqlHelper) convertMySQLError(err *mysql.MySQLError) error {
	switch err.Number {
	case 1062:
		return ErrResourceExists
	case 1213:
		return ErrDeadlock
	case 1406:
		return ErrBadRequest
	case 1146:
		return ErrUnrecoverableError
	default:
		return &ErrUnknownError{errorCode: strconv.Itoa(int(err.Number)), flavor: MySQL}
	}
}

func (h *sqlHelper) convertPostgresError(err *pgconn.PgError) error {
	switch err.Code {
	case "22001":
		return ErrBadRequest
	case "23505":
		return ErrResourceExists
	case "42P01":
		return ErrUnrecoverableError
	default:
		return &ErrUnknownError{errorCode: string(err.Code), flavor: Postgres}
	}
}
