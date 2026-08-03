package netns

import (
	"fmt"
	"os"
	"runtime"

	vishnetns "github.com/vishvananda/netns"
)

type Execer struct{}

func (e Execer) Exec(nsPath *os.File, cb func() error) error {
	return Exec(nsPath, cb)
}

func Exec(fd *os.File, cb func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	newns := vishnetns.NsHandle(int(fd.Fd()))
	origns, _ := vishnetns.Get()
	defer origns.Close()

	if err := vishnetns.Set(newns); err != nil {
		return fmt.Errorf("set netns: %s", err)
	}

	err := cb()
	mustSetNs(origns) // if this happens we definitely can't recover
	return err
}

func mustSetNs(ns vishnetns.NsHandle) {
	if err := vishnetns.Set(ns); err != nil {
		panic(err)
	}
}
