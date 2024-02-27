package gardener

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/lager/v3"
	"github.com/hashicorp/go-multierror"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . SysInfoProvider
//counterfeiter:generate . Containerizer
//counterfeiter:generate . Networker
//counterfeiter:generate . Volumizer
//counterfeiter:generate . VolumeCreator
//counterfeiter:generate . UidGenerator
//counterfeiter:generate . PropertyManager
//counterfeiter:generate . Restorer
//counterfeiter:generate . Starter
//counterfeiter:generate . BulkStarter
//counterfeiter:generate . PeaCleaner
//counterfeiter:generate . Sleeper

const ContainerInterfaceKey = "garden.network.interface"
const ContainerIPKey = "garden.network.container-ip"
const BridgeIPKey = "garden.network.host-ip"
const ExternalIPKey = "garden.network.external-ip"
const MappedPortsKey = "garden.network.mapped-ports"
const GraceTimeKey = "garden.grace-time"
const CleanupRetryLimit = 30
const CleanupRetrySleep = 3 * time.Second

const VolumizerSession = "volumizer"

type SysInfoProvider interface {
	TotalMemory() (uint64, error)
	TotalDisk() (uint64, error)
	CPUCores() (int, error)
}

type Containerizer interface {
	Create(log lager.Logger, desiredContainerSpec spec.DesiredContainerSpec) error
	Handles() ([]string, error)

	StreamIn(log lager.Logger, handle string, streamInSpec garden.StreamInSpec) error
	StreamOut(log lager.Logger, handle string, streamOutSpec garden.StreamOutSpec) (io.ReadCloser, error)

	Run(log lager.Logger, handle string, processSpec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(log lager.Logger, handle string, processGUID string, io garden.ProcessIO) (garden.Process, error)
	Stop(log lager.Logger, handle string, kill bool) error
	Destroy(log lager.Logger, handle string) error
	RemoveBundle(log lager.Logger, handle string) error

	Info(log lager.Logger, handle string) (spec.ActualContainerSpec, error)
	Metrics(log lager.Logger, handle string) (ActualContainerMetrics, error)
	WatchRuntimeEvents(log lager.Logger) error

	Shutdown() error
}

type Networker interface {
	SetupBindMounts(log lager.Logger, handle string, privileged bool, rootfsPath string) ([]garden.BindMount, error)
	Network(log lager.Logger, spec garden.ContainerSpec, pid int) error
	Capacity() uint64
	Destroy(log lager.Logger, handle string) error
	NetIn(log lager.Logger, handle string, hostPort, containerPort uint32) (uint32, uint32, error)
	BulkNetOut(log lager.Logger, handle string, rules []garden.NetOutRule) error
	NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error
	Restore(log lager.Logger, handle string) error
}

type Volumizer interface {
	Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error)
	VolumeDestroyMetricsGC
}

type VolumeDestroyMetricsGC interface {
	Destroy(log lager.Logger, handle string) error
	Metrics(log lager.Logger, handle string, namespaced bool) (garden.ContainerDiskStat, error)
	GC(log lager.Logger) error
	Capacity(log lager.Logger) (uint64, error)
}

type UidGenerator interface {
	Generate() string
}

type PropertyManager interface {
	All(handle string) (props garden.Properties, err error)
	Set(handle string, name string, value string)
	Remove(handle string, name string) error
	Get(handle string, name string) (string, bool)
	MatchesAll(handle string, props garden.Properties) bool
	DestroyKeySpace(string) error
}

type Starter interface {
	Start() error
}

type BulkStarter interface {
	StartAll() error
}

type Restorer interface {
	Restore(logger lager.Logger, handles []string) []string
}

type PeaCleaner interface {
	CleanAll(logger lager.Logger) error
	Clean(logger lager.Logger, handle string) error
}

type Sleeper func(time.Duration)

type UidGeneratorFunc func() string

func (fn UidGeneratorFunc) Generate() string {
	return fn()
}

type StatsContainerMetrics struct {
	CPU    garden.ContainerCPUStat
	Memory garden.ContainerMemoryStat
	Pid    garden.ContainerPidStat
	Age    time.Duration
}

