package rep

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/ecrhelper"
	"code.cloudfoundry.org/executor"
)

const (
	LifecycleTag  = "lifecycle"
	ResultFileTag = "result-file"
	DomainTag     = "domain"

	TaskLifecycle = "task"
	LRPLifecycle  = "lrp"

	ProcessGuidTag  = "process-guid"
	InstanceGuidTag = "instance-guid"
	ProcessIndexTag = "process-index"

	VolumeDriversTag = "volume-drivers"
	PlacementTagsTag = "placement-tags"
)

var (
	ErrContainerMissingTags = errors.New("container is missing tags")
	ErrInvalidProcessIndex  = errors.New("container does not have a valid process index")
	ErrIntergerOverflow     = errors.New("integer overflow")
)

func ActualLRPKeyFromTags(tags executor.Tags) (*models.ActualLRPKey, error) {
	if tags == nil {
		return &models.ActualLRPKey{}, ErrContainerMissingTags
	}

	processIndex, err := strconv.Atoi(tags[ProcessIndexTag])
	if err != nil {
		return &models.ActualLRPKey{}, ErrInvalidProcessIndex
	}

	if processIndex > math.MaxInt32 {
		return &models.ActualLRPKey{}, ErrIntergerOverflow
	}

	actualLRPKey := models.NewActualLRPKey(
		tags[ProcessGuidTag],
		int32(processIndex), //#nosec G109 -- Checked for integer overflow above
		tags[DomainTag],
	)

	err = actualLRPKey.Validate()
	if err != nil {
		return &models.ActualLRPKey{}, err
	}

	return &actualLRPKey, nil
}

func ActualLRPInstanceKeyFromContainer(container executor.Container, cellID string) (*models.ActualLRPInstanceKey, error) {
	if container.Tags == nil {
		return &models.ActualLRPInstanceKey{}, ErrContainerMissingTags
	}

	actualLRPInstanceKey := models.NewActualLRPInstanceKey(
		container.Tags[InstanceGuidTag],
		cellID,
	)

	err := actualLRPInstanceKey.Validate()
	if err != nil {
		return &models.ActualLRPInstanceKey{}, err
	}

	return &actualLRPInstanceKey, nil
}

func ActualLRPNetInfoFromContainer(container executor.Container) (*models.ActualLRPNetInfo, error) {
	ports := []*models.PortMapping{}

	for _, port := range container.Ports {
		ports = append(ports, models.NewPortMappingWithTLSProxy(
			uint32(port.HostPort),
			uint32(port.ContainerPort),
			uint32(port.HostTLSProxyPort),
			uint32(port.ContainerTLSProxyPort),
		))
	}

	preferredAddress := models.ActualLRPNetInfo_PreferredAddressHost
	if container.AdvertisePreferenceForInstanceAddress {
		preferredAddress = models.ActualLRPNetInfo_PreferredAddressInstance
	}

	actualLRPNetInfo := models.NewActualLRPNetInfoWithIPv6(container.ExternalIP, container.InternalIP, container.InternalIPv6, preferredAddress, ports...)

	err := actualLRPNetInfo.Validate()
	if err != nil {
		return nil, err
	}

	return &actualLRPNetInfo, nil
}

func LRPContainerGuid(processGuid, instanceGuid string) string {
	return instanceGuid
}

const (
	LayeringModeSingleLayer = "single-layer"
	LayeringModeTwoLayer    = "two-layer"
)

// ConvertPreloadedRootFS takes in a rootFS URL and a list of image layers and in most cases
// just returns the same rootFS URL and list of image layers.
//
// In the case where all of the following are true:
//   - layeringMode == LayeringModeTwoLayer
//   - the rootfs URL has a `preloaded` scheme
//   - the list of image layers contains at least one image layer that has
//     an `exclusive` layer type, `tgz` media type, and a `sha256` digest algorithm.
//
// then the rootfs URL will be converted to have a `preloaded+layer` scheme and
// a query string that references the first image layer that matches all of those
// restrictions. This image layer will also be removed from the list.
func ConvertPreloadedRootFS(rootFS string, imageLayers []*models.ImageLayer, layeringMode string) (string, []*models.ImageLayer) {
	if layeringMode != LayeringModeTwoLayer {
		return rootFS, imageLayers
	}
	if !strings.HasPrefix(rootFS, "preloaded:") {
		return rootFS, imageLayers
	}

	newImageLayers := []*models.ImageLayer{}
	var newRootFS string
	for _, v := range imageLayers {
		isExclusiveLayer := v.GetLayerType() == models.LayerTypeExclusive
		isMediaTypeTgz := v.GetMediaType() == models.MediaTypeTgz
		isSha256 := v.GetDigestAlgorithm() == models.DigestAlgorithmSha256
		suitableLayer := isExclusiveLayer && isMediaTypeTgz && isSha256
		if suitableLayer && newRootFS == "" {
			rootFSArray := strings.Split(rootFS, ":")
			newRootFS = fmt.Sprintf("preloaded+layer:%s?layer=%s&layer_path=%s&layer_digest=%s",
				rootFSArray[1],
				url.QueryEscape(v.GetUrl()),
				url.QueryEscape(v.DestinationPath),
				url.QueryEscape(v.DigestValue),
			)
			continue
		}
		newImageLayers = append(newImageLayers, v)
	}
	if newRootFS == "" {
		return rootFS, imageLayers
	}
	return newRootFS, newImageLayers
}

