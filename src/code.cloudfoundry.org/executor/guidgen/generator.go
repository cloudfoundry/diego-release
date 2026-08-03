package guidgen

import (
	"code.cloudfoundry.org/lager/v3"
	uuid "github.com/nu7hatch/gouuid"
)

var DefaultGenerator Generator = &generator{}

//go:generate counterfeiter -o fakeguidgen/fake_generator.go . Generator

type Generator interface {
	Guid(lager.Logger) string
}

type generator struct{}

func (*generator) Guid(logger lager.Logger) string {
	guid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("failed-to-generate-guid", err)
	}
	return guid.String()
}
