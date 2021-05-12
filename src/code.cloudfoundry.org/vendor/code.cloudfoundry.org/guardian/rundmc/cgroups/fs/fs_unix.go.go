package fs

import (
	"os"

	"golang.org/x/sys/unix"
)

//go:generate counterfeiter . FS
type FS interface {
	Chown(string, int, int) error
	Mount(string, string, string, uintptr, string) error
}

type Impl struct{}

func Functions() *Impl {
	return new(Impl)
}

func (i Impl) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (i Impl) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return unix.Mount(source, target, fstype, flags, data)
}
