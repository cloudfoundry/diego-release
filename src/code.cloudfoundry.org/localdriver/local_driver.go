package localdriver

import (
	"errors"
	"fmt"
	"os"

	"strings"

	"code.cloudfoundry.org/dockerdriver"
	dockerdriverutils "code.cloudfoundry.org/dockerdriver/utils"
	"code.cloudfoundry.org/goshims/filepathshim"
	"code.cloudfoundry.org/goshims/osshim"
	"code.cloudfoundry.org/lager/v3"
)

const VolumesRootDir = "_volumes"
const MountsRootDir = "_mounts"

type LocalVolumeInfo struct {
	dockerdriver.VolumeInfo // see dockerdriver.resources.go
}

type OsHelper interface {
	Umask(mask int) (oldmask int)
}

type LocalDriver struct {
	volumes         map[string]*LocalVolumeInfo
	os              osshim.Os
	filepath        filepathshim.Filepath
	mountPathRoot   string
	osHelper        OsHelper
	uniqueVolumeIds bool
}

func NewLocalDriver(os osshim.Os, filepath filepathshim.Filepath, mountPathRoot string, osHelper OsHelper, uniqueVolumeIds bool) *LocalDriver {
	return &LocalDriver{
		volumes:         map[string]*LocalVolumeInfo{},
		os:              os,
		filepath:        filepath,
		mountPathRoot:   mountPathRoot,
		osHelper:        osHelper,
		uniqueVolumeIds: uniqueVolumeIds,
	}
}

func NewLocalDriverWithState(state map[string]*LocalVolumeInfo, os osshim.Os, filepath filepathshim.Filepath, mountPathRoot string, osHelper OsHelper, uniqueVolumeIds bool) *LocalDriver {
	return &LocalDriver{
		volumes:         state,
		os:              os,
		filepath:        filepath,
		mountPathRoot:   mountPathRoot,
		osHelper:        osHelper,
		uniqueVolumeIds: uniqueVolumeIds,
	}
}

func (d *LocalDriver) Activate(_ dockerdriver.Env) dockerdriver.ActivateResponse {
	return dockerdriver.ActivateResponse{
		Implements: []string{"VolumeDriver"},
	}
}

func (d *LocalDriver) Create(env dockerdriver.Env, createRequest dockerdriver.CreateRequest) dockerdriver.ErrorResponse {
	logger := env.Logger().Session("create")
	var ok bool
	if createRequest.Name == "" {
		return dockerdriver.ErrorResponse{Err: "Missing mandatory 'volume_name'"}
	}

	var existingVolume *LocalVolumeInfo
	if existingVolume, ok = d.volumes[createRequest.Name]; !ok {
		logger.Info("creating-volume", lager.Data{"volume_name": createRequest.Name, "volume_id": createRequest.Name})
		volInfo := LocalVolumeInfo{VolumeInfo: dockerdriver.VolumeInfo{Name: createRequest.Name}}
		d.volumes[createRequest.Name] = &volInfo

		createDir := d.volumePath(logger, createRequest.Name)
		logger.Info("creating-volume-folder", lager.Data{"volume": createDir})
		orig := d.osHelper.Umask(000)
		defer d.osHelper.Umask(orig)
		err := d.os.MkdirAll(createDir, os.ModePerm)
		if err != nil {
			logger.Fatal("failed-creating-path", err, lager.Data{"path": createDir})
		}

		return dockerdriver.ErrorResponse{}
	}

	if existingVolume.Name != createRequest.Name {
		logger.Info("duplicate-volume", lager.Data{"volume_name": createRequest.Name})
		return dockerdriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' already exists with a different volume ID", createRequest.Name)}
	}

	return dockerdriver.ErrorResponse{}
}

func (d *LocalDriver) List(env dockerdriver.Env) dockerdriver.ListResponse {
	listResponse := dockerdriver.ListResponse{}
	for _, volume := range d.volumes {
		listResponse.Volumes = append(listResponse.Volumes, volume.VolumeInfo)
	}
	listResponse.Err = ""
	return listResponse
}

