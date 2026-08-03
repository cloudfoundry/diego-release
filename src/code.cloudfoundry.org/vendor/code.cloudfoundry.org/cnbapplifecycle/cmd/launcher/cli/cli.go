package cli

import (
	"os"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle/api"
	"github.com/buildpacks/lifecycle/cmd"
	cli "github.com/buildpacks/lifecycle/cmd/launcher/cli"
	"github.com/buildpacks/lifecycle/env"
	"github.com/buildpacks/lifecycle/launch"
	platform "github.com/buildpacks/lifecycle/platform/launch"
	"github.com/spf13/cobra"

	builderCli "code.cloudfoundry.org/cnbapplifecycle/cmd/builder/cli"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/credhub"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/databaseuri"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/errors"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"
)

const (
	defaultProcessType  = "web"
	launcherProcessType = "launcher"
)

var (
	credhubConnectionAttempts int
	credhubRetryDelay         time.Duration
)

func Execute() error {
	return launcherCmd.Execute()
}

func init() {
	launcherCmd.Flags().IntVar(&credhubConnectionAttempts, "credhub-connection-attempts", 3, "number of times that the credhub client will attempt to connect to credhub")
	launcherCmd.Flags().DurationVar(&credhubRetryDelay, "credhub-retry-delay", 1*time.Second, "delay duration that credhub client will wait before retries (ex. 1s, 2m, etc.)")
}

func findLaunchProcessType(processes []launch.Process, expectedCmd string) (string, bool) {
	for _, proc := range processes {
		command := append(proc.Command.Entries, proc.Args...)
		if expectedCmd == strings.Join(command, " ") {
			return proc.Type, false
		}
	}

	return launcherProcessType, true
}

var launcherCmd = &cobra.Command{
	Use:          "launcher",
	SilenceUsage: true,
	RunE: func(cobraCmd *cobra.Command, cmdArgs []string) error {
		return Launch(os.Args, &LifecycleLauncher{})
	},
}

func verifyBuildpackAPIs(bps []launch.Buildpack) error {
	for _, bp := range bps {
		if err := cmd.VerifyBuildpackAPI(cli.KindBuildpack, bp.ID, bp.API, cmd.DefaultLogger); err != nil {
			return err
		}
	}
	return nil
}

func Launch(osArgs []string, theLauncher TheLauncher) error {
	var md launch.Metadata
	var args []string
	logger := log.NewLogger()
	defaultProc := defaultProcessType

	osArgs = slices.DeleteFunc(osArgs, func(s string) bool {
		return s == ""
	})

	if _, err := toml.DecodeFile(launch.GetMetadataFilePath(cmd.EnvOrDefault(platform.EnvLayersDir, builderCli.DefaultLayersPath)), &md); err != nil {
		logger.Errorf("failed decoding, error: %s\n", err.Error())
		return errors.ErrLaunching
	}

	if err := verifyBuildpackAPIs(md.Buildpacks); err != nil {
		logger.Errorf("failed verifying buildpack API, error: %s\n", err.Error())
		return errors.ErrLaunching
	}

	if err := credhub.InterpolateServiceRefs(credhubConnectionAttempts, credhubRetryDelay); err != nil {
		logger.Error(err.Error())
		return errors.ErrLaunching
	}

	databaseUrl, err := databaseuri.ParseDatabaseURI(os.Getenv("VCAP_SERVICES"))
	if err != nil {
		logger.Errorf("failed to parse database URI, error: %s\n", err.Error())
		return errors.ErrLaunching
	}
	if databaseUrl != "" {
		err = os.Setenv("DATABASE_URL", databaseUrl)
		if err != nil {
			logger.Errorf("Unable to set DATABASE_URL envirionment variable: %v", err)
			return errors.ErrLaunching
		}
	}

	var self string
	var isSidecar bool
	if len(osArgs) > 1 {
		defaultProc = ""
		args = osArgs[2:]

		// Tasks are launched with a "--" prefix, all other processes are launched with "app"
		if osArgs[1] == "--" {
			self = launcherProcessType
		} else {
			self, isSidecar = findLaunchProcessType(md.Processes, strings.Join(osArgs[2:], " "))
			logger.Infof("Detected process type: %q, isSidecar: %v", self, isSidecar)

			if !isSidecar {
				defaultProc = self
				args = nil
			}
		}
	}

	launcher := &launch.Launcher{
		DefaultProcessType: defaultProc,
		LayersDir:          cmd.EnvOrDefault(platform.EnvLayersDir, builderCli.DefaultLayersPath),
		AppDir:             cmd.EnvOrDefault(platform.EnvAppDir, builderCli.DefaultWorkspacePath),
		PlatformAPI:        api.MustParse(builderCli.PlatformAPI),
		Processes:          md.Processes,
		Buildpacks:         md.Buildpacks,
		Env:                env.NewLaunchEnv(os.Environ(), launch.ProcessDir, "/tmp/lifecycle"),
		Exec:               launch.OSExecFunc,
		ExecD:              launch.NewExecDRunner(),
		Shell:              launch.DefaultShell,
		Setenv:             os.Setenv,
	}

	if err := theLauncher.Launch(launcher, self, args); err != nil {
		logger.Errorf("failed launching with self: %q, defaultProc: %q, args: %#v, error: %s\n", self, defaultProc, args, err.Error())
		return errors.ErrLaunching
	}

	return nil
}
