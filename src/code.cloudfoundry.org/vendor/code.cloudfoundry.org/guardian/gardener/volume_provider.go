package gardener

import (
	"errors"
	"net/url"
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const RawRootFSScheme = "raw"

type CommandFactory func(ctrPID, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd

type VolumeProvider struct {
	VolumeCreator VolumeCreator
	VolumeDestroyMetricsGC
	commandRunner    commandrunner.CommandRunner
	ContainerRootUID int
	ContainerRootGID int
}

func NewVolumeProvider(creator VolumeCreator, manager VolumeDestroyMetricsGC, commandrunner commandrunner.CommandRunner, rootUID, rootGID int) *VolumeProvider {
	return &VolumeProvider{
		VolumeCreator:          creator,
		VolumeDestroyMetricsGC: manager,
		commandRunner:          commandrunner,
		ContainerRootUID:       rootUID,
		ContainerRootGID:       rootGID,
	}
}

type VolumeCreator interface {
	Create(log lager.Logger, handle string, spec RootfsSpec) (specs.Spec, error)
}

// TODO GoRename RootfsSpec
type RootfsSpec struct {
	RootFS     *url.URL
	Username   string `json:"-"`
	Password   string `json:"-"`
	Namespaced bool
	QuotaSize  int64
	QuotaScope garden.DiskLimitScope
}

func (v *VolumeProvider) Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error) {
	path := spec.Image.URI
	if path == "" {
		path = spec.RootFSPath
	} else if spec.RootFSPath != "" {
		return specs.Spec{}, errors.New("Cannot provide both Image.URI and RootFSPath")
	}

	rootFSURL, err := url.Parse(path)
	if err != nil {
		return specs.Spec{}, err
	}

	var baseConfig specs.Spec
	if rootFSURL.Scheme == RawRootFSScheme {
		baseConfig.Root = &specs.Root{Path: rootFSURL.Path}
		baseConfig.Process = &specs.Process{}
	} else {
		var err error
		baseConfig, err = v.VolumeCreator.Create(log.Session("volume-creator"), spec.Handle, RootfsSpec{
			RootFS:     rootFSURL,
			Username:   spec.Image.Username,
			Password:   spec.Image.Password,
			QuotaSize:  int64(spec.Limits.Disk.ByteHard),
			QuotaScope: spec.Limits.Disk.Scope,
			Namespaced: !spec.Privileged,
		})
		if err != nil {
			return specs.Spec{}, err
		}
	}

	return baseConfig, nil
}
