package helpers

import (
	"context"
	"database/sql/driver"
	"time"

	"github.com/go-sql-driver/mysql"

	"code.cloudfoundry.org/lager/v3"
)

// BEGIN TRANSACTION; f ... ; COMMIT; or
// BEGIN TRANSACTION; f ... ; ROLLBACK; if f returns an error.
func (h *sqlHelper) Transact(ctx context.Context, logger lager.Logger, db QueryableDB, f func(logger lager.Logger, tx Tx) error) error {
	return h.RetryOnDeadlock(logger, func() error {
		// meow - the transact wrapper called Begin.
		// The test is making sure Begin is called 3 times.
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			logger.Error("failed-starting-transaction", err)
			return err
		}
		defer tx.Rollback()

		err = f(logger, tx)
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			logger.Error("failed-committing-transaction", err)

		}
		return err
	})
}

func (h *sqlHelper) RetryOnDeadlock(logger lager.Logger, f func() error) error {
	var err error

	for attempts := 0; attempts < 3; attempts++ {
		err = f()
		convertedErr := h.ConvertSQLError(err)
		// golang sql package does not always retry query on ErrBadConn, e.g. if it
		// is in the middle of a transaction. This make sense since the package
		// cannot retry the entire transaction and has to return control to the
		// caller to initiate a retry
		if attempts >= 2 || (convertedErr != ErrDeadlock && convertedErr != driver.ErrBadConn && convertedErr != mysql.ErrInvalidConn) {
			break
		} else {
			logger.Error("deadlock-transaction", err, lager.Data{"attempts": attempts})
			time.Sleep(500 * time.Millisecond)
		}
	}
	return err
}
