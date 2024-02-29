package models

import "errors"

func (envVar EnvironmentVariable) Validate() error {
	if envVar.Name == "" {
		return errors.New("invalid field: name cannot be blank")
	}
	return nil
}
