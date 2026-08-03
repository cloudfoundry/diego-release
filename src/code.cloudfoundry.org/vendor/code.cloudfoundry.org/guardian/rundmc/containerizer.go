package rundmc

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/event"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/v3"
	"github.com/cloudfoundry/dropsonde/metrics"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Depot
//counterfeiter:generate . OCIRuntime
//counterfeiter:generate . NstarRunner
//counterfeiter:generate . EventStore
//counterfeiter:generate . ProcessesStopper
//counterfeiter:generate . StateStore
//counterfeiter:generate . PeaCreator
//counterfeiter:generate . PeaUsernameResolver
//counterfeiter:generate . RuntimeStopper
//counterfeiter:generate . CPUCgrouper

type Depot interface {
	Destroy(log lager.Logger, handle string) error
}

//counterfeiter:generate . BundleGenerator
type BundleGenerator interface {
	Generate(desiredContainerSpec spec.DesiredContainerSpec) (goci.Bndl, error)
}

type Status string

const RunningStatus Status = "running"
const CreatedStatus Status = "created"
const StoppedStatus Status = "stopped"

type State struct {
	Pid    int
	Status Status
}

type OCIRuntime interface {
	Create(log lager.Logger, id string, bundle goci.Bndl, io garden.ProcessIO) error
	Exec(log lager.Logger, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(log lager.Logger, id, processId string, io garden.ProcessIO) (garden.Process, error)
	Delete(log lager.Logger, id string) error
	State(log lager.Logger, id string) (State, error)
	Stats(log lager.Logger, id string) (gardener.StatsContainerMetrics, error)
	Events(log lager.Logger) (<-chan event.Event, error)
	ContainerHandles() ([]string, error)
	ContainerPeaHandles(log lager.Logger, id string) ([]string, error)
	BundleInfo(log lager.Logger, id string) (string, goci.Bndl, error)
	RemoveBundle(log lager.Logger, id string) error
}

type PeaCreator interface {
	CreatePea(log lager.Logger, processSpec garden.ProcessSpec, pio garden.ProcessIO, sandboxHandle string) (garden.Process, error)
}

type NstarRunner interface {
	StreamIn(log lager.Logger, pid int, path string, user string, tarStream io.Reader) error
	StreamOut(log lager.Logger, pid int, path string, user string) (io.ReadCloser, error)
}

type ProcessesStopper interface {
	StopAll(log lager.Logger, cgroupName string, save []int, kill bool) error
}

type RuntimeStopper interface {
	Stop() error
}

type EventStore interface {
	OnEvent(id string, event string) error
	Events(id string) []string
}

type StateStore interface {
	StoreStopped(handle string)
	IsStopped(handle string) bool
}

type PeaUsernameResolver interface {
	ResolveUser(log lager.Logger, handle string, image garden.ImageRef, username string) (int, int, error)
}

type CPUCgrouper interface {
	PrepareCgroups(handle string) error
	CleanupCgroups(handle string) error
	ReadTotalCgroupUsage(handle string, cpuStats garden.ContainerCPUStat) (garden.ContainerCPUStat, error)
}

// Containerizer knows how to manage a depot of container bundles
type Containerizer struct {
	depot                  Depot
	bundler                BundleGenerator
	runtime                OCIRuntime
	processesStopper       ProcessesStopper
	nstar                  NstarRunner
	events                 EventStore
	states                 StateStore
	peaCreator             PeaCreator
	peaUsernameResolver    PeaUsernameResolver
	cpuEntitlementPerShare float64
	runtimeStopper         RuntimeStopper
	cpuCgrouper            CPUCgrouper
}

func New(
	depot Depot,
	bundler BundleGenerator,
	runtime OCIRuntime,
	nstarRunner NstarRunner,
	processesStopper ProcessesStopper,
	events EventStore,
	states StateStore,
	peaCreator PeaCreator,
	peaUsernameResolver PeaUsernameResolver,
	cpuEntitlementPerShare float64,
	runtimeStopper RuntimeStopper,
	cpuCgrouper CPUCgrouper,
) *Containerizer {
	containerizer := &Containerizer{
		depot:                  depot,
		bundler:                bundler,
		runtime:                runtime,
		nstar:                  nstarRunner,
		processesStopper:       processesStopper,
		events:                 events,
		states:                 states,
		peaCreator:             peaCreator,
		peaUsernameResolver:    peaUsernameResolver,
		cpuEntitlementPerShare: cpuEntitlementPerShare,
		runtimeStopper:         runtimeStopper,
		cpuCgrouper:            cpuCgrouper,
	}
	return containerizer
}

func (c *Containerizer) WatchRuntimeEvents(log lager.Logger) error {
	events, err := c.runtime.Events(log)
	if err != nil {
		return err
	}

	go func() {
		for event := range events {
			if err := c.events.OnEvent(event.ContainerID, event.Message); err != nil {
				log.Error("failed to store event", err, lager.Data{"event": event})
			}
		}
	}()

	return nil
}

// Create creates a bundle in the depot and starts its init process
func (c *Containerizer) Create(log lager.Logger, spec spec.DesiredContainerSpec) error {
	log = log.Session("containerizer-create", lager.Data{"handle": spec.Handle})

	log.Info("start")
	defer log.Info("finished")

	bundle, err := c.bundler.Generate(spec)
	if err != nil {
		log.Error("bundle-generate-failed", err)
		return err
	}

	if err := c.cpuCgrouper.PrepareCgroups(spec.Handle); err != nil {
		log.Error("prepare-cgroups-failed", err)
		return err
	}

	if err := c.runtime.Create(log, spec.Handle, bundle, garden.ProcessIO{}); err != nil {
		log.Error("runtime-create-failed", err)
		return err
	}

	return nil
}

// Run runs a process inside a running container
func (c *Containerizer) Run(log lager.Logger, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("run", lager.Data{"handle": handle, "path": spec.Path})

	log.Info("started")
	defer log.Info("finished")

	if isPea(spec) {
		if shouldResolveUsername(spec.User) {
			resolvedUID, resolvedGID, err := c.peaUsernameResolver.ResolveUser(log, handle, spec.Image, spec.User)
			if err != nil {
				return nil, err
			}

			spec.User = fmt.Sprintf("%d:%d", resolvedUID, resolvedGID)
		}

		return c.peaCreator.CreatePea(log, spec, io, handle)
	}

	if spec.BindMounts != nil {
		err := fmt.Errorf("Running a process with bind mounts and no image provided is not allowed")
		log.Error("invalid-spec", err)
		return nil, err
	}

	return c.runtime.Exec(log, handle, spec, io)
}

func isPea(spec garden.ProcessSpec) bool {
	return spec.Image != (garden.ImageRef{})
}

func shouldResolveUsername(username string) bool {
	return username != "" && !strings.Contains(username, ":")
}

func (c *Containerizer) Attach(log lager.Logger, handle string, processID string, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("attach", lager.Data{"handle": handle, "process-id": processID})

	log.Info("started")
	defer log.Info("finished")

	return c.runtime.Attach(log, handle, processID, io)
}

// StreamIn streams files in to the container
func (c *Containerizer) StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error {
	log = log.Session("stream-in", lager.Data{"handle": handle})
	log.Info("started")
	defer log.Info("finished")

	defer func(startedAt time.Time) {
		_ = metrics.SendValue("StreamInDuration", float64(time.Since(startedAt).Nanoseconds()), "nanos")
	}(time.Now())

	state, err := c.runtime.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return fmt.Errorf("stream-in: pid not found for container")
	}

	if err := c.nstar.StreamIn(log, state.Pid, spec.Path, spec.User, spec.TarStream); err != nil {
		log.Error("nstar-failed", err)
		return fmt.Errorf("stream-in: nstar: %s", err)
	}

	return nil
}