func (d *LocalDriver) Mount(env dockerdriver.Env, mountRequest dockerdriver.MountRequest) dockerdriver.MountResponse {
	logger := env.Logger().Session("mount", lager.Data{"volume": mountRequest.Name})

	if mountRequest.Name == "" {
		return dockerdriver.MountResponse{Err: "Missing mandatory 'volume_name'"}
	}

	var vol *LocalVolumeInfo
	var ok bool
	if vol, ok = d.volumes[mountRequest.Name]; !ok {
		return dockerdriver.MountResponse{Err: fmt.Sprintf("Volume '%s' must be created before being mounted", mountRequest.Name)}
	}

	volumePath := d.volumePath(logger, vol.Name)

	exists, err := d.exists(volumePath)
	if err != nil {
		logger.Error("mount-volume-failed", err)
		return dockerdriver.MountResponse{Err: err.Error()}
	}

	if !exists {
		logger.Error("mount-volume-failed", errors.New("Volume '"+mountRequest.Name+"' is missing"))
		return dockerdriver.MountResponse{Err: "Volume '" + mountRequest.Name + "' is missing"}
	}

	mountPath := d.mountPath(logger, vol.Name)
	logger.Info("mounting-volume", lager.Data{"id": vol.Name, "mountpoint": mountPath})

	if vol.MountCount < 1 {
		err := d.mount(logger, volumePath, mountPath)
		if err != nil {
			logger.Error("mount-volume-failed", err)
			return dockerdriver.MountResponse{Err: fmt.Sprintf("Error mounting volume: %s", err.Error())}
		}
		vol.Mountpoint = mountPath
	}

	vol.MountCount++
	logger.Info("volume-mounted", lager.Data{"name": vol.Name, "count": vol.MountCount})

	mountResponse := dockerdriver.MountResponse{Mountpoint: vol.Mountpoint}
	return mountResponse
}

func (d *LocalDriver) Path(env dockerdriver.Env, pathRequest dockerdriver.PathRequest) dockerdriver.PathResponse {
	logger := env.Logger().Session("path", lager.Data{"volume": pathRequest.Name})

	if pathRequest.Name == "" {
		return dockerdriver.PathResponse{Err: "Missing mandatory 'volume_name'"}
	}

	mountPath, err := d.get(logger, pathRequest.Name)
	if err != nil {
		logger.Error("failed-no-such-volume-found", err, lager.Data{"mountpoint": mountPath})

		return dockerdriver.PathResponse{Err: fmt.Sprintf("Volume '%s' not found", pathRequest.Name)}
	}

	if mountPath == "" {
		errText := "Volume not previously mounted"
		logger.Error("failed-mountpoint-not-assigned", errors.New(errText))
		return dockerdriver.PathResponse{Err: errText}
	}

	return dockerdriver.PathResponse{Mountpoint: mountPath}
}

func (d *LocalDriver) Unmount(env dockerdriver.Env, unmountRequest dockerdriver.UnmountRequest) dockerdriver.ErrorResponse {
	logger := env.Logger().Session("unmount", lager.Data{"volume": unmountRequest.Name})

	if unmountRequest.Name == "" {
		return dockerdriver.ErrorResponse{Err: "Missing mandatory 'volume_name'"}
	}

	mountPath, err := d.get(logger, unmountRequest.Name)
	if err != nil {
		logger.Error("failed-no-such-volume-found", err, lager.Data{"mountpoint": mountPath})

		return dockerdriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' not found", unmountRequest.Name)}
	}

	if mountPath == "" {
		errText := "Volume not previously mounted"
		logger.Error("failed-mountpoint-not-assigned", errors.New(errText))
		return dockerdriver.ErrorResponse{Err: errText}
	}

	return d.unmount(logger, unmountRequest.Name, mountPath)
}

func (d *LocalDriver) Remove(env dockerdriver.Env, removeRequest dockerdriver.RemoveRequest) dockerdriver.ErrorResponse {
	logger := env.Logger().Session("remove", lager.Data{"volume": removeRequest})
	logger.Info("start")
	defer logger.Info("end")

	if removeRequest.Name == "" {
		return dockerdriver.ErrorResponse{Err: "Missing mandatory 'volume_name'"}
	}

	var response dockerdriver.ErrorResponse
	var vol *LocalVolumeInfo
	var exists bool
	if vol, exists = d.volumes[removeRequest.Name]; !exists {
		logger.Error("failed-volume-removal", fmt.Errorf("Volume %s not found", removeRequest.Name))
		return dockerdriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' not found", removeRequest.Name)}
	}

	if vol.Mountpoint != "" {
		response = d.unmount(logger, removeRequest.Name, vol.Mountpoint)
		if response.Err != "" {
			return response
		}
	}

	volumePath := d.volumePath(logger, vol.Name)

	logger.Info("remove-volume-folder", lager.Data{"volume": volumePath})
	err := d.os.RemoveAll(volumePath)
	if err != nil {
		logger.Error("failed-removing-volume", err)
		return dockerdriver.ErrorResponse{Err: fmt.Sprintf("Failed removing mount path: %s", err)}
	}

	logger.Info("removing-volume", lager.Data{"name": removeRequest.Name})
	delete(d.volumes, removeRequest.Name)
	return dockerdriver.ErrorResponse{}
}