type ActualContainerMetrics struct {
	StatsContainerMetrics
	CPUEntitlement uint64
}

// Gardener orchestrates other components to implement the Garden API
type Gardener struct {
	// SysInfoProvider returns total memory and total disk
	SysInfoProvider SysInfoProvider

	// Containerizer runs and manages linux containers
	Containerizer Containerizer

	// UidGenerator generates unique ids for containers
	UidGenerator UidGenerator

	// BulkStarter runs any needed Starters that do start-up tasks (e.g. setting up cgroups)
	BulkStarter BulkStarter

	// Networker creates a network for containers
	Networker Networker

	// Volumizer creates volumes for containers
	Volumizer Volumizer

	Logger lager.Logger

	// PropertyManager creates map of container properties
	PropertyManager PropertyManager

	Sleep Sleeper

	// MaxContainers limits the advertised container capacity
	MaxContainers uint64

	Restorer Restorer

	PeaCleaner PeaCleaner

	AllowPrivilgedContainers bool

	ContainerNetworkMetricsProvider ContainerNetworkMetricsProvider
}

func New(
	uidGenerator UidGenerator,
	bulkStarter BulkStarter,
	sysInfoProvider SysInfoProvider,
	networker Networker,
	volumizer Volumizer,
	containerizer Containerizer,
	propertyManager PropertyManager,
	restorer Restorer,
	peaCleaner PeaCleaner,
	logger lager.Logger,
	maxContainers uint64,
	allowPrivilegedContainers bool,
	containerNetworkMetricsProvider ContainerNetworkMetricsProvider,
) *Gardener {

	gdnr := Gardener{
		UidGenerator:                    uidGenerator,
		BulkStarter:                     bulkStarter,
		SysInfoProvider:                 sysInfoProvider,
		Networker:                       networker,
		Volumizer:                       volumizer,
		Containerizer:                   containerizer,
		PropertyManager:                 propertyManager,
		MaxContainers:                   maxContainers,
		Restorer:                        restorer,
		PeaCleaner:                      peaCleaner,
		AllowPrivilgedContainers:        allowPrivilegedContainers,
		Logger:                          logger,
		ContainerNetworkMetricsProvider: containerNetworkMetricsProvider,

		Sleep: time.Sleep,
	}
	return &gdnr
}

// Create creates a container by combining the results of networker.Network,
// volumizer.Create and containzer.Create.
func (g *Gardener) Create(containerSpec garden.ContainerSpec) (ctr garden.Container, err error) {
	if containerSpec.Handle == "" {
		containerSpec.Handle = g.UidGenerator.Generate()
	}

	log := g.Logger.Session("create", lager.Data{"handle": containerSpec.Handle})
	log.Info("start")

	defer func(startedAt time.Time) {
		_ = metrics.SendValue("ContainerCreationDuration", float64(time.Since(startedAt).Nanoseconds()), "nanos")
	}(time.Now())

	if !g.AllowPrivilgedContainers && containerSpec.Privileged {
		return nil, errors.New("privileged container creation is disabled")
	}

	knownHandles, err := g.Containerizer.Handles()
	if err != nil {
		return nil, err
	}

	if err := g.checkDuplicateHandle(knownHandles, containerSpec.Handle); err != nil {
		return nil, err
	}

	if err := g.checkMaxContainers(knownHandles); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			log := log.Session("create-failed-cleaningup", lager.Data{
				"cause": err.Error(),
			})

			log.Info("start")

			err := g.destroy(log, containerSpec.Handle)
			if err != nil {
				log.Error("destroy-failed", err)
			}

			log.Info("cleanedup")
		} else {
			log.Info("created")
		}
	}()

	if err := g.Volumizer.GC(log.Session(VolumizerSession)); err != nil {
		log.Error("graph-cleanup-failed", err)
	}

	container, err := g.Lookup(containerSpec.Handle)
	if err != nil {
		return nil, err
	}

	for name, value := range containerSpec.Properties {
		if err := container.SetProperty(name, value); err != nil {
			return nil, err
		}
	}

	runtimeSpec, err := g.Volumizer.Create(log, containerSpec)
	if err != nil {
		return nil, err
	}

	networkBindMounts, err := g.Networker.SetupBindMounts(log, containerSpec.Handle, containerSpec.Privileged, runtimeSpec.Root.Path)
	if err != nil {
		return nil, err
	}

	desiredSpec := spec.DesiredContainerSpec{
		Handle:     containerSpec.Handle,
		Hostname:   containerSpec.Handle,
		Privileged: containerSpec.Privileged,
		Env:        containerSpec.Env,
		BindMounts: append(containerSpec.BindMounts, networkBindMounts...),
		Limits:     containerSpec.Limits,
		BaseConfig: runtimeSpec,
	}

	if err := g.Containerizer.Create(log, desiredSpec); err != nil {
		return nil, err
	}

	actualSpec, err := g.Containerizer.Info(log, containerSpec.Handle)
	if err != nil {
		return nil, err
	}

	if actualSpec.Pid == 0 {
		err := errors.New("container init PID was 0")
		log.Error("checking-init-pid", err)
		return nil, err
	}

	if err = g.Networker.Network(log, containerSpec, actualSpec.Pid); err != nil {
		return nil, err
	}

	if containerSpec.GraceTime != 0 {
		if err := container.SetGraceTime(containerSpec.GraceTime); err != nil {
			return nil, err
		}
	}

	if err := container.SetProperty("garden.state", "created"); err != nil {
		return nil, err
	}

	return container, nil
}

