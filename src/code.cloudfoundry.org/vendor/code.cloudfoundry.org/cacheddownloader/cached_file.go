package cacheddownloader

import (
	"os"
	"runtime"
)

type CachedFile struct {
	*os.File

	onClose func(string)
}

func NewFileCloser(file *os.File, onClose func(string)) *CachedFile {
	fc := &CachedFile{
		File:    file,
		onClose: onClose,
	}

	runtime.SetFinalizer(fc, func(f *CachedFile) {
		// #nosec G104 - ignore this error closing a file. not that important and we dont have a logger plumbed
		f.Close()
	})

	return fc
}

func (fw *CachedFile) Close() error {
	err := fw.File.Close()
	if err != nil {
		return err
	}

	fw.onClose(fw.File.Name())
	runtime.SetFinalizer(fw, nil)

	return nil
}
