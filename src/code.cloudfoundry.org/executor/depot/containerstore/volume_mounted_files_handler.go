package containerstore

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o containerstorefakes/fake_volume_mounted_files_handler.go . VolumeMountedFilesImplementor
type VolumeMountedFilesImplementor interface {
	CreateDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, error)
	RemoveDir(logger lager.Logger, container executor.Container) error
}

type VolumeMountedFilesHandler struct {
	fsOperations       FSOperator
	containerMountPath string
	volumeMountPath    string
}

func NewVolumeMountedFilesHandler(
	fsOperations FSOperator,
	volumeMountPath string,
	containerMountPath string,
) *VolumeMountedFilesHandler {
	return &VolumeMountedFilesHandler{
		fsOperations:       fsOperations,
		volumeMountPath:    volumeMountPath,
		containerMountPath: containerMountPath,
	}
}

func (h *VolumeMountedFilesHandler) CreateDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, error) {
	err := h.fsOperations.Chdir(h.volumeMountPath)
	if err != nil {
		return nil, fmt.Errorf("volume mount path does not exist %w", err)
	}

	containerDir := filepath.Join(h.volumeMountPath, container.Guid)

	err = h.fsOperations.MkdirAll(container.Guid)
	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("creating container dir: %s", containerDir))

	errServiceBinding := h.volumeMountedFilesForServices(logger, containerDir, container)
	if errServiceBinding != nil {
		logger.Error("creating-dir-volume-mounted-files-failed", errServiceBinding)
		return nil, errServiceBinding
	}

	return []garden.BindMount{
		{
			SrcPath: containerDir,
			DstPath: h.containerMountPath,
			Mode:    garden.BindMountModeRO,
			Origin:  garden.BindMountOriginHost,
		},
	}, nil
}

func (h *VolumeMountedFilesHandler) volumeMountedFilesForServices(
	logger lager.Logger,
	containerDir string,
	containers executor.Container,
) error {
	var cleanupFiles []*os.File

	for _, roots := range containers.RunInfo.VolumeMountedFiles {
		dirName := filepath.Dir(roots.Path)   // if path is empty the dirName will be "."
		fileName := filepath.Base(roots.Path) // if filename is empty the fileName will be "."

		if dirName == "." && fileName == "." {
			err := fmt.Errorf("failed to extract volume-mounted-files directory. format is: /serviceName/fileName")
			logger.Error(" volume-mounted-files directory is required", err, lager.Data{"dirName": "is empty"})

			return err
		}

		dirName = filepath.Join(containerDir, dirName)

		err := h.fsOperations.MkdirAll(dirName)
		if err != nil {
			logger.Error("failed-to-create-directory", err, lager.Data{"dirName": dirName})
			return fmt.Errorf("failed to create directory %s: %w", dirName, err)
		}

		filePath := filepath.Join(dirName, fileName)

		serviceFile, err := h.fsOperations.CreateFile(filePath)
		if err != nil {
			logger.Error("failed-to-create-file", err, lager.Data{"filePath": filePath})
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}

		cleanupFiles = append(cleanupFiles, serviceFile)

		err = h.fsOperations.WriteFile(filePath, []byte(roots.Content))
		if err != nil {
			logger.Error("failed-to-write-to-file", err, lager.Data{"filePath": filePath})

			return fmt.Errorf("failed to write to file %s: %w", filePath, err)
		}
	}

	for _, file := range cleanupFiles {
		err := file.Close()
		if err != nil {
			logger.Error("failed-to-close-file", err, lager.Data{"filePath": file.Name()})
			return fmt.Errorf("failed to close file %s: %w", file.Name(), err)
		}
	}

	return nil
}

func (h *VolumeMountedFilesHandler) RemoveDir(logger lager.Logger, container executor.Container) error {
	path := filepath.Join(h.volumeMountPath, container.Guid)

	err := h.fsOperations.RemoveAll(path)
	if err != nil {
		errMsg := "failed-to-remove-volume-mounted-files-directory"
		logger.Error(errMsg, err, lager.Data{"directory": path})

		err = fmt.Errorf("%s, %s", errMsg, err.Error())
	}

	return err
}

//go:generate counterfeiter -o containerstorefakes/fake_fs_operations.go . FSOperator
type FSOperator interface {
	CreateFile(filePath string) (*os.File, error)
	WriteFile(name string, data []byte) error
	MkdirAll(dirName string) error
	RemoveAll(path string) error
	Chdir(dir string) error
}

type FSOperations struct{}

var _ FSOperator = (*FSOperations)(nil)

func NewFSOperations() *FSOperations {
	return &FSOperations{}
}

func (f FSOperations) CreateFile(filePath string) (*os.File, error) {
	return os.Create(filePath)
}

func (f FSOperations) WriteFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0644)
}

func (f FSOperations) MkdirAll(dirName string) error {
	return os.MkdirAll(dirName, 0755)
}

func (f FSOperations) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (f FSOperations) Chdir(path string) error {
	return os.Chdir(path)
}
