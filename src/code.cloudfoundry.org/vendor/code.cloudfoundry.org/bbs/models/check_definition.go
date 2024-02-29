package models

type PortChecker interface {
	GetPort() uint32
}

func (check CheckDefinition) Validate() error {
	var validationError ValidationError

	checks := check.GetChecks()

	for _, check := range checks {
		checkError := check.Validate()
		if checkError != nil {
			validationError = validationError.Append(checkError)
		}
	}

	readiness_checks := check.GetReadinessChecks()

	for _, check := range readiness_checks {
		checkError := check.Validate()
		if checkError != nil {
			validationError = validationError.Append(checkError)
		}
	}

	return validationError.ToError()

}

func (check Check) GetPortChecker() PortChecker {
	httpCheck := check.GetHttpCheck()
	tcpCheck := check.GetTcpCheck()
	if httpCheck != nil && tcpCheck != nil {
		return nil
	}
	if httpCheck != nil {
		return httpCheck
	} else {
		return tcpCheck
	}
}

func (check Check) Validate() error {
	var validationError ValidationError
	c := check.GetPortChecker()

	if c == nil {
		validationError = validationError.Append(ErrInvalidField{"check"})
	} else if !(c.GetPort() > 0 && c.GetPort() <= 65535) {
		validationError = validationError.Append(ErrInvalidField{"port"})
	}
	return validationError.ToError()
}
