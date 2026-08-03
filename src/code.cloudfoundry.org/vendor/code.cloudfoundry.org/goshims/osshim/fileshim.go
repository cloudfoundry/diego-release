package osshim

import (
	"os"
)

type FileShim struct {
	Delegate *os.File
}

func (f *FileShim) Name() string {
	return f.Delegate.Name()
}

func (f *FileShim) Fd() uintptr {
	return f.Delegate.Fd()
}

func (f *FileShim) Close() error {
	return f.Delegate.Close()
}

func (f *FileShim) Stat() (os.FileInfo, error) {
	return f.Delegate.Stat()
}

func (f *FileShim) Read(b []byte) (n int, err error) {
	return f.Delegate.Read(b)
}

func (f *FileShim) ReadAt(b []byte, off int64) (n int, err error) {
	return f.Delegate.ReadAt(b, off)
}

func (f *FileShim) Write(b []byte) (n int, err error) {
	return f.Delegate.Write(b)
}

func (f *FileShim) WriteAt(b []byte, off int64) (n int, err error) {
	return f.Delegate.WriteAt(b, off)
}

func (f *FileShim) Seek(offset int64, whence int) (ret int64, err error) {
	return f.Delegate.Seek(offset, whence)
}

func (f *FileShim) WriteString(s string) (n int, err error) {
	return f.Delegate.WriteString(s)
}

func (f *FileShim) Chdir() error {
	return f.Delegate.Chdir()
}
