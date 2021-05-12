package errors

import (
	"errors"
)

func NewUnauthorizedError() error {
	return errors.New("Unauthorized")
}
