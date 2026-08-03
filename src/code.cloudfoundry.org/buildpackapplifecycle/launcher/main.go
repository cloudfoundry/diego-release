package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/buildpackapplifecycle/buildpackrunner"
	"code.cloudfoundry.org/buildpackapplifecycle/credhub_flags"
	"code.cloudfoundry.org/buildpackapplifecycle/env"
	"code.cloudfoundry.org/goshims/osshim"
	yaml "gopkg.in/yaml.v2"
)

var preStartMessage = "Invoking pre-start scripts."
var startMessage = "Invoking start command."

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "%s: received only %d arguments\n", os.Args[0], len(os.Args)-1)
		fmt.Fprintf(os.Stderr, "Usage: %s <app-directory> <start-command> <metadata>", os.Args[0])
		os.Exit(1)
	}

	dir := os.Args[1]
	startCommand := os.Args[2]

	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}

	stagingInfo, err := unmarhsalStagingInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid staging info - %s", err)
		os.Exit(1)
	}

	var command string
	if startCommand != "" {
		command = startCommand
	} else {
		command = stagingInfo.StartCommand
	}

	if command == "" {
		fmt.Fprintf(os.Stderr, "%s: no start command specified or detected in droplet", os.Args[0])
		os.Exit(1)
	}

	credhubFlags := credhub_flags.NewCredhubFlags("launcher")
	err = credhubFlags.Parse(os.Args[3:len(os.Args)])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not parse credhub flags: %s", os.Args[0], err)
		os.Exit(1)
	}

	attempts := credhubFlags.ConnectAttempts()
	delay := credhubFlags.RetryDelay()

	if err := env.CalcEnv(&osshim.OsShim{}, dir, attempts, delay); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(3)
	}

	runtime.GOMAXPROCS(1)
	err = runProcess(dir, command, stagingInfo.GetEntrypointPrefix())
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(4)
	}
}

func unmarhsalStagingInfo() (buildpackrunner.DeaStagingInfo, error) {
	stagingInfo := buildpackrunner.DeaStagingInfo{}
	stagingInfoData, err := os.ReadFile(buildpackrunner.DeaStagingInfoFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return stagingInfo, nil
		}
		return stagingInfo, err
	}

	err = yaml.Unmarshal(stagingInfoData, &stagingInfo)
	if err != nil {
		return stagingInfo, fmt.Errorf("failed to unmarshal %s: %w", buildpackrunner.DeaStagingInfoFilename, err)
	}

	return stagingInfo, nil
}
