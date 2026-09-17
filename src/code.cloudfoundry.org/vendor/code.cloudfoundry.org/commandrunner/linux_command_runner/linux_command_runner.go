//go:build !windows

package linux_command_runner // import "code.cloudfoundry.org/commandrunner/linux_command_runner"

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type RealCommandRunner struct{}

type CommandNotRunningError struct {
	cmd *exec.Cmd
}

func (e CommandNotRunningError) Error() string {
	return fmt.Sprintf("command is not running: %#v", e.cmd)
}

func New() *RealCommandRunner {
	return &RealCommandRunner{}
}

func (r *RealCommandRunner) Run(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	} else {
		cmd.SysProcAttr.Setpgid = true
	}

	return cmd.Run()
}

func (r *RealCommandRunner) Start(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	} else {
		cmd.SysProcAttr.Setpgid = true
	}

	return cmd.Start()
}

func (r *RealCommandRunner) Background(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	} else {
		cmd.SysProcAttr.Setpgid = true
	}

	return cmd.Start()
}

func (r *RealCommandRunner) Wait(cmd *exec.Cmd) error {
	return cmd.Wait()
}

func (r *RealCommandRunner) Kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return CommandNotRunningError{cmd}
	}

	return cmd.Process.Kill()
}

func (r *RealCommandRunner) Signal(cmd *exec.Cmd, signal os.Signal) error {
	if cmd.Process == nil {
		return CommandNotRunningError{cmd}
	}

	return cmd.Process.Signal(signal)
}
