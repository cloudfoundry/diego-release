package runner

import (
	"os"
	"os/exec"
	"syscall"
)

func MustMountTmpfs(destination string) {
	if destination == "" {
		return
	}

	if _, err := os.Stat(destination); os.IsNotExist(err) {
		must(os.MkdirAll(destination, 0755))
	}

	if err := exec.Command("mountpoint", destination).Run(); err != nil {
		must(syscall.Mount("tmpfs", destination, "tmpfs", 0, ""))
	}
}

func MustUnmountTmpfs(destination string) {
	if destination == "" {
		return
	}

	must(syscall.Unmount(destination, 0))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
