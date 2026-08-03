//go:build windows

package fs

import (
	"errors"
)

//counterfeiter:generate . FS
type FS interface {
	Chown(string, int, int) error
	Mount(string, string, string, uintptr, string) error
}

type Impl struct{}

func Functions() *Impl {
	return new(Impl)
}

func (i Impl) Chown(name string, uid, gid int) error {
	return errors.New("Not Implemented for windows")
}

func (i Impl) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return errors.New("Not Implemented for windows")
}
