//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

func getLauncher(entrypointPrefix string) string {
	entryPoint := "bash -c"
	if entrypointPrefix != "" {
		entryPoint = entrypointPrefix
	}
	return fmt.Sprintf(`
cd "$1"

echo '%s'

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

if [ -f .profile ]; then
  source .profile
fi

shift

echo '%s'

exec %s "$@"
`, preStartMessage, startMessage, entryPoint)
}

func runProcess(dir, command, entrypointPrefix string) error {
	return syscall.Exec("/bin/bash", []string{
		"bash",
		"-c",
		getLauncher(entrypointPrefix),
		os.Args[0],
		dir,
		command,
	}, os.Environ())
}