// StreamOut stream files from the container
func (c *Containerizer) StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
	log = log.Session("stream-out", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.runtime.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return nil, fmt.Errorf("stream-out: pid not found for container")
	}

	stream, err := c.nstar.StreamOut(log, state.Pid, spec.Path, spec.User)
	if err != nil {
		log.Error("nstar-failed", err)
		return nil, fmt.Errorf("stream-out: nstar: %s", err)
	}

	return stream, nil
}

// Stop stops all the processes other than the init process in the container
func (c *Containerizer) Stop(log lager.Logger, handle string, kill bool) error {
	log = log.Session("stop", lager.Data{"handle": handle, "kill": kill})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.runtime.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return fmt.Errorf("stop: pid not found for container: %s", err)
	}

	if err = c.processesStopper.StopAll(log, handle, []int{state.Pid}, kill); err != nil {
		log.Error("stop-all-processes-failed", err, lager.Data{"pid": state.Pid})
		return fmt.Errorf("stop: %s", err)
	}

	c.states.StoreStopped(handle)
	return nil
}

// Destroy deletes the container and the bundle directory
func (c *Containerizer) Destroy(log lager.Logger, handle string) error {
	log = log.Session("destroy", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	if err := c.runtime.Delete(log, handle); err != nil {
		return err
	}

	return c.cpuCgrouper.CleanupCgroups(handle)
}

func (c *Containerizer) RemoveBundle(log lager.Logger, handle string) error {
	log = log.Session("remove-bundle", lager.Data{"handle": handle})
	// TODO: this should be removed once containerd processes are the only code path
	// and this should be managed by the network depot
	if err := c.depot.Destroy(log, handle); err != nil {
		log.Debug("failed-to-remove-bundle-dir")
	}
	return c.runtime.RemoveBundle(log, handle)
}

func (c *Containerizer) Info(log lager.Logger, handle string) (spec.ActualContainerSpec, error) {
	bundlePath, bundle, err := c.runtime.BundleInfo(log, handle)
	if err != nil {
		return spec.ActualContainerSpec{}, err
	}

	state, err := c.runtime.State(log, handle)
	if err != nil {
		return spec.ActualContainerSpec{}, err
	}

	privileged := true
	for _, ns := range bundle.Namespaces() {
		if ns.Type == specs.UserNamespace {
			privileged = false
			break
		}
	}

	var cpuShares, limitInBytes uint64
	if bundle.Resources() != nil {
		if bundle.Resources().CPU != nil {
			cpuShares = *bundle.Resources().CPU.Shares
		}
		if cpuWeight, ok := bundle.Resources().Unified["cpu.weight"]; ok {
			cpuSharesInt, err := strconv.Atoi(cpuWeight)
			if err != nil {
				return spec.ActualContainerSpec{}, err
			}
			cpuShares = uint64(cpuSharesInt)
		}
		if bundle.Resources().Memory != nil {
			// #nosec G115 - limits should never be negative
			limitInBytes = uint64(*bundle.Resources().Memory.Limit)
		}
		if memoryMax, ok := bundle.Resources().Unified["memory.max"]; ok {
			limitInBytesInt, err := strconv.Atoi(memoryMax)
			if err != nil {
				return spec.ActualContainerSpec{}, err
			}
			limitInBytes = uint64(limitInBytesInt)
		}
	} else {
		log.Debug("bundle-resources-is-nil", lager.Data{"bundle": bundle})
	}

	return spec.ActualContainerSpec{
		Pid:        state.Pid,
		BundlePath: bundlePath,
		RootFSPath: bundle.RootFS(),
		Events:     c.events.Events(handle),
		Stopped:    c.states.IsStopped(handle),
		Limits: garden.Limits{
			CPU: garden.CPULimits{
				LimitInShares: cpuShares,
			},
			Memory: garden.MemoryLimits{
				LimitInBytes: limitInBytes,
			},
		},
		Privileged: privileged,
	}, nil
}

func (c *Containerizer) Metrics(log lager.Logger, handle string) (gardener.ActualContainerMetrics, error) {
	containerMetrics, err := c.runtime.Stats(log, handle)
	if err != nil {
		return gardener.ActualContainerMetrics{}, err
	}

	totalCPUUsage, err := c.cpuCgrouper.ReadTotalCgroupUsage(handle, containerMetrics.CPU)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory") {
			totalCPUUsage = containerMetrics.CPU
		} else {
			return gardener.ActualContainerMetrics{}, err
		}
	}

	containerMetrics.CPU = garden.ContainerCPUStat{
		Usage:  totalCPUUsage.Usage,
		User:   totalCPUUsage.User,
		System: totalCPUUsage.System,
	}

	actualContainerMetrics := gardener.ActualContainerMetrics{
		StatsContainerMetrics: containerMetrics,
	}

	_, bundle, err := c.runtime.BundleInfo(log, handle)
	if isNotFound(err) { // pea
		return actualContainerMetrics, nil
	}
	if err != nil {
		return gardener.ActualContainerMetrics{}, err
	}

	actualContainerMetrics.CPUEntitlement = calculateCPUEntitlement(getShares(bundle), c.cpuEntitlementPerShare, containerMetrics.Age)

	return actualContainerMetrics, nil
}

func (c *Containerizer) Shutdown() error {
	return c.runtimeStopper.Stop()
}

func isNotFound(err error) bool {
	_, ok := err.(garden.ContainerNotFoundError)
	return ok
}

func (c *Containerizer) Handles() ([]string, error) {
	return c.runtime.ContainerHandles()
}

func calculateCPUEntitlement(shares uint64, entitlementPerShare float64, containerAge time.Duration) uint64 {
	return uint64(float64(shares) * (entitlementPerShare / 100) * float64(containerAge.Nanoseconds()))
}

func getShares(bundle goci.Bndl) uint64 {
	resources := bundle.Resources()
	if resources == nil {
		return 0
	}
	cpu := resources.CPU
	if cpu == nil {
		if cpuWeight, ok := resources.Unified["cpu.weight"]; ok {
			cpuWeightUint, err := strconv.ParseUint(cpuWeight, 10, 64)
			if err != nil {
				return 0
			}
			return ConvertCgroupV2ValueToCPUShares(cpuWeightUint)
		}

		return 0
	}
	shares := cpu.Shares
	if shares == nil {
		return 0
	}
	return *shares
}
