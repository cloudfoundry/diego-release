package models

func (s Sidecar) Validate() error {
	var validationError ValidationError

	if s.Action == nil {
		validationError = validationError.Append(ErrInvalidActionType)
	} else if err := s.Action.Validate(); err != nil {
		validationError = validationError.Append(ErrInvalidField{"action"})
		validationError = validationError.Append(err)
	}

	if s.GetMemoryMb() < 0 {
		validationError = validationError.Append(ErrInvalidField{"memory_mb"})
	}

	if s.GetDiskMb() < 0 {
		validationError = validationError.Append(ErrInvalidField{"disk_mb"})
	}

	return validationError
}

func validateSidecars(sidecars []*Sidecar) ValidationError {
	var validationError ValidationError

	for _, s := range sidecars {
		validationError = validationError.Check(s)
	}

	return validationError
}
