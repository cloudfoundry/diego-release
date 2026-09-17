package k8sgarden

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/processes"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager/v3"
	ctrdclient "github.com/containerd/containerd/v2/client"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	corev1 "k8s.io/api/core/v1"
)

var (
	Caps = []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_FSETID", "CAP_KILL", "CAP_SETGID", "CAP_SETUID", "CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_NET_RAW", "CAP_SYS_CHROOT", "CAP_MKNOD", "CAP_AUDIT_WRITE", "CAP_SETFCAP"}
)

type container struct {
	log             lager.Logger
	pod             *corev1.Pod
	env             []string
	cpuAssignment   float64
	rootfsSize      uint64
	userLookupper   users.UserLookupper
	taskMap         map[string]ctrdclient.Task
	containerIDMap  map[string]string
	propertyManager gardener.PropertyManager
	sandboxPath     string
	mu              sync.RWMutex
}

func NewContainer(
	log lager.Logger,
	pod *corev1.Pod,
	env []string,
	cpuAssignment float64,
	userLookupper users.UserLookupper,
	propertyManager gardener.PropertyManager,
	rootfsSize uint64,
	taskMap map[string]ctrdclient.Task,
	sandboxPath string,
) *container {
	return &container{
		log:             log,
		pod:             pod,
		env:             env,
		cpuAssignment:   cpuAssignment,
		rootfsSize:      rootfsSize,
		userLookupper:   userLookupper,
		taskMap:         taskMap,
		propertyManager: propertyManager,
		sandboxPath:     sandboxPath,
		mu:              sync.RWMutex{},
	}
}

// Handle implements [garden.Container].
func (c *container) Handle() string {
	return c.pod.GetName()
}

// Info implements [garden.Container].
func (c *container) Info() (garden.ContainerInfo, error) {
	portMapping := []garden.PortMapping{}
	for _, container := range c.pod.Spec.Containers {
		if len(container.Ports) == 0 {
			continue
		}

		for _, containerPort := range container.Ports {
			portMapping = append(portMapping, garden.PortMapping{
				HostPort:      uint32(containerPort.HostPort),
				ContainerPort: uint32(containerPort.ContainerPort),
			})
		}
	}

	return garden.ContainerInfo{
		State:         "active",
		Events:        []string{},
		HostIP:        c.pod.Status.HostIP,
		ContainerIP:   c.pod.Status.PodIP,
		ContainerIPv6: "",
		ExternalIP:    c.pod.Status.HostIP,
		MappedPorts:   portMapping,
	}, nil
}

// Run implements [garden.Container].
func (c *container) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return c.run(spec, io, false)
}

func (c *container) run(spec garden.ProcessSpec, io garden.ProcessIO, cleanEnv bool) (garden.Process, error) {
	c.log.Info("running-process", lager.Data{"processSpec": spec})
	targetContainer := appContainerName
	if spec.Image.URI != "" {
		targetContainer = sidecarContainerName
	}

	execUser, err := c.userLookupper.Lookup(filepath.Join(c.sandboxPath, c.containerIDMap[targetContainer], "rootfs"), spec.User)
	if err != nil {
		return nil, fmt.Errorf("get user %q: %w", spec.User, err)
	}

	baseEnv := c.env
	if cleanEnv {
		baseEnv = nil
	}

	processSpec := &specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  baseEnv,
		Cwd:  spec.Dir,
		User: specs.User{
			UID:      uint32(execUser.Uid),
			GID:      uint32(execUser.Gid),
			Username: spec.User,
		},
		Capabilities: &specs.LinuxCapabilities{
			Bounding:    Caps,
			Inheritable: Caps,
			Permitted:   Caps,
		},
		NoNewPrivileges: false,
	}
	processSpec.Env = processes.UnixEnvFor(goci.Bndl{Spec: specs.Spec{Process: processSpec}}, spec, execUser.Uid)
	processSpec.Env = append(processSpec.Env, spec.Env...)

	if spec.Dir == "" {
		processSpec.Cwd = execUser.Home
	}

	id := spec.ID
	if id == "" {
		id = uuid.NewString()
	}

	return NewProcess(
		c.log.Session("process", lager.Data{"processID": id}),
		id,
		processSpec,
		io,
		c.taskMap[targetContainer],
	), nil
}

