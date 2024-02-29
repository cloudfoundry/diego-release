package models

import (
	bytes "bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"time"

	"code.cloudfoundry.org/bbs/format"
)

const PreloadedRootFSScheme = "preloaded"
const PreloadedOCIRootFSScheme = "preloaded+layer"

var processGuidPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type DesiredLRPChange struct {
	Before *DesiredLRP
	After  *DesiredLRP
}

type DesiredLRPFilter struct {
	Domain       string
	ProcessGuids []string
}

func PreloadedRootFS(stack string) string {
	return (&url.URL{
		Scheme: PreloadedRootFSScheme,
		Opaque: stack,
	}).String()
}

func NewDesiredLRP(schedInfo DesiredLRPSchedulingInfo, runInfo DesiredLRPRunInfo) DesiredLRP {
	environmentVariables := make([]*EnvironmentVariable, len(runInfo.EnvironmentVariables))
	for i := range runInfo.EnvironmentVariables {
		environmentVariables[i] = &runInfo.EnvironmentVariables[i]
	}

	egressRules := make([]*SecurityGroupRule, len(runInfo.EgressRules))
	for i := range runInfo.EgressRules {
		egressRules[i] = &runInfo.EgressRules[i]
	}

	return DesiredLRP{
		ProcessGuid:                   schedInfo.ProcessGuid,
		Domain:                        schedInfo.Domain,
		LogGuid:                       schedInfo.LogGuid,
		MemoryMb:                      schedInfo.MemoryMb,
		DiskMb:                        schedInfo.DiskMb,
		MaxPids:                       schedInfo.MaxPids,
		RootFs:                        schedInfo.RootFs,
		Instances:                     schedInfo.Instances,
		Annotation:                    schedInfo.Annotation,
		Routes:                        &schedInfo.Routes,
		ModificationTag:               &schedInfo.ModificationTag,
		EnvironmentVariables:          environmentVariables,
		CachedDependencies:            runInfo.CachedDependencies,
		Setup:                         runInfo.Setup,
		Action:                        runInfo.Action,
		Monitor:                       runInfo.Monitor,
		StartTimeoutMs:                runInfo.StartTimeoutMs,
		Privileged:                    runInfo.Privileged,
		CpuWeight:                     runInfo.CpuWeight,
		Ports:                         runInfo.Ports,
		EgressRules:                   egressRules,
		LogSource:                     runInfo.LogSource,
		MetricsGuid:                   runInfo.MetricsGuid,
		LegacyDownloadUser:            runInfo.LegacyDownloadUser,
		TrustedSystemCertificatesPath: runInfo.TrustedSystemCertificatesPath,
		VolumeMounts:                  runInfo.VolumeMounts,
		Network:                       runInfo.Network,
		PlacementTags:                 schedInfo.PlacementTags,
		CertificateProperties:         runInfo.CertificateProperties,
		ImageUsername:                 runInfo.ImageUsername,
		ImagePassword:                 runInfo.ImagePassword,
		CheckDefinition:               runInfo.CheckDefinition,
		ImageLayers:                   runInfo.ImageLayers,
		MetricTags:                    runInfo.MetricTags,
		Sidecars:                      runInfo.Sidecars,
		LogRateLimit:                  runInfo.LogRateLimit,
	}
}

func (desiredLRP *DesiredLRP) AddRunInfo(runInfo DesiredLRPRunInfo) {
	environmentVariables := make([]*EnvironmentVariable, len(runInfo.EnvironmentVariables))
	for i := range runInfo.EnvironmentVariables {
		environmentVariables[i] = &runInfo.EnvironmentVariables[i]
	}

	egressRules := make([]*SecurityGroupRule, len(runInfo.EgressRules))
	for i := range runInfo.EgressRules {
		egressRules[i] = &runInfo.EgressRules[i]
	}

	desiredLRP.EnvironmentVariables = environmentVariables
	desiredLRP.CachedDependencies = runInfo.CachedDependencies
	desiredLRP.Setup = runInfo.Setup
	desiredLRP.Action = runInfo.Action
	desiredLRP.Monitor = runInfo.Monitor
	desiredLRP.StartTimeoutMs = runInfo.StartTimeoutMs
	desiredLRP.Privileged = runInfo.Privileged
	desiredLRP.CpuWeight = runInfo.CpuWeight
	desiredLRP.Ports = runInfo.Ports
	desiredLRP.EgressRules = egressRules
	desiredLRP.LogSource = runInfo.LogSource
	desiredLRP.MetricsGuid = runInfo.MetricsGuid
	desiredLRP.LegacyDownloadUser = runInfo.LegacyDownloadUser
	desiredLRP.TrustedSystemCertificatesPath = runInfo.TrustedSystemCertificatesPath
	desiredLRP.VolumeMounts = runInfo.VolumeMounts
	desiredLRP.Network = runInfo.Network
	desiredLRP.CheckDefinition = runInfo.CheckDefinition
}

