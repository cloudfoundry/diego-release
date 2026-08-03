package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"code.cloudfoundry.org/cnbapplifecycle/pkg/archive"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/buildpacks"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/credhub"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/databaseuri"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/errors"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/keychain"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/staging"
	"github.com/spf13/cobra"

	"github.com/buildpacks/pack/pkg/blob"
	"github.com/buildpacks/pack/pkg/image"

	"github.com/buildpacks/lifecycle/api"
	"github.com/buildpacks/lifecycle/buildpack"
	"github.com/buildpacks/lifecycle/cache"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/launch"
	"github.com/buildpacks/lifecycle/layers"
	"github.com/buildpacks/lifecycle/phase"
	"github.com/buildpacks/lifecycle/platform"
	"github.com/buildpacks/lifecycle/platform/files"
)

const (
	PlatformAPI          = "0.14"
	DefaultLayersPath    = "/home/vcap/layers"
	DefaultWorkspacePath = "/home/vcap/workspace"
)

var (
	layersDir                 string
	workspaceDir              string
	cacheDir                  string
	cacheOutputFile           string
	result                    string
	dropletFile               string
	buildpackList             []string
	envVarNames               []string
	autoDetect                bool
	platformDir               string
	buildpacksDir             string
	systemBuildpacksDir       string
	extensionsDir             string
	downloadCacheDir          string
	credhubConnectionAttempts int
	credhubRetryDelay         time.Duration
)

func Execute() error {
	return builderCmd.Execute()
}

func init() {
	builderCmd.Flags().StringSliceVarP(&buildpackList, "buildpack", "b", nil, "buildpack(s) to use")
	builderCmd.Flags().StringVarP(&systemBuildpacksDir, "system-buildpacks-dir", "", "/tmp/buildpacks", "system buildpacks dir")
	builderCmd.Flags().StringVarP(&dropletFile, "droplet", "d", "/tmp/droplet", "output droplet file")
	builderCmd.Flags().StringVarP(&result, "result", "r", "/tmp/result.json", "result file")
	builderCmd.Flags().StringVarP(&workspaceDir, "workspace-dir", "w", DefaultWorkspacePath, "app workspace dir")
	builderCmd.Flags().StringVarP(&layersDir, "layers", "l", DefaultLayersPath, "layers dir")
	builderCmd.Flags().StringSliceVarP(&envVarNames, "pass-env-var", "", nil, "environment variable(s) to pass to buildpacks")
	builderCmd.Flags().StringVarP(&cacheDir, "cache-dir", "c", "/tmp/cache", "cache dir")
	builderCmd.Flags().StringVarP(&cacheOutputFile, "cache-output", "", "/tmp/cache-output.tgz", "cache output")
	builderCmd.Flags().BoolVar(&autoDetect, "auto-detect", false, "run auto-detection with the provided buildpacks")
	builderCmd.Flags().IntVar(&credhubConnectionAttempts, "credhub-connection-attempts", 3, "number of times that the credhub client will attempt to connect to credhub")
	builderCmd.Flags().DurationVar(&credhubRetryDelay, "credhub-retry-delay", 1*time.Second, "delay duration that credhub client will wait before retries (ex. 1s, 2m, etc.)")
	_ = builderCmd.MarkFlagRequired("buildpack")
}

