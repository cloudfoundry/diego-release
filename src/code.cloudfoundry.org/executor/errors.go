package executor

type Error interface {
	error

	Name() string
}

type execError struct {
	name    string
	message string
}

func (err execError) Name() string {
	return err.name
}

func (err execError) Error() string {
	return err.message
}

var Errors = map[string]Error{}

func registerError(name, message string) Error {
	err := execError{name, message}
	Errors[name] = err
	return err
}

var (
	ErrContainerGuidNotAvailable      = registerError("ContainerGuidNotAvailable", "container guid not available")
	ErrContainerNotCompleted          = registerError("ContainerNotCompleted", "container must be stopped before it can be deleted")
	ErrInsufficientResourcesAvailable = registerError("InsufficientResourcesAvailable", "insufficient resources available")
	ErrContainerNotFound              = registerError("ContainerNotFound", "container not found")
	ErrStepsInvalid                   = registerError("StepsInvalid", "steps invalid")
	ErrLimitsInvalid                  = registerError("LimitsInvalid", "container limits invalid")
	ErrGuidNotSpecified               = registerError("GuidNotSpecified", "container guid not specified")
	ErrInvalidTransition              = registerError("InvalidStateTransition", "container cannot transition to given state")
	ErrFailureToCheckSpace            = registerError("ErrFailureToCheckSpace", "failed to check available space")
	ErrInvalidSecurityGroup           = registerError("ErrInvalidSecurityGroup", "security group has invalid values")
	ErrNoProcessToStop                = registerError("ErrNoProcessToStop", "failed to find a process to stop")
)