func (*DesiredLRP) Version() format.Version {
	return format.V3
}

func (d *DesiredLRP) actionsFromCachedDependencies() []ActionInterface {
	actions := make([]ActionInterface, len(d.CachedDependencies))
	for i := range d.CachedDependencies {
		cacheDependency := d.CachedDependencies[i]
		actions[i] = &DownloadAction{
			Artifact:  cacheDependency.Name,
			From:      cacheDependency.From,
			To:        cacheDependency.To,
			CacheKey:  cacheDependency.CacheKey,
			LogSource: cacheDependency.LogSource,
			User:      d.LegacyDownloadUser,
		}
	}
	return actions
}

func newDesiredLRPWithCachedDependenciesAsSetupActions(d *DesiredLRP) *DesiredLRP {
	d = d.Copy()
	if len(d.CachedDependencies) > 0 {

		cachedDownloads := Parallel(d.actionsFromCachedDependencies()...)

		if d.Setup != nil {
			d.Setup = WrapAction(Serial(cachedDownloads, UnwrapAction(d.Setup)))
		} else {
			d.Setup = WrapAction(Serial(cachedDownloads))
		}
		d.CachedDependencies = nil
	}

	return d
}

func downgradeDesiredLRPV2ToV1(d *DesiredLRP) *DesiredLRP {
	return d
}

func downgradeDesiredLRPV1ToV0(d *DesiredLRP) *DesiredLRP {
	d.Action = d.Action.SetDeprecatedTimeoutNs()
	d.Setup = d.Setup.SetDeprecatedTimeoutNs()
	d.Monitor = d.Monitor.SetDeprecatedTimeoutNs()
	d.DeprecatedStartTimeoutS = uint32(d.StartTimeoutMs) / 1000
	return newDesiredLRPWithCachedDependenciesAsSetupActions(d)
}

func downgradeDesiredLRPV3ToV2(d *DesiredLRP) *DesiredLRP {
	layers := ImageLayers(d.ImageLayers)

	d.CachedDependencies = append(layers.ToCachedDependencies(), d.CachedDependencies...)
	d.Setup = layers.ToDownloadActions(d.LegacyDownloadUser, d.Setup)
	d.ImageLayers = nil

	return d
}

var downgrades = []func(*DesiredLRP) *DesiredLRP{
	downgradeDesiredLRPV1ToV0,
	downgradeDesiredLRPV2ToV1,
	downgradeDesiredLRPV3ToV2,
}

func (d *DesiredLRP) VersionDownTo(v format.Version) *DesiredLRP {
	versionedLRP := d.Copy()

	for version := d.Version(); version > v; version-- {
		versionedLRP = downgrades[version-1](versionedLRP)
	}

	return versionedLRP
}

func (d *DesiredLRP) PopulateMetricsGuid() *DesiredLRP {
	sourceId, sourceIDIsSet := d.MetricTags["source_id"]
	switch {
	case sourceIDIsSet && d.MetricsGuid == "":
		d.MetricsGuid = sourceId.Static
	case !sourceIDIsSet && d.MetricsGuid != "":
		if d.MetricTags == nil {
			d.MetricTags = make(map[string]*MetricTagValue)
		}
		d.MetricTags["source_id"] = &MetricTagValue{
			Static: d.MetricsGuid,
		}
	}
	return d
}

func (d *DesiredLRP) DesiredLRPKey() DesiredLRPKey {
	return NewDesiredLRPKey(d.ProcessGuid, d.Domain, d.LogGuid)
}

func (d *DesiredLRP) DesiredLRPResource() DesiredLRPResource {
	return NewDesiredLRPResource(d.MemoryMb, d.DiskMb, d.MaxPids, d.RootFs)
}

