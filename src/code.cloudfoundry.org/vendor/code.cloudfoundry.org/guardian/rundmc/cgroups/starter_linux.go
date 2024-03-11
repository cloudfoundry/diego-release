package cgroups

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/cgroups/fs"
	"code.cloudfoundry.org/lager/v3"
)

const (
	Root   = "/sys/fs/cgroup"
	Garden = "garden"
	Header = "#subsys_name hierarchy num_cgroups enabled"
)

type CgroupsFormatError struct {
	Content string
}

func (err CgroupsFormatError) Error() string {
	return fmt.Sprintf("unknown /proc/cgroups format: %s", err.Content)
}

func NewStarter(
	logger lager.Logger,
	procCgroupReader io.ReadCloser,
	procSelfCgroupReader io.ReadCloser,
	cgroupMountpoint string,
	gardenCgroup string,
	allowedDevices []specs.LinuxDeviceCgroup,
	mountPointChecker rundmc.MountPointChecker,
	enableCPUThrottling bool,
) *CgroupStarter {
	return &CgroupStarter{
		CgroupPath:        cgroupMountpoint,
		GardenCgroup:      gardenCgroup,
		ProcCgroups:       procCgroupReader,
		ProcSelfCgroups:   procSelfCgroupReader,
		CPUThrottling:     enableCPUThrottling,
		AllowedDevices:    allowedDevices,
		Logger:            logger,
		MountPointChecker: mountPointChecker,
		FS:                fs.Functions(),
	}
}

type CgroupStarter struct {
	CgroupPath      string
	GardenCgroup    string
	AllowedDevices  []specs.LinuxDeviceCgroup
	ProcCgroups     io.ReadCloser
	ProcSelfCgroups io.ReadCloser
	CPUThrottling   bool

	Logger            lager.Logger
	MountPointChecker rundmc.MountPointChecker
	FS                fs.FS

	uid *int
	gid *int
}

func (s *CgroupStarter) WithUID(uid int) *CgroupStarter {
	s.uid = &uid
	return s
}

func (s *CgroupStarter) WithGID(gid int) *CgroupStarter {
	s.gid = &gid
	return s
}

func (s *CgroupStarter) Start() error {
	return s.mountCgroupsIfNeeded(s.Logger)
}

func (s *CgroupStarter) mountCgroupsIfNeeded(logger lager.Logger) error {
	defer s.ProcCgroups.Close()
	defer s.ProcSelfCgroups.Close()
	if err := os.MkdirAll(s.CgroupPath, 0755); err != nil {
		return err
	}

	mountPoint, err := s.MountPointChecker.IsMountPoint(s.CgroupPath)
	if err != nil {
		return err
	}
	if !mountPoint {
		s.mountTmpfsOnCgroupPath(logger, s.CgroupPath)
	} else {
		logger.Info("cgroups-tmpfs-already-mounted", lager.Data{"path": s.CgroupPath})
	}

	subsystemGroupings, err := s.subsystemGroupings()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(s.ProcCgroups)

	if !scanner.Scan() {
		return CgroupsFormatError{Content: "(empty)"}
	}

	if _, err := fmt.Sscanf(scanner.Text(), Header); err != nil {
		return CgroupsFormatError{Content: scanner.Text()}
	}

	kernelSubsystems := []string{}
	for scanner.Scan() {
		var subsystem string
		var skip, enabled int
		n, err := fmt.Sscanf(scanner.Text(), "%s %d %d %d ", &subsystem, &skip, &skip, &enabled)
		if err != nil || n != 4 {
			return CgroupsFormatError{Content: scanner.Text()}
		}

		kernelSubsystems = append(kernelSubsystems, subsystem)

		if enabled == 0 {
			continue
		}

		subsystemToMount, dirToCreate := subsystem, s.GardenCgroup
		if v, ok := subsystemGroupings[subsystem]; ok {
			subsystemToMount = v.SubSystem
			dirToCreate = path.Join(v.Path, s.GardenCgroup)
		}

		subsystemMountPath := path.Join(s.CgroupPath, subsystem)
		gardenCgroupPath := filepath.Join(subsystemMountPath, dirToCreate)
		if err := s.createAndChownCgroup(logger, subsystemMountPath, subsystemToMount, gardenCgroupPath); err != nil {
			return err
		}

		if subsystem == "devices" {
			if s.CPUThrottling {
				if err := s.modifyAllowedDevices(filepath.Join(gardenCgroupPath, GoodCgroupName), s.AllowedDevices); err != nil {
					return err
				}
			} else {
				if err := s.modifyAllowedDevices(gardenCgroupPath, s.AllowedDevices); err != nil {
					return err
				}
			}
		}
	}

	for _, subsystem := range namedCgroupSubsystems(procSelfSubsystems(subsystemGroupings), kernelSubsystems) {
		cgroup := subsystemGroupings[subsystem]
		subsystemName := cgroup.SubSystem[len("name="):len(cgroup.SubSystem)]
		subsystemMountPath := path.Join(s.CgroupPath, subsystemName)
		gardenCgroupPath := filepath.Join(subsystemMountPath, cgroup.Path, s.GardenCgroup)

		if err := s.createAndChownCgroup(logger, subsystemMountPath, subsystem, gardenCgroupPath); err != nil {
			return err
		}
	}

	return nil
}

