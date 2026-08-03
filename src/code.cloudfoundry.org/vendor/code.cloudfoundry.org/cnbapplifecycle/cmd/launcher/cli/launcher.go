package cli

import (
	"github.com/buildpacks/lifecycle/launch"
)

type TheLauncher interface {
	Launch(launcher *launch.Launcher, self string, cmd []string) error
}

type LifecycleLauncher struct {
}

var _ TheLauncher = &LifecycleLauncher{}

func (l *LifecycleLauncher) Launch(launcher *launch.Launcher, self string, cmd []string) error {
	return launcher.Launch(self, cmd)
}
