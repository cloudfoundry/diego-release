package credhub

import (
	"errors"
	"fmt"
)

// Error provides errors for the CredHub client
type Error struct {
	Name        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *Error) Error() string {
	if e.Description == "" {
		return e.Name
	}
	return fmt.Sprintf("%s: %s", e.Name, e.Description)
}

func newCredhubError(name, description string) error {
	return &Error{
		Name:        name,
		Description: description,
	}
}

type NotFoundError struct {
	Description string `json:"error"`
}

func (e *NotFoundError) Error() string {
	return e.Description
}

var ServerDoesNotSupportMetadataError = errors.New("the server does not support credential metadata, requires >= 2.6.x")