type RunRequestConversionHelper struct {
	ECRHelper ecrhelper.ECRHelper
}

func (rrch RunRequestConversionHelper) NewRunRequestFromDesiredLRP(
	containerGuid string,
	desiredLRP *models.DesiredLRP,
	lrpKey *models.ActualLRPKey,
	lrpInstanceKey *models.ActualLRPInstanceKey,
	stackPathMap StackPathMap,
	layeringMode string,
) (executor.RunRequest, error) {
	desiredLRPCopy := *desiredLRP
	desiredLRP = &desiredLRPCopy
	desiredLRP.RootFs, desiredLRP.ImageLayers = ConvertPreloadedRootFS(desiredLRP.RootFs, desiredLRP.ImageLayers, layeringMode)
	desiredLRP = desiredLRP.VersionDownTo(format.V2)

	mounts, err := convertVolumeMounts(desiredLRP.VolumeMounts, lrpKey)
	if err != nil {
		return executor.RunRequest{}, err
	}

	rootFSPath, err := stackPathMap.PathForRootFS(desiredLRP.RootFs)
	if err != nil {
		return executor.RunRequest{}, err
	}

	metricTags, err := models.ConvertMetricTags(desiredLRP.MetricTags, map[models.MetricTagValue_DynamicValue]any{
		models.MetricTagDynamicValueIndex:        lrpKey.Index,
		models.MetricTagDynamicValueInstanceGuid: lrpInstanceKey.InstanceGuid,
	})
	if err != nil {
		return executor.RunRequest{}, err
	}

	username, password, err := rrch.convertCredentials(rootFSPath, desiredLRP.ImageUsername, desiredLRP.ImagePassword)
	if err != nil {
		return executor.RunRequest{}, err
	}

	internalRoutes, err := models.InternalRoutesFromRoutingInfo(*desiredLRP.Routes)
	if err != nil {
		return executor.RunRequest{}, err
	}

	runInfo := executor.RunInfo{
		RootFSPath:     rootFSPath,
		CPUWeight:      uint(math.Max(float64(desiredLRP.CpuWeight), 1.0)),
		Ports:          ConvertPortMappings(desiredLRP.Ports),
		InternalRoutes: internalRoutes,
		LogConfig: models.LogConfig{
			Guid:       desiredLRP.LogGuid,
			Index:      int(lrpKey.Index),
			SourceName: desiredLRP.LogSource,
			Tags:       metricTags,
		},

		MetricsConfig: executor.MetricsConfig{
			Guid:  desiredLRP.MetricsGuid,
			Index: int(lrpKey.Index),
			Tags:  metricTags,
		},
		StartTimeoutMs:     uint(desiredLRP.StartTimeoutMs),
		Privileged:         desiredLRP.Privileged,
		CachedDependencies: ConvertCachedDependencies(desiredLRP.CachedDependencies),
		Setup:              desiredLRP.Setup,
		Action:             desiredLRP.Action,
		Monitor:            desiredLRP.Monitor,
		CheckDefinition:    desiredLRP.CheckDefinition,
		EgressRules:        desiredLRP.EgressRules,
		Env: append([]executor.EnvironmentVariable{
			{Name: "INSTANCE_GUID", Value: lrpInstanceKey.InstanceGuid},
			{Name: "INSTANCE_INDEX", Value: strconv.Itoa(int(lrpKey.Index))},
			{Name: "CF_INSTANCE_GUID", Value: lrpInstanceKey.InstanceGuid},
			{Name: "CF_INSTANCE_INDEX", Value: strconv.Itoa(int(lrpKey.Index))},
		}, executor.EnvironmentVariablesFromModel(desiredLRP.EnvironmentVariables)...),
		TrustedSystemCertificatesPath: desiredLRP.TrustedSystemCertificatesPath,
		VolumeMounts:                  mounts,
		Network:                       convertNetwork(desiredLRP.Network),
		CertificateProperties:         convertCertificateProperties(desiredLRP.CertificateProperties),
		ImageUsername:                 username,
		ImagePassword:                 password,
		EnableContainerProxy:          true,
		Sidecars:                      convertSidecars(desiredLRP.Sidecars),
		LogRateLimitBytesPerSecond:    convertLogRateLimit(desiredLRP.LogRateLimit),
		VolumeMountedFiles:            executor.VolumeMountedFilesFromModel(desiredLRP.VolumeMountedFiles),
	}

	// No need for the envoy proxy if there are no ports.  This flag controls the
	// step transformation (either prevent or include a run_step to run envoy) as
	// well as the proxy config handler to avoid generate the config
	// unecessarily.
	if len(runInfo.Ports) == 0 {
		runInfo.EnableContainerProxy = false
	}

	tags := executor.Tags{}
	return executor.NewRunRequest(containerGuid, &runInfo, tags), nil
}

