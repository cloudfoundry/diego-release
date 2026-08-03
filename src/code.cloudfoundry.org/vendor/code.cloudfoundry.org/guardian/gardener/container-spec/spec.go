package spec

import (
	"code.cloudfoundry.org/garden"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type ActualContainerSpec struct {
	// The PID of the container's init process
	Pid int

	// The path to the container's bundle directory
	BundlePath string

	// The path to the container's rootfs
	RootFSPath string

	// Whether the container is stopped
	Stopped bool

	// Process IDs (not PIDs) of processes in the container
	ProcessIDs []string

	// Events (e.g. OOM) which have occured in the container
	Events []string

	// Applied limits
	Limits garden.Limits

	// Whether the container is privileged
	Privileged bool
}

type DesiredContainerSpec struct {
	Handle string

	CgroupPath string

	Namespaces map[string]string

	// Container hostname
	Hostname string

	// Bind mounts
	BindMounts []garden.BindMount

	Env []string

	// Container is privileged
	Privileged bool

	Limits garden.Limits

	BaseConfig specs.Spec
}
