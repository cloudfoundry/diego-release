package osshim

import "os"

//go:generate counterfeiter -o os_fake/fake_file.go . File
type File interface {
	Name() string
	Fd() uintptr
	Close() error
	Stat() (os.FileInfo, error)
	Read(b []byte) (n int, err error)
	ReadAt(b []byte, off int64) (n int, err error)
	Write(b []byte) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	Seek(offset int64, whence int) (ret int64, err error)
	WriteString(s string) (n int, err error)
	Chdir() error
}
