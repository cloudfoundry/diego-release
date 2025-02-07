package goci

import (
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Bndl represents an in-memory OCI bundle
type Bndl struct {
	Spec specs.Spec
}

// Bundle creates a Bndl
func Bundle() Bndl {
	return Bndl{
		Spec: specs.Spec{
			Version: "1.0.0",
			Linux:   &specs.Linux{},
			Windows: &specs.Windows{
				Network:      &specs.WindowsNetwork{},
				LayerFolders: []string{},
			},
			Process: &specs.Process{
				ConsoleSize: &specs.Box{},
			},
			Root: &specs.Root{},
		},
	}
}

var (
	NetworkNamespace = specs.LinuxNamespace{Type: specs.NetworkNamespace}
	UserNamespace    = specs.LinuxNamespace{Type: specs.UserNamespace}
	PIDNamespace     = specs.LinuxNamespace{Type: specs.PIDNamespace}
	IPCNamespace     = specs.LinuxNamespace{Type: specs.IPCNamespace}
	UTSNamespace     = specs.LinuxNamespace{Type: specs.UTSNamespace}
	MountNamespace   = specs.LinuxNamespace{Type: specs.MountNamespace}
	CgroupNamespace  = specs.LinuxNamespace{Type: specs.CgroupNamespace}
)

// WithProcess returns a bundle with the process replaced with the given Process. The original bundle is not modified.
func (b Bndl) WithProcess(process specs.Process) Bndl {
	b.Spec.Process = &process
	return b
}

func (b Bndl) CGroupPath() string {
	return b.Spec.Linux.CgroupsPath
}

func (b Bndl) WithCGroupPath(path string) Bndl {
	b.Spec.Linux.CgroupsPath = path
	return b
}

func (b Bndl) Hostname() string {
	return b.Spec.Hostname
}

func (b Bndl) WithHostname(hostname string) Bndl {
	b.Spec.Hostname = hostname
	return b
}

func (b Bndl) Process() specs.Process {
	return *(b.Spec.Process)
}

func (b Bndl) WithApparmorProfile(profile string) Bndl {
	b.CloneProcess().Spec.Process.ApparmorProfile = profile
	return b
}

func (b Bndl) WithRootFS(absolutePath string) Bndl {
	b.Spec.Root = &specs.Root{Path: absolutePath}
	return b
}

func (b Bndl) RootFSPropagation() string {
	return b.Spec.Linux.RootfsPropagation
}

func (b Bndl) WithRootFSPropagation(rootfsPropagation string) Bndl {
	b.Spec.Linux.RootfsPropagation = rootfsPropagation
	return b
}

// RootFS returns the path to the rootfs of this bundle. Nothing is modified
func (b Bndl) RootFS() string {
	return b.Spec.Root.Path
}

func (b Bndl) WithWindows(windows specs.Windows) Bndl {
	b.Spec.Windows = &windows
	return b
}

func (b Bndl) Resources() *specs.LinuxResources {
	return b.Spec.Linux.Resources
}

func (b Bndl) WindowsResources() *specs.WindowsResources {
	return b.Spec.Windows.Resources
}

func (b Bndl) WithBlockIO(blockIO specs.LinuxBlockIO) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.LinuxResources{}
	}

	resources.BlockIO = &blockIO
	b.CloneLinux().Spec.Linux.Resources = resources

	return b
}

func (b Bndl) WithCPUShares(shares specs.LinuxCPU) Bndl {
	b = b.setCPUShares(shares)
	return b
}

func (b Bndl) WithWindowsCPUShares(shares specs.WindowsCPUResources) Bndl {
	resources := b.WindowsResources()
	if resources == nil {
		resources = &specs.WindowsResources{}
	}

	resources.CPU = &shares
	b.CloneWindows().Spec.Windows.Resources = resources

	return b
}

func (b Bndl) WithMemoryLimit(limit specs.LinuxMemory) Bndl {
	b = b.setMemoryLimit(limit)
	return b
}

func (b Bndl) WithWindowsMemoryLimit(limit specs.WindowsMemoryResources) Bndl {
	resources := b.WindowsResources()
	if resources == nil {
		resources = &specs.WindowsResources{}
	}

	resources.Memory = &limit
	b.CloneWindows().Spec.Windows.Resources = resources

	return b
}

func (b Bndl) WithPidLimit(limit specs.LinuxPids) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.LinuxResources{}
	}

	resources.Pids = &limit
	b.CloneLinux().Spec.Linux.Resources = resources

	return b
}

func (b Bndl) WithDeviceRestrictions(deviceRestrictions []specs.LinuxDeviceCgroup) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.LinuxResources{}
	}

	resources.Devices = deviceRestrictions
	b.CloneLinux().Spec.Linux.Resources = resources

	return b
}

