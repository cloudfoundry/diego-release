//go:build !windows
// +build !windows

package exec

import (
	"syscall"
)

type exec struct{}

func New() *exec {
	return &exec{}
}

func (e *exec) Exec(dir, launcher, executable, command string, environ []string) error {
	return syscall.Exec("/bin/bash", []string{
		"bash",
		"-c",
		launcher,
		executable,
		dir,
		command,
	}, environ)
}
