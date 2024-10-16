package osshim

import "io/fs"

//go:generate counterfeiter -o os_fake/fake_direntry.go . DirEntry
type DirEntry interface {
	Name() string
	IsDir() bool
	Type() fs.FileMode
	Info() (fs.FileInfo, error)
}