func (d *DesiredLRP) DesiredLRPSchedulingInfo() DesiredLRPSchedulingInfo {
	var routes Routes
	if d.Routes != nil {
		routes = *d.Routes
	}
	var modificationTag ModificationTag
	if d.ModificationTag != nil {
		modificationTag = *d.ModificationTag
	}

	var volumePlacement VolumePlacement
	volumePlacement.DriverNames = []string{}
	for _, mount := range d.VolumeMounts {
		volumePlacement.DriverNames = append(volumePlacement.DriverNames, mount.Driver)
	}

	return NewDesiredLRPSchedulingInfo(
		d.DesiredLRPKey(),
		d.Annotation,
		d.Instances,
		d.DesiredLRPResource(),
		routes,
		modificationTag,
		&volumePlacement,
		d.PlacementTags,
	)
}

func (d *DesiredLRP) DesiredLRPRoutingInfo() DesiredLRP {
	var routes Routes
	if d.Routes != nil {
		routes = *d.Routes
	}

	var modificationTag ModificationTag
	if d.ModificationTag != nil {
		modificationTag = *d.ModificationTag
	}

	return NewDesiredLRPRoutingInfo(
		d.DesiredLRPKey(),
		d.Instances,
		&routes,
		&modificationTag,
		d.MetricTags,
	)
}

func (d *DesiredLRP) DesiredLRPRunInfo(createdAt time.Time) DesiredLRPRunInfo {
	environmentVariables := make([]EnvironmentVariable, len(d.EnvironmentVariables))
	for i := range d.EnvironmentVariables {
		environmentVariables[i] = *d.EnvironmentVariables[i]
	}

	egressRules := make([]SecurityGroupRule, len(d.EgressRules))
	for i := range d.EgressRules {
		egressRules[i] = *d.EgressRules[i]
	}

	return NewDesiredLRPRunInfo(
		d.DesiredLRPKey(),
		createdAt,
		environmentVariables,
		d.CachedDependencies,
		d.Setup,
		d.Action,
		d.Monitor,
		d.StartTimeoutMs,
		d.Privileged,
		d.CpuWeight,
		d.Ports,
		egressRules,
		d.LogSource,
		d.MetricsGuid,
		d.LegacyDownloadUser,
		d.TrustedSystemCertificatesPath,
		d.VolumeMounts,
		d.Network,
		d.CertificateProperties,
		d.ImageUsername,
		d.ImagePassword,
		d.CheckDefinition,
		d.ImageLayers,
		d.MetricTags,
		d.Sidecars,
		d.LogRateLimit,
	)
}

func (d *DesiredLRP) Copy() *DesiredLRP {
	newDesired := *d
	return &newDesired
}

func (desired DesiredLRP) Validate() error {
	var validationError ValidationError

	if desired.GetDomain() == "" {
		validationError = validationError.Append(ErrInvalidField{"domain"})
	}

	if desired.GetRootFs() == "" {
		validationError = validationError.Append(ErrInvalidField{"rootfs"})
	}

	rootFSURL, err := url.Parse(desired.GetRootFs())
	if err != nil || rootFSURL.Scheme == "" {
		validationError = validationError.Append(ErrInvalidField{"rootfs"})
	}

	if desired.GetInstances() < 0 {
		validationError = validationError.Append(ErrInvalidField{"instances"})
	}

	if desired.GetMemoryMb() < 0 {
		validationError = validationError.Append(ErrInvalidField{"memory_mb"})
	}

	if desired.GetDiskMb() < 0 {
		validationError = validationError.Append(ErrInvalidField{"disk_mb"})
	}

	if limit := desired.GetLogRateLimit(); limit != nil {
		if limit.GetBytesPerSecond() < -1 {
			validationError = validationError.Append(ErrInvalidField{"log_rate_limit_bytes_per_second"})
		}
	}

	if len(desired.GetAnnotation()) > maximumAnnotationLength {
		validationError = validationError.Append(ErrInvalidField{"annotation"})
	}

	if desired.GetMaxPids() < 0 {
		validationError = validationError.Append(ErrInvalidField{"max_pids"})
	}

	totalRoutesLength := 0
	if desired.Routes != nil {
		for _, value := range *desired.Routes {
			totalRoutesLength += len(*value)
			if totalRoutesLength > maximumRouteLength {
				validationError = validationError.Append(ErrInvalidField{"routes"})
				break
			}
		}
	}

	runInfoErrors := desired.DesiredLRPRunInfo(time.Now()).Validate()
	if runInfoErrors != nil {
		validationError = validationError.Append(runInfoErrors)
	}

	return validationError.ToError()
}

