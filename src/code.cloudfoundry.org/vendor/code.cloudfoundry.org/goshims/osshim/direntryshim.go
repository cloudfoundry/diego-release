package osshim

import (
	"io/fs"
	"os"
)

type DirEntryShim struct {
	Delegate os.DirEntry
}

func (de *DirEntryShim) Name() string {
	return de.Delegate.Name()
}

func (de *DirEntryShim) IsDir() bool {
	return de.Delegate.IsDir()
}

func (de *DirEntryShim) Type() fs.FileMode {
	return de.Delegate.Type()
}

func (de *DirEntryShim) Info() (fs.FileInfo, error) {
	return de.Delegate.Info()
}
