package containerstore

import (
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

func NewInstanceIdentityHandler(
	credDir string,
	containerMountPath string,
) *InstanceIdentityHandler {
	return &InstanceIdentityHandler{
		credDir:            credDir,
		containerMountPath: containerMountPath,
	}
}

type InstanceIdentityHandler struct {
	containerMountPath string
	credDir            string
}

func (h *InstanceIdentityHandler) CreateDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	containerDir := filepath.Join(h.credDir, container.Guid)
	err := os.Mkdir(containerDir, 0755)
	if err != nil {
		return nil, nil, err
	}

	return []garden.BindMount{
			{
				SrcPath: containerDir,
				DstPath: h.containerMountPath,
				Mode:    garden.BindMountModeRO,
				Origin:  garden.BindMountOriginHost,
			},
		}, []executor.EnvironmentVariable{
			{Name: "CF_INSTANCE_CERT", Value: path.Join(h.containerMountPath, "instance.crt")},
			{Name: "CF_INSTANCE_KEY", Value: path.Join(h.containerMountPath, "instance.key")},
		}, nil
}

func (h *InstanceIdentityHandler) RemoveDir(logger lager.Logger, container executor.Container) error {
	return os.RemoveAll(filepath.Join(h.credDir, container.Guid))
}

func (h *InstanceIdentityHandler) Update(creds Credentials, container executor.Container) error {
	if creds.InstanceIdentityCredential.IsEmpty() {
		return nil
	}

	instanceKeyPath := filepath.Join(h.credDir, container.Guid, "instance.key")
	tmpInstanceKeyPath := instanceKeyPath + ".tmp"
	certificatePath := filepath.Join(h.credDir, container.Guid, "instance.crt")
	tmpCertificatePath := certificatePath + ".tmp"

	instanceKey, err := os.Create(tmpInstanceKeyPath)
	if err != nil {
		return err
	}
	defer instanceKey.Close()

	certificate, err := os.Create(tmpCertificatePath)
	if err != nil {
		return err
	}
	defer certificate.Close()

	_, err = certificate.Write([]byte(creds.InstanceIdentityCredential.Cert))
	if err != nil {
		return err
	}

	_, err = instanceKey.Write([]byte(creds.InstanceIdentityCredential.Key))
	if err != nil {
		return err
	}

	err = instanceKey.Close()
	if err != nil {
		return err
	}

	err = certificate.Close()
	if err != nil {
		return err
	}

	err = os.Rename(tmpInstanceKeyPath, instanceKeyPath)
	if err != nil {
		return err
	}

	err = os.Rename(tmpCertificatePath, certificatePath)
	if err != nil {
		return err
	}

	return nil
}

func (h *InstanceIdentityHandler) Close(creds Credentials, container executor.Container) error {
	return nil
}