// WithNamespace returns a bundle with the given namespace in the list of namespaces. The bundle is not modified, but any
// existing namespace of this type will be replaced.
func (b Bndl) WithNamespace(ns specs.LinuxNamespace) Bndl {
	slice := NamespaceSlice(b.Spec.Linux.Namespaces)
	b.CloneLinux().Spec.Linux.Namespaces = []specs.LinuxNamespace(slice.Set(ns))
	return b
}

func (b Bndl) Namespaces() []specs.LinuxNamespace {
	return b.Spec.Linux.Namespaces
}

func (b Bndl) WithUIDMappings(mappings ...specs.LinuxIDMapping) Bndl {
	b.CloneLinux().Spec.Linux.UIDMappings = mappings
	return b
}

func (b Bndl) UIDMappings() []specs.LinuxIDMapping {
	return b.Spec.Linux.UIDMappings
}

func (b Bndl) WithGIDMappings(mappings ...specs.LinuxIDMapping) Bndl {
	b.CloneLinux().Spec.Linux.GIDMappings = mappings
	return b
}

func (b Bndl) GIDMappings() []specs.LinuxIDMapping {
	return b.Spec.Linux.GIDMappings
}

func (b Bndl) WithPoststopHooks(hook ...specs.Hook) Bndl {
	b.Spec.Hooks = &specs.Hooks{Poststop: hook}
	return b
}

func (b Bndl) PoststopHooks() []specs.Hook {
	return b.Spec.Hooks.Poststop
}

// WithNamespaces returns a bundle with the given namespaces. The original bundle is not modified, but the original
// set of namespaces is replaced in the returned bundle.
func (b Bndl) WithNamespaces(namespaces ...specs.LinuxNamespace) Bndl {
	b.CloneLinux().Spec.Linux.Namespaces = namespaces
	return b
}

// WithDevices returns a bundle with the given devices added. The original bundle is not modified.
func (b Bndl) WithDevices(devices ...specs.LinuxDevice) Bndl {
	b.CloneLinux().Spec.Linux.Devices = devices
	return b
}

func (b Bndl) Devices() []specs.LinuxDevice {
	return b.Spec.Linux.Devices
}

// WithCapabilities returns a bundle with the given capabilities added. The original bundle is not modified.
func (b Bndl) WithCapabilities(capabilities ...string) Bndl {
	caps := &specs.LinuxCapabilities{
		Bounding:    capabilities,
		Inheritable: capabilities,
		Permitted:   capabilities,
	}
	b.CloneProcess().Spec.Process.Capabilities = caps
	return b
}

func (b Bndl) Capabilities() []string {
	if b.Spec.Process.Capabilities == nil {
		return []string{}
	}

	return b.Spec.Process.Capabilities.Bounding
}

// WithMounts returns a bundle with the given mounts appended. The original bundle is not modified.
func (b Bndl) WithMounts(mounts ...specs.Mount) Bndl {
	b.Spec.Mounts = append(b.Spec.Mounts, mounts...)
	return b
}

// WithPrependedMounts returns a bundle with the given mounts prepended. The original bundle is not modified.
func (b Bndl) WithPrependedMounts(mounts ...specs.Mount) Bndl {
	b.Spec.Mounts = append(mounts, b.Spec.Mounts...)
	return b
}

func (b Bndl) Mounts() []specs.Mount {
	return b.Spec.Mounts
}

func (b Bndl) WithMaskedPaths(maskedPaths []string) Bndl {
	b.CloneLinux().Spec.Linux.MaskedPaths = maskedPaths
	return b
}

func (b Bndl) MaskedPaths() []string {
	return b.Spec.Linux.MaskedPaths
}

type NamespaceSlice []specs.LinuxNamespace

func (slice NamespaceSlice) Set(ns specs.LinuxNamespace) NamespaceSlice {
	for i, namespace := range slice {
		if namespace.Type == ns.Type {
			slice[i] = ns
			return slice
		}
	}

	return append(slice, ns)
}

// Process returns an OCI Process struct with the given args.
func Process(args ...string) specs.Process {
	return specs.Process{Args: args}
}

func (b *Bndl) CloneLinux() Bndl {
	l := copy(*b.Spec.Linux)
	b.Spec.Linux = &l
	return *b
}
func (b *Bndl) CloneWindows() Bndl {
	l := copyWindows(*b.Spec.Windows)
	b.Spec.Windows = &l
	return *b
}

func (b *Bndl) CloneProcess() Bndl {
	l := (*b.Spec.Process)
	b.Spec.Process = &l
	return *b
}

func copy(l specs.Linux) specs.Linux {
	return l
}

func copyWindows(l specs.Windows) specs.Windows {
	return l
}