func (desired *DesiredLRPUpdate) Validate() error {
	var validationError ValidationError

	if desired.GetInstances() < 0 {
		validationError = validationError.Append(ErrInvalidField{"instances"})
	}

	if len(desired.GetAnnotation()) > maximumAnnotationLength {
		validationError = validationError.Append(ErrInvalidField{"annotation"})
	}

	totalRoutesLength := 0
	if desired.Routes != nil {
		for _, value := range *desired.Routes {
			totalRoutesLength += len(*value)
			if totalRoutesLength > maximumRouteLength {
				validationError = validationError.Append(ErrInvalidField{"routes"})
				break
			}
		}
	}

	err := validateMetricTags(desired.MetricTags, "")
	if err != nil {
		validationError = validationError.Append(ErrInvalidField{"metric_tags"})
		validationError = validationError.Append(err)
	}

	return validationError.ToError()
}

func (desired *DesiredLRPUpdate) SetInstances(instances int32) {
	desired.OptionalInstances = &DesiredLRPUpdate_Instances{
		Instances: instances,
	}
}

func (desired DesiredLRPUpdate) InstancesExists() bool {
	_, ok := desired.GetOptionalInstances().(*DesiredLRPUpdate_Instances)
	return ok
}

func (desired *DesiredLRPUpdate) SetAnnotation(annotation string) {
	desired.OptionalAnnotation = &DesiredLRPUpdate_Annotation{
		Annotation: annotation,
	}
}

func (desired DesiredLRPUpdate) AnnotationExists() bool {
	_, ok := desired.GetOptionalAnnotation().(*DesiredLRPUpdate_Annotation)
	return ok
}

func (desired DesiredLRPUpdate) IsRoutesGroupUpdated(routes *Routes, routerGroup string) bool {
	if desired.Routes == nil {
		return false
	}

	if routes == nil {
		return true
	}

	desiredRoutes, desiredRoutesPresent := (*desired.Routes)[routerGroup]
	requestRoutes, requestRoutesPresent := (*routes)[routerGroup]
	if desiredRoutesPresent != requestRoutesPresent {
		return true
	}

	if desiredRoutesPresent && requestRoutesPresent {
		return !bytes.Equal(*desiredRoutes, *requestRoutes)
	}

	return true
}

func (desired DesiredLRPUpdate) IsMetricTagsUpdated(existingTags map[string]*MetricTagValue) bool {
	if desired.MetricTags == nil {
		return false
	}
	if len(desired.MetricTags) != len(existingTags) {
		return true
	}
	for k, v := range existingTags {
		updateTag, ok := desired.MetricTags[k]
		if !ok {
			return true
		}
		if updateTag.Static != v.Static || updateTag.Dynamic != v.Dynamic {
			return true
		}
	}
	return false
}

type internalDesiredLRPUpdate struct {
	Instances  *int32                     `json:"instances,omitempty"`
	Routes     *Routes                    `json:"routes,omitempty"`
	Annotation *string                    `json:"annotation,omitempty"`
	MetricTags map[string]*MetricTagValue `json:"metric_tags,omitempty"`
}

func (desired *DesiredLRPUpdate) UnmarshalJSON(data []byte) error {
	var update internalDesiredLRPUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return err
	}

	if update.Instances != nil {
		desired.SetInstances(*update.Instances)
	}
	desired.Routes = update.Routes
	if update.Annotation != nil {
		desired.SetAnnotation(*update.Annotation)
	}
	desired.MetricTags = update.MetricTags

	return nil
}

func (desired DesiredLRPUpdate) MarshalJSON() ([]byte, error) {
	var update internalDesiredLRPUpdate
	if desired.InstancesExists() {
		i := desired.GetInstances()
		update.Instances = &i
	}
	update.Routes = desired.Routes
	if desired.AnnotationExists() {
		a := desired.GetAnnotation()
		update.Annotation = &a
	}
	update.MetricTags = desired.MetricTags
	return json.Marshal(update)
}

func NewDesiredLRPKey(processGuid, domain, logGuid string) DesiredLRPKey {
	return DesiredLRPKey{
		ProcessGuid: processGuid,
		Domain:      domain,
		LogGuid:     logGuid,
	}
}