// StreamIn implements [garden.Container].
func (c *container) StreamIn(spec garden.StreamInSpec) error {
	c.log.Info("stream-in-starting", lager.Data{"path": spec.Path, "user": spec.User})
	defer c.log.Info("stream-in-completed", lager.Data{"path": spec.Path, "user": spec.User})

	c.log.Info("stream-in-running-tar", lager.Data{"path": spec.Path, "user": spec.User})
	output := new(bytes.Buffer)
	streamProcess, err := c.run(garden.ProcessSpec{
		ID:   "stream-in-" + uuid.NewString(),
		Path: "/tmp/bin/untar",
		Args: []string{"/tmp/bin/tar", streamUser(spec.User), spec.Path},
		User: "root",
	}, garden.ProcessIO{
		Stdin:  spec.TarStream,
		Stdout: io.Discard,
		Stderr: output,
	}, true)

	if err != nil {
		c.log.Error("failed-to-run-stream-in-tar", err)
		return fmt.Errorf("stream-in: failed to run tar: %w", err)
	}

	exitCode, err := streamProcess.Wait()
	if err != nil {
		c.log.Error("failed-to-wait-for-stream-in-tar", err)
		return fmt.Errorf("stream-in: failed to wait for tar: %w", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("stream-in: tar exited %d: %s", exitCode, output.String())
	}

	return nil
}

// StreamOut implements [garden.Container].
func (c *container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	c.log.Info("stream-out-starting", lager.Data{"path": spec.Path, "user": spec.User})
	defer c.log.Info("stream-out-completed", lager.Data{"path": spec.Path, "user": spec.User})

	sourcePath := filepath.Dir(spec.Path)
	compressPath := filepath.Base(spec.Path)
	if strings.HasSuffix(spec.Path, "/") {
		sourcePath = spec.Path
		compressPath = "."
	}

	errOut := new(bytes.Buffer)
	reader, writer := io.Pipe()

	c.log.Info("stream-out-running-tar", lager.Data{"sourcePath": sourcePath, "compressPath": compressPath, "user": spec.User})
	streamProcess, err := c.run(garden.ProcessSpec{
		ID:   "stream-out-" + uuid.NewString(),
		Path: "/tmp/bin/tar",
		Args: []string{"--no-same-permissions", "--no-same-owner", "--xattrs", "--xattrs-include=*", "-C", sourcePath, "-cf", "-", compressPath},
		User: streamUser(spec.User),
	}, garden.ProcessIO{
		Stdout: writer,
		Stderr: errOut,
	}, true)
	if err != nil {
		c.log.Error("failed-to-run-stream-out-tar", err)
		return nil, fmt.Errorf("stream-out: failed to run tar: %w", err)
	}

	go func() {
		exitCode, waitErr := streamProcess.Wait()
		switch {
		case waitErr != nil:
			waitErr = fmt.Errorf("stream-out: failed to wait for tar: %w (output: %s)", waitErr, errOut.String())
			c.log.Error("failed-to-wait-for-stream-out-tar", waitErr)
		case exitCode != 0:
			waitErr = fmt.Errorf("stream-out: tar exited %d: %s", exitCode, errOut.String())
			c.log.Error("stream-out-tar-non-zero-exit", waitErr)
		}

		writer.CloseWithError(waitErr)
	}()

	return reader, nil
}

func streamUser(user string) string {
	if user == "" {
		return "root"
	}
	return user
}

func (c *container) Properties() (garden.Properties, error) {
	return c.propertyManager.All(c.Handle())
}

func (c *container) Property(name string) (string, error) {
	if prop, ok := c.propertyManager.Get(c.Handle(), name); ok {
		return prop, nil
	}

	return "", fmt.Errorf("property does not exist: %s", name)
}

func (c *container) SetProperty(name string, value string) error {
	c.propertyManager.Set(c.Handle(), name, value)
	return nil
}

func (c *container) RemoveProperty(name string) error {
	// we explicitly stopped handling this in 2016, see git blame + commit log -- COPIED FROM GARDEN
	_ = c.propertyManager.Remove(c.Handle(), name)
	return nil
}

func (c *container) SetGraceTime(t time.Duration) error {
	c.propertyManager.Set(c.Handle(), gardener.GraceTimeKey, fmt.Sprintf("%d", t))
	return nil
}

// Attach implements [garden.Container].
func (c *container) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	return nil, ErrNotSupported
}

// BulkNetOut implements [garden.Container].
func (c *container) BulkNetOut(netOutRules []garden.NetOutRule) error {
	return ErrNotSupported
}

// CurrentBandwidthLimits implements [garden.Container].
func (c *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, ErrNotSupported
}

// CurrentCPULimits implements [garden.Container].
func (c *container) CurrentCPULimits() (garden.CPULimits, error) {
	return garden.CPULimits{}, ErrNotSupported
}

// CurrentDiskLimits implements [garden.Container].
func (c *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, ErrNotSupported
}

// CurrentMemoryLimits implements [garden.Container].
func (c *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return garden.MemoryLimits{}, ErrNotSupported
}

// Metrics implements [garden.Container].
func (c *container) Metrics() (garden.Metrics, error) {
	return garden.Metrics{}, ErrNotSupported
}

// NetIn implements [garden.Container].
func (c *container) NetIn(hostPort uint32, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, ErrNotSupported
}

// NetOut implements [garden.Container].
func (c *container) NetOut(netOutRule garden.NetOutRule) error {
	return ErrNotSupported
}

// Stop implements [garden.Container].
func (c *container) Stop(kill bool) error {
	return ErrNotSupported
}
