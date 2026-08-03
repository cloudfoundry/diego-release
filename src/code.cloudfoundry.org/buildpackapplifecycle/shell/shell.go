//go:build !windows
// +build !windows

package shell

import (
	"fmt"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/buildpackapplifecycle/credhub_flags"
	"code.cloudfoundry.org/buildpackapplifecycle/env"
	"code.cloudfoundry.org/goshims/osshim"
)

type exec interface {
	Exec(dir, launcher, args, command string, environ []string) error
}

func Run(os osshim.Os, exec exec, shellArgs []string) error {
	var dir string
	var commands []string

	if len(shellArgs) >= 2 {
		dir = shellArgs[1]
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("Provided app direcory does not exist")
		}
	} else {
		dir = filepath.Join(os.Getenv("HOME"), "app")
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("Could not infer app directory, please provide one")
		}
	}
	if absDir, err := filepath.Abs(dir); err == nil {
		dir = absDir
	}

	argsToParseForFlags := []string{}
	if len(shellArgs) >= 3 {
		commands = shellArgs[2:]
		argsToParseForFlags = shellArgs[3:]
	} else {
		commands = []string{"bash"}
	}

	credhubFlags := credhub_flags.NewCredhubFlags("shell")
	err := credhubFlags.Parse(argsToParseForFlags)
	if err != nil {
		return fmt.Errorf("Could not parse credhub flags: %s", err)
	}
	attempts := credhubFlags.ConnectAttempts()
	delay := credhubFlags.RetryDelay()

	if err := env.CalcEnv(os, dir, attempts, delay); err != nil {
		return err
	}

	runtime.GOMAXPROCS(1)

	return exec.Exec(dir, launcher, shellArgs[0], commands[0], os.Environ())
}

const launcher = `
cd "$1"

if [ -n "$(ls ../profile.d/* 2> /dev/null)" ]; then
  for env_file in ../profile.d/*; do
    source $env_file
  done
fi

if [ -n "$(ls .profile.d/* 2> /dev/null)" ]; then
  for env_file in .profile.d/*; do
    source $env_file
  done
fi

shift

exec bash -c "$@"
`