func (key DesiredLRPKey) Validate() error {
	var validationError ValidationError
	if key.GetDomain() == "" {
		validationError = validationError.Append(ErrInvalidField{"domain"})
	}

	if !processGuidPattern.MatchString(key.GetProcessGuid()) {
		validationError = validationError.Append(ErrInvalidField{"process_guid"})
	}

	return validationError.ToError()
}

func NewDesiredLRPSchedulingInfo(
	key DesiredLRPKey,
	annotation string,
	instances int32,
	resource DesiredLRPResource,
	routes Routes,
	modTag ModificationTag,
	volumePlacement *VolumePlacement,
	placementTags []string,
) DesiredLRPSchedulingInfo {
	return DesiredLRPSchedulingInfo{
		DesiredLRPKey:      key,
		Annotation:         annotation,
		Instances:          instances,
		DesiredLRPResource: resource,
		Routes:             routes,
		ModificationTag:    modTag,
		VolumePlacement:    volumePlacement,
		PlacementTags:      placementTags,
	}
}

func NewDesiredLRPRoutingInfo(
	key DesiredLRPKey,
	instances int32,
	routes *Routes,
	modTag *ModificationTag,
	metrTags map[string]*MetricTagValue,
) DesiredLRP {
	return DesiredLRP{
		ProcessGuid:     key.ProcessGuid,
		Domain:          key.Domain,
		LogGuid:         key.LogGuid,
		Instances:       instances,
		Routes:          routes,
		ModificationTag: modTag,
		MetricTags:      metrTags,
	}
}

func (s *DesiredLRPSchedulingInfo) ApplyUpdate(update *DesiredLRPUpdate) {
	if update.InstancesExists() {
		s.Instances = update.GetInstances()
	}
	if update.Routes != nil {
		s.Routes = *update.Routes
	}
	if update.AnnotationExists() {
		s.Annotation = update.GetAnnotation()
	}
	s.ModificationTag.Increment()
}

func (*DesiredLRPSchedulingInfo) Version() format.Version {
	return format.V0
}

func (s DesiredLRPSchedulingInfo) Validate() error {
	var validationError ValidationError

	validationError = validationError.Check(s.DesiredLRPKey, s.DesiredLRPResource, s.Routes)

	if s.GetInstances() < 0 {
		validationError = validationError.Append(ErrInvalidField{"instances"})
	}

	if len(s.GetAnnotation()) > maximumAnnotationLength {
		validationError = validationError.Append(ErrInvalidField{"annotation"})
	}

	return validationError.ToError()
}

func NewDesiredLRPResource(memoryMb, diskMb, maxPids int32, rootFs string) DesiredLRPResource {
	return DesiredLRPResource{
		MemoryMb: memoryMb,
		DiskMb:   diskMb,
		MaxPids:  maxPids,
		RootFs:   rootFs,
	}
}

func (resource DesiredLRPResource) Validate() error {
	var validationError ValidationError

	rootFSURL, err := url.Parse(resource.GetRootFs())
	if err != nil || rootFSURL.Scheme == "" {
		validationError = validationError.Append(ErrInvalidField{"rootfs"})
	}

	if resource.GetMemoryMb() < 0 {
		validationError = validationError.Append(ErrInvalidField{"memory_mb"})
	}

	if resource.GetDiskMb() < 0 {
		validationError = validationError.Append(ErrInvalidField{"disk_mb"})
	}

	if resource.GetMaxPids() < 0 {
		validationError = validationError.Append(ErrInvalidField{"max_pids"})
	}

	return validationError.ToError()
}

