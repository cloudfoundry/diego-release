package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/heroku/color"

	"github.com/buildpacks/lifecycle/api"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/env"
	"github.com/buildpacks/lifecycle/launch"
	platform "github.com/buildpacks/lifecycle/platform/launch"
)

const KindBuildpack = "buildpack"

func RunLaunch() error {
	color.Disable(boolEnv(platform.EnvNoColor))

	platformAPI := cmd.EnvOrDefault(platform.EnvPlatformAPI, platform.DefaultPlatformAPI)
	if err := cmd.VerifyPlatformAPI(platformAPI, cmd.DefaultLogger); err != nil {
		cmd.Exit(err)
	}
	p := platform.NewPlatform(platformAPI)

	var md launch.Metadata
	if _, err := toml.DecodeFile(launch.GetMetadataFilePath(cmd.EnvOrDefault(platform.EnvLayersDir, platform.DefaultLayersDir)), &md); err != nil {
		return cmd.FailErr(err, "read metadata")
	}
	if err := verifyBuildpackAPIs(md.Buildpacks); err != nil {
		return err
	}

	defaultProcessType := defaultProcessType(p.API(), md)

	launcher := &launch.Launcher{
		DefaultProcessType: defaultProcessType,
		LayersDir:          cmd.EnvOrDefault(platform.EnvLayersDir, platform.DefaultLayersDir),
		AppDir:             cmd.EnvOrDefault(platform.EnvAppDir, platform.DefaultAppDir),
		PlatformAPI:        p.API(),
		Processes:          md.Processes,
		Buildpacks:         md.Buildpacks,
		Env:                env.NewLaunchEnv(os.Environ(), launch.ProcessDir, launch.LifecycleDir),
		Exec:               launch.OSExecFunc,
		ExecD:              launch.NewExecDRunner(),
		Shell:              launch.DefaultShell,
		Setenv:             os.Setenv,
	}

	if err := launcher.Launch(os.Args[0], os.Args[1:]); err != nil {
		return cmd.FailErrCode(err, p.CodeFor(platform.LaunchError), "launch")
	}
	return nil
}

func boolEnv(k string) bool {
	v := os.Getenv(k)
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

func defaultProcessType(platformAPI *api.Version, launchMD launch.Metadata) string {
	if pType := os.Getenv(platform.EnvProcessType); pType != "" {
		cmd.DefaultLogger.Warnf("CNB_PROCESS_TYPE is not supported in Platform API %s", platformAPI)
		cmd.DefaultLogger.Warnf("Run with ENTRYPOINT '%s' to invoke the '%s' process type", pType, pType)
	}

	_, process := filepath.Split(os.Args[0])
	processType := strings.TrimSuffix(process, platform.DefaultExecExt)
	if _, ok := launchMD.FindProcessType(processType); ok {
		return processType
	}
	return ""
}

func verifyBuildpackAPIs(bps []launch.Buildpack) error {
	for _, bp := range bps {
		if err := cmd.VerifyBuildpackAPI(KindBuildpack, bp.ID, bp.API, cmd.DefaultLogger); err != nil {
			return err
		}
	}
	return nil
}