func (s *CgroupStarter) createAndChownCgroup(logger lager.Logger, mountPath, subsystem, gardenCgroupPath string) error {
	if err := s.idempotentCgroupMount(logger, mountPath, subsystem); err != nil {
		return err
	}

	if err := s.createChownedCgroup(logger, gardenCgroupPath); err != nil {
		return err
	}

	if s.CPUThrottling {
		if err := s.createChownedCgroup(logger, filepath.Join(gardenCgroupPath, GoodCgroupName)); err != nil {
			return err
		}

		if contains(strings.Split(subsystem, ","), "cpu") {
			return s.createChownedCgroup(logger, filepath.Join(gardenCgroupPath, BadCgroupName))
		}
	}

	return nil
}

func (s *CgroupStarter) createChownedCgroup(logger lager.Logger, cgroupPath string) error {
	if err := s.createGardenCgroup(logger, cgroupPath); err != nil {
		return err
	}

	return s.recursiveChown(cgroupPath)
}

func procSelfSubsystems(m map[string]group) []string {
	result := []string{}
	for k := range m {
		result = append(result, k)
	}

	return result
}

func subtract(from, values []string) []string {
	result := []string{}
	for _, v := range from {
		if !contains(values, v) {
			result = append(result, v)
		}
	}

	return result
}

func namedCgroupSubsystems(procSelfSubsystems, kernelSubsystems []string) []string {
	result := []string{}
	for _, subsystem := range subtract(procSelfSubsystems, kernelSubsystems) {
		if strings.HasPrefix(subsystem, "name=") {
			result = append(result, subsystem)
		}
	}

	return result
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (s *CgroupStarter) modifyAllowedDevices(dir string, devices []specs.LinuxDeviceCgroup) error {
	if has, err := hasSubdirectories(dir); err != nil {
		return err
	} else if has {
		return nil
	}

	if err := os.WriteFile(filepath.Join(dir, "devices.deny"), []byte("a"), 0770); err != nil {
		return err
	}

	for _, device := range devices {
		data := fmt.Sprintf("%s %s:%s %s", device.Type, s.deviceNumberString(device.Major), s.deviceNumberString(device.Minor), device.Access)
		if err := s.setDeviceCgroup(dir, "devices.allow", data); err != nil {
			return err
		}
	}

	return nil
}

func hasSubdirectories(dir string) (bool, error) {
	dirs, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, fileInfo := range dirs {
		if fileInfo.Type().IsDir() {
			return true, nil
		}
	}
	return false, nil
}

func (d *CgroupStarter) setDeviceCgroup(dir, file, data string) error {
	if err := os.WriteFile(filepath.Join(dir, file), []byte(data), 0); err != nil {
		return fmt.Errorf("failed to write %s to %s: %v", data, file, err)
	}

	return nil
}

func (s *CgroupStarter) deviceNumberString(number *int64) string {
	if *number == -1 {
		return "*"
	}
	return fmt.Sprint(*number)
}

func (s *CgroupStarter) createGardenCgroup(log lager.Logger, gardenCgroupPath string) error {
	log = log.Session("creating-garden-cgroup", lager.Data{"gardenCgroup": gardenCgroupPath})
	log.Info("started")
	defer log.Info("finished")

	if err := os.MkdirAll(gardenCgroupPath, 0755); err != nil {
		return err
	}

	return os.Chmod(gardenCgroupPath, 0755)
}

func (s *CgroupStarter) mountTmpfsOnCgroupPath(log lager.Logger, path string) {
	log = log.Session("cgroups-tmpfs-mounting", lager.Data{"path": path})
	log.Info("started")

	if err := s.FS.Mount("cgroup", path, "tmpfs", uintptr(0), "uid=0,gid=0,mode=0755"); err != nil {
		log.Error("mount-failed-continuing-anyway", err)
		return
	}
	log.Info("finished")
}

type group struct {
	SubSystem string
	Path      string
}

func (s *CgroupStarter) subsystemGroupings() (map[string]group, error) {
	groupings := map[string]group{}

	scanner := bufio.NewScanner(s.ProcSelfCgroups)
	for scanner.Scan() {
		segs := strings.Split(scanner.Text(), ":")
		if len(segs) != 3 {
			continue
		}

		subsystems := strings.Split(segs[1], ",")
		for _, subsystem := range subsystems {
			groupings[subsystem] = group{segs[1], segs[2]}
		}
	}

	return groupings, scanner.Err()
}

func (s *CgroupStarter) idempotentCgroupMount(logger lager.Logger, cgroupPath, subsystem string) error {
	logger = logger.Session("mount-cgroup", lager.Data{
		"path":      cgroupPath,
		"subsystem": subsystem,
	})

	logger.Info("started")

	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return fmt.Errorf("mkdir '%s': %s", cgroupPath, err)
	}

	err := s.FS.Mount("cgroup", cgroupPath, "cgroup", uintptr(0), subsystem)
	switch err {
	case nil:
	case unix.EBUSY:
		// Attempting a mount over an exising mount of type cgroup and the same
		// source and target results in EBUSY errno
		logger.Info("subsystem-already-mounted")
	default:
		return fmt.Errorf("mounting subsystem '%s' in '%s': %s", subsystem, cgroupPath, err)
	}

	logger.Info("finished")

	return nil
}

func (s *CgroupStarter) recursiveChown(path string) error {
	if (s.uid == nil) != (s.gid == nil) {
		return errors.New("either both UID and GID must be nil, or neither can be nil")
	}

	if s.uid == nil || s.gid == nil {
		return nil
	}

	return filepath.Walk(path, func(name string, info os.FileInfo, statErr error) error {
		if statErr != nil {
			return statErr
		}

		return s.FS.Chown(name, *s.uid, *s.gid)
	})
}