func NewDesiredLRPRunInfo(
	key DesiredLRPKey,
	createdAt time.Time,
	envVars []EnvironmentVariable,
	cacheDeps []*CachedDependency,
	setup,
	action,
	monitor *Action,
	startTimeoutMs int64,
	privileged bool,
	cpuWeight uint32,
	ports []uint32,
	egressRules []SecurityGroupRule,
	logSource,
	metricsGuid string,
	legacyDownloadUser string,
	trustedSystemCertificatesPath string,
	volumeMounts []*VolumeMount,
	network *Network,
	certificateProperties *CertificateProperties,
	imageUsername, imagePassword string,
	checkDefinition *CheckDefinition,
	imageLayers []*ImageLayer,
	metricTags map[string]*MetricTagValue,
	sidecars []*Sidecar,
	logRateLimit *LogRateLimit,
) DesiredLRPRunInfo {
	return DesiredLRPRunInfo{
		DesiredLRPKey:                 key,
		CreatedAt:                     createdAt.UnixNano(),
		EnvironmentVariables:          envVars,
		CachedDependencies:            cacheDeps,
		Setup:                         setup,
		Action:                        action,
		Monitor:                       monitor,
		StartTimeoutMs:                startTimeoutMs,
		Privileged:                    privileged,
		CpuWeight:                     cpuWeight,
		Ports:                         ports,
		EgressRules:                   egressRules,
		LogSource:                     logSource,
		MetricsGuid:                   metricsGuid,
		LegacyDownloadUser:            legacyDownloadUser,
		TrustedSystemCertificatesPath: trustedSystemCertificatesPath,
		VolumeMounts:                  volumeMounts,
		Network:                       network,
		CertificateProperties:         certificateProperties,
		ImageUsername:                 imageUsername,
		ImagePassword:                 imagePassword,
		CheckDefinition:               checkDefinition,
		ImageLayers:                   imageLayers,
		MetricTags:                    metricTags,
		Sidecars:                      sidecars,
		LogRateLimit:                  logRateLimit,
	}
}

func (runInfo DesiredLRPRunInfo) Validate() error {
	var validationError ValidationError

	validationError = validationError.Check(runInfo.DesiredLRPKey)

	if runInfo.Setup != nil {
		if err := runInfo.Setup.Validate(); err != nil {
			validationError = validationError.Append(ErrInvalidField{"setup"})
			validationError = validationError.Append(err)
		}
	}

	if runInfo.Action == nil {
		validationError = validationError.Append(ErrInvalidActionType)
	} else if err := runInfo.Action.Validate(); err != nil {
		validationError = validationError.Append(ErrInvalidField{"action"})
		validationError = validationError.Append(err)
	}

	if runInfo.Monitor != nil {
		if err := runInfo.Monitor.Validate(); err != nil {
			validationError = validationError.Append(ErrInvalidField{"monitor"})
			validationError = validationError.Append(err)
		}
	}

	for _, envVar := range runInfo.EnvironmentVariables {
		validationError = validationError.Check(envVar)
	}

	for _, rule := range runInfo.EgressRules {
		err := rule.Validate()
		if err != nil {
			validationError = validationError.Append(ErrInvalidField{"egress_rules"})
			validationError = validationError.Append(err)
		}
	}

	err := validateCachedDependencies(runInfo.CachedDependencies)
	if err != nil {
		validationError = validationError.Append(err)
	}

	err = validateImageLayers(runInfo.ImageLayers, runInfo.LegacyDownloadUser)
	if err != nil {
		validationError = validationError.Append(err)
	}

	if runInfo.MetricTags == nil {
		validationError = validationError.Append(ErrInvalidField{"metric_tags"})
	}

	err = validateMetricTags(runInfo.MetricTags, runInfo.GetMetricsGuid())
	if err != nil {
		validationError = validationError.Append(ErrInvalidField{"metric_tags"})
		validationError = validationError.Append(err)
	}

	err = validateSidecars(runInfo.Sidecars)
	if err != nil {
		validationError = validationError.Append(ErrInvalidField{"sidecars"})
		validationError = validationError.Append(err)
	}

	for _, mount := range runInfo.VolumeMounts {
		validationError = validationError.Check(mount)
	}

	if runInfo.ImageUsername == "" && runInfo.ImagePassword != "" {
		validationError = validationError.Append(ErrInvalidField{"image_username"})
	}

	if runInfo.ImageUsername != "" && runInfo.ImagePassword == "" {
		validationError = validationError.Append(ErrInvalidField{"image_password"})
	}

	if runInfo.CheckDefinition != nil {
		if err := runInfo.CheckDefinition.Validate(); err != nil {
			validationError = validationError.Append(ErrInvalidField{"check_definition"})
			validationError = validationError.Append(err)
		}
	}

	if limit := runInfo.LogRateLimit; limit != nil {
		if limit.BytesPerSecond < -1 {
			validationError = validationError.Append(ErrInvalidField{"log_rate_limit"})
		}
	}

	return validationError.ToError()
}

func (*CertificateProperties) Version() format.Version {
	return format.V0
}

func (CertificateProperties) Validate() error {
	return nil
}