func (d *LocalDriver) Get(env dockerdriver.Env, getRequest dockerdriver.GetRequest) dockerdriver.GetResponse {
	logger := env.Logger().Session("Get")
	mountpoint, err := d.get(logger, getRequest.Name)
	if err != nil {
		return dockerdriver.GetResponse{Err: err.Error()}
	}

	return dockerdriver.GetResponse{Volume: dockerdriver.VolumeInfo{Name: getRequest.Name, Mountpoint: mountpoint}}
}

func (d *LocalDriver) get(logger lager.Logger, volumeName string) (string, error) {
	if vol, ok := d.volumes[volumeName]; ok {
		logger.Info("getting-volume", lager.Data{"name": volumeName})
		return vol.Mountpoint, nil
	}

	return "", errors.New("Volume not found")
}

func (d *LocalDriver) Capabilities(_ dockerdriver.Env) dockerdriver.CapabilitiesResponse {
	return dockerdriver.CapabilitiesResponse{
		Capabilities: dockerdriver.CapabilityInfo{Scope: "local"},
	}
}

func (d *LocalDriver) exists(path string) (bool, error) {
	_, err := d.os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (d *LocalDriver) mountPath(logger lager.Logger, volumeId string) string {
	dir, err := d.filepath.Abs(d.mountPathRoot)
	if err != nil {
		logger.Fatal("abs-failed", err)
	}

	if !strings.HasSuffix(dir, string(os.PathSeparator)) {
		dir = fmt.Sprintf("%s%s", dir, string(os.PathSeparator))
	}

	mountsPathRoot := fmt.Sprintf("%s%s", dir, MountsRootDir)
	orig := d.osHelper.Umask(000)
	defer d.osHelper.Umask(orig)
	err = d.os.MkdirAll(mountsPathRoot, os.ModePerm)
	if err != nil {
		logger.Fatal("failed-creating-path", err, lager.Data{"path": mountsPathRoot})
	}

	return fmt.Sprintf("%s%s%s", mountsPathRoot, string(os.PathSeparator), volumeId)
}

func (d *LocalDriver) volumePath(logger lager.Logger, volumeId string) string {
	dir, err := d.filepath.Abs(d.mountPathRoot)
	if err != nil {
		logger.Fatal("abs-failed", err)
	}

	volumesPathRoot := d.filepath.Join(dir, VolumesRootDir)
	orig := d.osHelper.Umask(000)
	defer d.osHelper.Umask(orig)
	err = d.os.MkdirAll(volumesPathRoot, os.ModePerm)
	if err != nil {
		logger.Fatal("failed-creating-path", err, lager.Data{"path": volumesPathRoot})
	}

	if d.uniqueVolumeIds {
		uniqueVolumeId, err := dockerdriverutils.NewVolumeIdFromEncodedString(volumeId)
		if err != nil {
			logger.Fatal("decode-unique-volume-id-failed", err)
		}

		volumeId = uniqueVolumeId.Prefix
	}

	return d.filepath.Join(volumesPathRoot, volumeId)
}

func (d *LocalDriver) mount(logger lager.Logger, volumePath, mountPath string) error {
	logger.Info("link", lager.Data{"src": volumePath, "tgt": mountPath})
	orig := d.osHelper.Umask(000)
	defer d.osHelper.Umask(orig)
	return d.os.Symlink(volumePath, mountPath)
}

func (d *LocalDriver) unmount(logger lager.Logger, name string, mountPath string) dockerdriver.ErrorResponse {
	logger = logger.Session("unmount")
	logger.Info("start")
	defer logger.Info("end")

	exists, err := d.exists(mountPath)
	if err != nil {
		logger.Error("failed-retrieving-mount-info", err, lager.Data{"mountpoint": mountPath})
		return dockerdriver.ErrorResponse{Err: "Error establishing whether volume exists"}
	}

	if !exists {
		errText := fmt.Sprintf("Volume %s does not exist (path: %s), nothing to do!", name, mountPath)
		logger.Error("failed-mountpoint-not-found", errors.New(errText))
		return dockerdriver.ErrorResponse{Err: errText}
	}

	d.volumes[name].MountCount--
	if d.volumes[name].MountCount > 0 {
		logger.Info("volume-still-in-use", lager.Data{"name": name, "count": d.volumes[name].MountCount})
		return dockerdriver.ErrorResponse{}
	} else {
		logger.Info("unmount-volume-folder", lager.Data{"mountpath": mountPath})
		err := d.os.Remove(mountPath)
		if err != nil {
			logger.Error("unmount-failed", err)
			return dockerdriver.ErrorResponse{Err: fmt.Sprintf("Error unmounting volume: %s", err.Error())}
		}
	}

	logger.Info("unmounted-volume")

	d.volumes[name].Mountpoint = ""

	return dockerdriver.ErrorResponse{}
}