var builderCmd = &cobra.Command{
	Use:          "builder",
	SilenceUsage: true,
	RunE: func(cobraCmd *cobra.Command, cmdArgs []string) error {
		platformAPI := api.MustParse(PlatformAPI)
		inputs := platform.NewLifecycleInputs(platformAPI)

		cmd.DisableColor(inputs.NoColor)
		logger := log.NewLogger()
		if err := logger.SetLevel(inputs.LogLevel); err != nil {
			logger.Errorf("failed to set log level to %q, error: %s\n", inputs.LogLevel, err.Error())
			return errors.ErrGenericBuild
		}

		if err := credhub.InterpolateServiceRefs(credhubConnectionAttempts, credhubRetryDelay); err != nil {
			logger.Error(err.Error())
			return errors.ErrGenericBuild
		}

		databaseUrl, err := databaseuri.ParseDatabaseURI(os.Getenv("VCAP_SERVICES"))
		if err != nil {
			logger.Errorf("failed to parse database URI, error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}
		if databaseUrl != "" {
			err = os.Setenv("DATABASE_URL", databaseUrl)
			if err != nil {
				logger.Errorf("Unable to set DATABASE_URL envirionment variable: %v", err)
				return errors.ErrGenericBuild
			}
		}

		tempDirs := map[string]*string{
			"platform":       &platformDir,
			"buildpacks":     &buildpacksDir,
			"extensions":     &extensionsDir,
			"download-cache": &downloadCacheDir,
		}

		for name, dir := range tempDirs {
			if *dir, err = os.MkdirTemp("", name); err != nil {
				logger.Errorf("failed to create folder %q, error: %s\n", name, err.Error())
				return errors.ErrGenericBuild
			}
		}

		for _, dir := range []string{layersDir, cacheDir} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				logger.Errorf("failed to create %q, error: %s\n", dir, err.Error())
				return errors.ErrGenericBuild
			}
		}

		if err := staging.CreateEnvFiles(platformDir, envVarNames); err != nil {
			logger.Errorf("failed to write env var files, error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		orderFile, err := os.CreateTemp("", "order.toml")
		if err != nil {
			logger.Errorf("failed to create 'order.toml', error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		analyzePath := filepath.Join(layersDir, "analyzed.toml")
		analyzeMD, err := writeAnalyzed(analyzePath, logger)
		if err != nil {
			logger.Errorf("failed to create 'analyzed.toml', error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		creds, err := keychain.FromEnv()
		if err != nil {
			logger.Errorf("failed to parse %s environment variable, error: %s\n", keychain.CnbCredentialsEnv, err.Error())
			return errors.ErrGenericBuild
		}

		buildpackList, err = buildpacks.Translate(buildpackList, systemBuildpacksDir, logger)
		if err != nil {
			logger.Errorf("failed to translate buildpack locations %#v, error: %s\n", buildpackList, err.Error())
			return errors.ErrDownloadingBuildpack
		}

		err = buildpacks.DownloadBuildpacks(
			buildpackList,
			buildpacksDir,
			image.NewFetcher(logger, nil, image.WithKeychain(creds)),
			blob.NewDownloader(logger, downloadCacheDir, blob.WithClient(keychain.NewHTTPClient(creds))),
			orderFile,
			autoDetect,
			logger,
		)
		if err != nil {
			logger.Errorf("failed to download buildpacks, error: %s\n", err.Error())
			return errors.ErrDownloadingBuildpack
		}

		dirStore := platform.NewDirStore(buildpacksDir, extensionsDir)
		detectorFactory := phase.NewHermeticFactory(
			platformAPI,
			&cmd.BuildpackAPIVerifier{},
			files.Handler,
			dirStore,
		)

		detector, err := detectorFactory.NewDetector(platform.LifecycleInputs{
			AnalyzedPath:  analyzePath,
			PlatformAPI:   platformAPI,
			AppDir:        workspaceDir,
			BuildpacksDir: buildpacksDir,
			LayersDir:     layersDir,
			OrderPath:     orderFile.Name(),
			PlatformDir:   platformDir,
			CacheDir:      cacheDir,
			UseDaemon:     false,
		}, logger)
		if err != nil {
			logger.Errorf("failed creating detector, error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		logger.Phase("DETECTING")
		bGroup, plan, err := detector.Detect()
		if err != nil {
			logger.Errorf("failed 'detect' phase, error: %s\n", err.Error())
			return errors.ErrDetecting
		}

		logger.Phase("RESTORING")
		cache, err := cache.NewVolumeCache(cacheDir, logger)
		if err != nil {
			logger.Errorf("failed to initialise cache, error: %s\n", err.Error())
			return errors.ErrRestoring
		}

		restorer := phase.Restorer{
			LayersDir:   layersDir,
			Logger:      logger,
			Buildpacks:  bGroup.Group,
			PlatformAPI: platformAPI,
		}
		if err := restorer.Restore(cache); err != nil {
			logger.Errorf("failed to restore cached layers, error: %s\n", err.Error())
			return errors.ErrRestoring
		}

		bldr := phase.Builder{
			AppDir:        workspaceDir,
			LayersDir:     layersDir,
			PlatformDir:   platformDir,
			BuildExecutor: &buildpack.DefaultBuildExecutor{},
			DirStore:      dirStore,
			Group:         bGroup,
			Logger:        logger,
			Out:           os.Stdout,
			Err:           os.Stderr,
			Plan:          plan,
			PlatformAPI:   platformAPI,
			AnalyzeMD:     analyzeMD,
		}

		logger.Phase("BUILDING")
		buildMeta, err := bldr.Build()
		if err != nil {
			logger.Errorf("failed 'build' phase, error: %s\n", err.Error())
			return errors.ErrBuilding
		}
		ensureWebProcessType(buildMeta)

		if err := files.Handler.WriteBuildMetadata(launch.GetMetadataFilePath(layersDir), buildMeta); err != nil {
			logger.Errorf("failed writing build metadata, error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		artifactsDir, err := os.MkdirTemp("", "lifecycle.exporter.layer")
		if err != nil {
			logger.Errorf("create temp directory for artifacts, error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		exporter := phase.Exporter{
			Buildpacks:  bGroup.Group,
			Logger:      logger,
			PlatformAPI: platformAPI,
			LayerFactory: &layers.Factory{
				ArtifactsDir: artifactsDir,
				UID:          inputs.UID,
				GID:          inputs.GID,
				Logger:       logger,
				Ctx:          context.Background(),
			},
		}

		logger.Phase("EXPORTING")
		if err := exporter.Cache(layersDir, cache); err != nil {
			logger.Errorf("failed to save cached layers, error: %s\n", err.Error())
			return errors.ErrExporting
		}

		cacheOutFile, err := os.OpenFile(cacheOutputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			logger.Errorf("Failed to open %q, error: %s\n", cacheOutputFile, err.Error())
			return errors.ErrGenericBuild
		}
		defer cacheOutFile.Close()

		cgw := gzip.NewWriter(cacheOutFile)
		defer cgw.Close()

		if err := archive.FromDirectory(cacheDir, tar.NewWriter(cgw)); err != nil {
			logger.Errorf("failed to save archive cache folder, error: %s\n", err.Error())
			return errors.ErrExporting
		}

		resultData := staging.StagingResultFromMetadata(buildMeta)
		resultBytes, err := json.Marshal(resultData)
		if err != nil {
			logger.Errorf("failed to marshal '/tmp/result.json', error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		if err := os.WriteFile(result, resultBytes, 0o644); err != nil {
			logger.Errorf("failed to write '/tmp/result.json', error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}
		logger.Infof("result file saved to %q", result)

		dropletOutFile, err := os.OpenFile(dropletFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			logger.Errorf("failed to open %q, error: %s\n", dropletFile, err.Error())
			return errors.ErrGenericBuild
		}
		defer dropletOutFile.Close()

		dgw := gzip.NewWriter(dropletOutFile)
		defer dgw.Close()

		if err := staging.RemoveBuildOnlyLayers(layersDir, bGroup.Group, logger); err != nil {
			logger.Errorf("failed to remove build-only layers, error: %s\n", err.Error())
			return errors.ErrExporting
		}
		if err := archive.FromDirectory(filepath.Dir(workspaceDir), tar.NewWriter(dgw)); err != nil {
			logger.Errorf("failed 'export' phase, error: %s\n", err.Error())
			return errors.ErrExporting
		}
		logger.Infof("droplet archive saved to %q", dropletFile)

		return nil
	},
}

func writeAnalyzed(path string, logger *log.Logger) (files.Analyzed, error) {
	analyzed := files.Analyzed{
		RunImage: &files.RunImage{
			TargetMetadata: &files.TargetMetadata{
				OS:   "linux",
				Arch: runtime.GOARCH,
			},
		},
	}

	return analyzed, files.Handler.WriteAnalyzed(path, &analyzed, logger)
}

// cloudfoundry expects a web process type to exist
// this will create a web process type identical to the default process type set by the buildpack if one does not exist
func ensureWebProcessType(buildMeta *files.BuildMetadata) {
	var defaultProcess launch.Process
	hasWebProcessType := false
	for _, process := range buildMeta.Processes {

		if process.Type == "web" {
			hasWebProcessType = true
			break
		}

		if process.Type == buildMeta.BuildpackDefaultProcessType || len(buildMeta.Processes) == 1 {
			defaultProcess = process
		}
	}
	if !hasWebProcessType && defaultProcess.Type != "" {
		defaultProcess.Type = "web"
		buildMeta.Processes = append(buildMeta.Processes, defaultProcess)
	}
}