func (rrch RunRequestConversionHelper) NewRunRequestFromTask(task *models.Task, stackPathMap StackPathMap, layeringMode string) (executor.RunRequest, error) {
	taskDefinitionCopy := *task.TaskDefinition
	taskCopy := *task
	task = &taskCopy
	task.TaskDefinition = &taskDefinitionCopy
	task.RootFs, task.ImageLayers = ConvertPreloadedRootFS(task.RootFs, task.ImageLayers, layeringMode)
	cachedDependencies, setupAction := convertImageLayers(task.TaskDefinition)

	mounts, err := convertVolumeMounts(task.VolumeMounts, nil)
	if err != nil {
		return executor.RunRequest{}, err
	}

	rootFSPath, err := stackPathMap.PathForRootFS(task.RootFs)
	if err != nil {
		return executor.RunRequest{}, err
	}

	metricTags, err := models.ConvertMetricTags(task.MetricTags, map[models.MetricTagValue_DynamicValue]any{})
	if err != nil {
		return executor.RunRequest{}, err
	}

	username, password, err := rrch.convertCredentials(rootFSPath, task.ImageUsername, task.ImagePassword)
	if err != nil {
		return executor.RunRequest{}, err
	}

	tags := executor.Tags{
		ResultFileTag: task.ResultFile,
	}
	runInfo := executor.RunInfo{
		RootFSPath: rootFSPath,
		CPUWeight:  uint(task.CpuWeight),
		Privileged: task.Privileged,
		LogConfig: models.LogConfig{
			Guid:       task.LogGuid,
			SourceName: task.LogSource,
			Tags:       metricTags,
		},
		MetricsConfig: executor.MetricsConfig{
			Guid: task.MetricsGuid,
			Tags: metricTags,
		},
		CachedDependencies:            ConvertCachedDependencies(cachedDependencies),
		Action:                        task.Action,
		Setup:                         setupAction,
		Env:                           executor.EnvironmentVariablesFromModel(task.EnvironmentVariables),
		EgressRules:                   task.EgressRules,
		TrustedSystemCertificatesPath: task.TrustedSystemCertificatesPath,
		VolumeMounts:                  mounts,
		Network:                       convertNetwork(task.Network),
		CertificateProperties:         convertCertificateProperties(task.CertificateProperties),
		ImageUsername:                 username,
		ImagePassword:                 password,
		EnableContainerProxy:          false,
		LogRateLimitBytesPerSecond:    convertLogRateLimit(task.LogRateLimit),
		VolumeMountedFiles:            executor.VolumeMountedFilesFromModel(task.VolumeMountedFiles),
	}
	return executor.NewRunRequest(task.TaskGuid, &runInfo, tags), nil
}

func ConvertCachedDependencies(modelDeps []*models.CachedDependency) []executor.CachedDependency {
	execDeps := make([]executor.CachedDependency, len(modelDeps))
	for i := range modelDeps {
		execDeps[i] = ConvertCachedDependency(modelDeps[i])
	}
	return execDeps
}

func ConvertCachedDependency(modelDep *models.CachedDependency) executor.CachedDependency {
	return executor.CachedDependency{
		Name:              modelDep.Name,
		From:              modelDep.From,
		To:                modelDep.To,
		CacheKey:          modelDep.CacheKey,
		LogSource:         modelDep.LogSource,
		ChecksumValue:     modelDep.ChecksumValue,
		ChecksumAlgorithm: modelDep.ChecksumAlgorithm,
	}
}

