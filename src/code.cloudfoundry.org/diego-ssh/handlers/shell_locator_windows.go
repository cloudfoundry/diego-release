//go:build windows

package handlers

import "os/exec"

type shellLocator struct{}

func NewShellLocator() ShellLocator {
	return &shellLocator{}
}

func (shellLocator) ShellPath() string {
	for _, shell := range []string{"cmd.exe"} {
		if path, err := exec.LookPath(shell); err == nil {
			return path
		}
	}

	return "cmd.exe"
}
