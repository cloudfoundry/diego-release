package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/cnbapplifecycle/pkg/archive"
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
	PlatformAPI          = "0.13"
	DefaultLayersPath    = "/home/vcap/layers"
	DefaultWorkspacePath = "/home/vcap/workspace"
)

var (
	layersDir       string
	workspaceDir    string
	cacheDir        string
	cacheOutputFile string
	result          string
	dropletFile     string
	buildpacks      []string
	envVarNames     []string
)

var (
	platformDir      = filepath.Join(os.TempDir(), "platform")
	buildpacksDir    = filepath.Join(os.TempDir(), "buildpacks")
	extensionsDir    = filepath.Join(os.TempDir(), "extensions")
	downloadCacheDir = filepath.Join(os.TempDir(), "download-cache")
)

func Execute() error {
	return builderCmd.Execute()
}

func init() {
	builderCmd.Flags().StringSliceVarP(&buildpacks, "buildpack", "b", nil, "buildpack(s) to use")
	builderCmd.Flags().StringVarP(&dropletFile, "droplet", "d", "/tmp/droplet", "output droplet file")
	builderCmd.Flags().StringVarP(&result, "result", "r", "/tmp/result.json", "result file")
	builderCmd.Flags().StringVarP(&workspaceDir, "workspace-dir", "w", DefaultWorkspacePath, "app workspace dir")
	builderCmd.Flags().StringVarP(&layersDir, "layers", "l", DefaultLayersPath, "layers dir")
	builderCmd.Flags().StringSliceVarP(&envVarNames, "pass-env-var", "", nil, "environment variable(s) to pass to buildpacks")
	builderCmd.Flags().StringVarP(&cacheDir, "cache-dir", "c", "/tmp/cache", "cache dir")
	builderCmd.Flags().StringVarP(&cacheOutputFile, "cache-output", "", "/tmp/cache-output.tgz", "cache output")
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

		for _, dir := range []string{layersDir, platformDir, buildpacksDir, extensionsDir, downloadCacheDir, cacheDir} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				logger.Errorf("failed to create %q, error: %s\n", dir, err.Error())
				return errors.ErrGenericBuild
			}
		}

		if err := staging.CreateEnvFiles(platformDir, envVarNames); err != nil {
			logger.Errorf("failed to write env var files, error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		orderFile, err := os.Create(filepath.Join(buildpacksDir, "order.toml"))
		if err != nil {
			logger.Errorf("failed to create 'order.toml', error: %s\n", err.Error())
			return errors.ErrGenericBuild
		}

		creds, err := keychain.FromEnv()
		if err != nil {
			logger.Errorf("failed to parse %s environment variable, error: %s\n", keychain.CnbCredentialsEnv, err.Error())
			return errors.ErrGenericBuild
		}
		err = staging.DownloadBuildpacks(
			buildpacks,
			buildpacksDir,
			image.NewFetcher(logger, nil, image.WithKeychain(creds)),
			blob.NewDownloader(logger, downloadCacheDir, blob.WithClient(keychain.NewHTTPClient(creds))),
			orderFile,
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
		cache, err := cache.NewVolumeCache(cacheDir)
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
			AnalyzeMD:     files.Analyzed{},
		}

		logger.Phase("BUILDING")
		buildMeta, err := bldr.Build()
		if err != nil {
			logger.Errorf("failed 'build' phase, error: %s\n", err.Error())
			return errors.ErrBuilding
		}

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