func convertImageLayers(t *models.TaskDefinition) ([]*models.CachedDependency, *models.Action) {
	layers := models.ImageLayers(t.ImageLayers)

	cachedDependencies := append(layers.ToCachedDependencies(), t.CachedDependencies...)
	action := layers.ToDownloadActions(t.LegacyDownloadUser, nil)

	return cachedDependencies, action
}

func (rrch RunRequestConversionHelper) convertCredentials(rootFS string, username string, password string) (string, string, error) {
	isECRRepo, err := rrch.ECRHelper.IsECRRepo(rootFS)
	if err != nil {
		return "", "", err
	}

	if !isECRRepo {
		return username, password, nil
	}

	username, password, err = rrch.ECRHelper.GetECRCredentials(rootFS, username, password)
	if err != nil {
		return "", "", fmt.Errorf("failed to get ECR credentials: %s", err.Error())
	}

	return username, password, nil
}

func convertVolumeMounts(volumeMounts []*models.VolumeMount, lrpKey *models.ActualLRPKey) ([]executor.VolumeMount, error) {
	execMnts := make([]executor.VolumeMount, len(volumeMounts))
	for i := range volumeMounts {
		var err error
		execMnts[i], err = convertVolumeMount(volumeMounts[i], lrpKey)
		if err != nil {
			return nil, err
		}
	}
	return execMnts, nil
}

func convertVolumeMount(volumeMnt *models.VolumeMount, lrpKey *models.ActualLRPKey) (executor.VolumeMount, error) {
	var config map[string]interface{}

	var volumeId string
	if volumeMnt.Shared != nil {
		volumeId = volumeMnt.Shared.VolumeId

		if len(volumeMnt.Shared.MountConfig) > 0 {
			err := json.Unmarshal([]byte(volumeMnt.Shared.MountConfig), &config)
			if err != nil {
				return executor.VolumeMount{}, err
			}
		}
	} else if volumeMnt.Dedicated != nil {
		if lrpKey != nil {
			volumeId = fmt.Sprintf("%s-%d", volumeMnt.Dedicated.MounterId, lrpKey.Index)
		} else {
			volumeId = volumeMnt.Dedicated.MounterId
		}

		var mountConfig map[string]interface{}
		if len(volumeMnt.Dedicated.MountConfig) > 0 {
			err := json.Unmarshal([]byte(volumeMnt.Dedicated.MountConfig), &mountConfig)
			if err != nil {
				return executor.VolumeMount{}, err
			}
		}

		var deviceConfig map[string]interface{}
		if len(volumeMnt.Dedicated.DeviceConfig) > 0 {
			err := json.Unmarshal([]byte(volumeMnt.Dedicated.DeviceConfig), &deviceConfig)
			if err != nil {
				return executor.VolumeMount{}, err
			}
		}

		config = map[string]interface{}{"mount_config": mountConfig, "device_config": deviceConfig, "source": volumeId}
	}

	var mode executor.BindMountMode
	switch volumeMnt.Mode {
	case "r":
		mode = executor.BindMountModeRO
	case "rw":
		mode = executor.BindMountModeRW
	default:
		return executor.VolumeMount{}, errors.New("unrecognized volume mount mode")
	}

	return executor.VolumeMount{
		Driver:        volumeMnt.Driver,
		VolumeId:      volumeId,
		ContainerPath: volumeMnt.ContainerDir,
		Mode:          mode,
		Config:        config,
	}, nil
}

func convertNetwork(network *models.Network) *executor.Network {
	if network == nil {
		return nil
	}

	return &executor.Network{
		Properties: network.Properties,
	}
}

func convertCertificateProperties(props *models.CertificateProperties) executor.CertificateProperties {
	if props == nil {
		return executor.CertificateProperties{}
	}

	return executor.CertificateProperties{
		OrganizationalUnit: props.OrganizationalUnit,
	}
}

func convertSidecars(sidecars []*models.Sidecar) []executor.Sidecar {
	es := []executor.Sidecar{}
	for _, sidecar := range sidecars {
		es = append(es, executor.Sidecar{
			Action:   sidecar.Action,
			MemoryMB: sidecar.MemoryMb,
			DiskMB:   sidecar.DiskMb,
		})
	}

	return es
}

func convertLogRateLimit(logRateLimit *models.LogRateLimit) int64 {
	if logRateLimit == nil {
		return -1
	}
	return logRateLimit.BytesPerSecond
}

func ConvertPortMappings(containerPorts []uint32) []executor.PortMapping {
	out := []executor.PortMapping{}
	for _, port := range containerPorts {
		out = append(out, executor.PortMapping{
			ContainerPort: uint16(port),
		})
	}

	return out
}
