package cli

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle/api"
	"github.com/buildpacks/lifecycle/cmd"
	cli "github.com/buildpacks/lifecycle/cmd/launcher/cli"
	"github.com/buildpacks/lifecycle/env"
	"github.com/buildpacks/lifecycle/launch"
	platform "github.com/buildpacks/lifecycle/platform/launch"
	"github.com/spf13/cobra"

	builderCli "code.cloudfoundry.org/cnbapplifecycle/cmd/builder/cli"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/errors"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"
)

const defaultProcessType = "web"

func Execute() error {
	return launcherCmd.Execute()
}

var launcherCmd = &cobra.Command{
	Use:          "launcher",
	SilenceUsage: true,
	RunE: func(cobraCmd *cobra.Command, cmdArgs []string) error {
		var md launch.Metadata
		var args []string
		logger := log.NewLogger()
		self := defaultProcessType
		defaultProc := defaultProcessType

		if _, err := toml.DecodeFile(launch.GetMetadataFilePath(cmd.EnvOrDefault(platform.EnvLayersDir, builderCli.DefaultLayersPath)), &md); err != nil {
			logger.Errorf("failed decoding, error: %s\n", err.Error())
			return errors.ErrLaunching
		}

		if err := verifyBuildpackAPIs(md.Buildpacks); err != nil {
			logger.Errorf("failed verifying buildpack API, error: %s\n", err.Error())
			return errors.ErrLaunching
		}

		if len(os.Args) > 1 && os.Args[1] == "--" {
			self = "launcher"
			args = os.Args[2:]
			defaultProc = ""
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

		if err := launcher.Launch(self, args); err != nil {
			logger.Errorf("failed launching with self: %q, defaultProc: %q, args: %#v, error: %s\n", self, defaultProc, args, err.Error())
			return errors.ErrLaunching
		}

		return nil
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