func (g *Gardener) Lookup(handle string) (garden.Container, error) {
	return g.lookup(handle), nil
}

func (g *Gardener) lookup(handle string) garden.Container {
	return &container{
		logger:                 g.Logger,
		handle:                 handle,
		containerizer:          g.Containerizer,
		volumizer:              g.Volumizer,
		networker:              g.Networker,
		propertyManager:        g.PropertyManager,
		networkMetricsProvider: g.ContainerNetworkMetricsProvider,
	}
}

func (g *Gardener) Destroy(handle string) error {
	log := g.Logger.Session("destroy", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")

	handles, err := g.Containerizer.Handles()
	if err != nil {
		return err
	}

	if !exists(handles, handle) {
		return garden.ContainerNotFoundError{Handle: handle}
	}

	return g.destroy(log, handle)
}

// destroy idempotently destroys any resources associated with the given handle
func (g *Gardener) destroy(log lager.Logger, handle string) error {
	var errs *multierror.Error

	errs = multierror.Append(errs, g.Containerizer.Destroy(log, handle))

	errs = multierror.Append(errs, g.Networker.Destroy(log, handle))

	errs = multierror.Append(errs, g.Volumizer.Destroy(log.Session(VolumizerSession), handle))

	if err := errs.ErrorOrNil(); err != nil {
		// keep container metadata so that destroy can be retried
		// in case not all container resources were destroyed
		return err
	}

	// after metadata is deleted the container can no longer be listed by the client
	return g.deleteContainerMetadata(log, handle)
}

func (g *Gardener) deleteContainerMetadata(log lager.Logger, handle string) error {
	if err := g.Containerizer.RemoveBundle(log, handle); err != nil {
		return err
	}
	return g.PropertyManager.DestroyKeySpace(handle)
}

func (g *Gardener) Stop() error {
	return g.Containerizer.Shutdown()
}

func (g *Gardener) GraceTime(container garden.Container) time.Duration {
	property, ok := g.PropertyManager.Get(container.Handle(), GraceTimeKey)
	if !ok {
		return 0
	}

	var graceTime time.Duration
	_, err := fmt.Sscanf(property, "%d", &graceTime)
	if err != nil {
		return 0
	}

	return graceTime
}

func (g *Gardener) Ping() error { return nil }

func (g *Gardener) Capacity() (garden.Capacity, error) {
	log := g.Logger.Session("capacity")

	mem, err := g.SysInfoProvider.TotalMemory()
	if err != nil {
		return garden.Capacity{}, err
	}

	disk, err := g.SysInfoProvider.TotalDisk()
	if err != nil {
		return garden.Capacity{}, err
	}

	schedulableDisk, err := g.Volumizer.Capacity(log)
	if err != nil {
		log.Info("failed to retrieve schedulable disk capacity, falling back to total disk size", lager.Data{"err": err})
		schedulableDisk = disk
	}

	cap := g.Networker.Capacity()
	if g.MaxContainers > 0 && g.MaxContainers < cap {
		cap = g.MaxContainers
	}

	return garden.Capacity{
		MemoryInBytes:          mem,
		DiskInBytes:            disk,
		SchedulableDiskInBytes: schedulableDisk,
		MaxContainers:          cap,
	}, nil
}

func (g *Gardener) Containers(props garden.Properties) ([]garden.Container, error) {
	log := g.Logger.Session("list-containers")

	log.Info("starting")
	defer log.Info("finished")

	handles, err := g.Containerizer.Handles()
	if err != nil {
		log.Error("handles-failed", err)
		return []garden.Container{}, err
	}

	if props == nil {
		props = garden.Properties{}
	}
	if _, ok := props["garden.state"]; !ok {
		props["garden.state"] = "created"
	} else if props["garden.state"] == "all" {
		delete(props, "garden.state")
	}

	var containers []garden.Container
	for _, handle := range handles {
		matched := g.PropertyManager.MatchesAll(handle, props)
		if matched {
			containers = append(containers, g.lookup(handle))
		}
	}

	return containers, nil
}

func (g *Gardener) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	result := make(map[string]garden.ContainerInfoEntry)
	for _, handle := range handles {
		container := g.lookup(handle)

		var infoErr *garden.Error = nil
		info, err := container.Info()
		if err != nil {
			infoErr = garden.NewError(err.Error())
		}
		result[handle] = garden.ContainerInfoEntry{
			Info: info,
			Err:  infoErr,
		}
	}

	return result, nil
}

