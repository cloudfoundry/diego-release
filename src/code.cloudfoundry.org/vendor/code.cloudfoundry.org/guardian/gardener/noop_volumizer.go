package gardener

import (
	"errors"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type NoopVolumizer struct{}

var ErrGraphDisabled = errors.New("no image plugin configured")

func (NoopVolumizer) Create(lager.Logger, string, RootfsSpec) (specs.Spec, error) {
	return specs.Spec{}, ErrGraphDisabled
}

func (NoopVolumizer) Destroy(lager.Logger, string) error {
	return nil
}

func (NoopVolumizer) Metrics(lager.Logger, string, bool) (garden.ContainerDiskStat, error) {
	return garden.ContainerDiskStat{}, nil
}

func (NoopVolumizer) GC(lager.Logger) error {
	return nil
}

func (NoopVolumizer) Capacity(lager.Logger) (uint64, error) {
	return 0, nil
}