func (g *Gardener) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	result := make(map[string]garden.ContainerMetricsEntry)
	for _, handle := range handles {
		var e *garden.Error
		m, err := g.lookup(handle).Metrics()
		if err != nil {
			e = garden.NewError(err.Error())
		}

		result[handle] = garden.ContainerMetricsEntry{
			Err:     e,
			Metrics: m,
		}
	}

	return result, nil
}

func (g *Gardener) checkDuplicateHandle(knownHandles []string, handle string) error {
	if exists(knownHandles, handle) {
		return fmt.Errorf("Handle '%s' already in use", handle)
	}

	return nil
}

func exists(handles []string, handle string) bool {
	for _, h := range handles {
		if h == handle {
			return true
		}
	}

	return false
}

func (g *Gardener) checkMaxContainers(handles []string) error {
	if g.MaxContainers == 0 {
		return nil
	}

	if len(handles) >= int(g.MaxContainers) {
		return errors.New("max containers reached")
	}

	return nil
}

func (g *Gardener) Start() error {
	log := g.Logger.Session("start")

	log.Info("starting")
	defer log.Info("completed")

	if err := g.Containerizer.WatchRuntimeEvents(log); err != nil {
		return fmt.Errorf("watch runtime events: %s", err)
	}

	if err := g.BulkStarter.StartAll(); err != nil {
		return fmt.Errorf("bulk starter: %s", err)
	}

	if err := g.Cleanup(log); err != nil {
		return fmt.Errorf("cleanup: %s", err)
	}

	return nil
}

func (g *Gardener) Cleanup(log lager.Logger) error {
	if err := g.PeaCleaner.CleanAll(log); err != nil {
		return fmt.Errorf("clean peas: %s", err)
	}

	handles, err := g.Containerizer.Handles()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	for _, handle := range g.Restorer.Restore(log, handles) {
		wg.Add(1)
		go func(handle string) {
			defer wg.Done()
			destroyLog := log.Session("clean-up-container", lager.Data{"handle": handle})
			destroyLog.Info("start")

			for i := 0; i < CleanupRetryLimit; i++ {
				if err := g.destroy(destroyLog, handle); err != nil {
					destroyLog.Error(fmt.Sprintf("failed attempt %d", i+1), err)
					g.Sleep(CleanupRetrySleep)
					continue
				}
				destroyLog.Info("cleaned-up")
				return
			}
			destroyLog.Info(fmt.Sprintf("failed to cleanup container after %d attempts", CleanupRetryLimit))
		}(handle)
	}
	wg.Wait()

	return nil
}
