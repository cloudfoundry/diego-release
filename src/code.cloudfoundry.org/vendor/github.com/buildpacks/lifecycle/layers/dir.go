package layers

import (
	"path/filepath"

	"github.com/buildpacks/lifecycle/archive"
)

// DirLayer creates a layer from the given directory
// DirLayer will set the UID and GID of entries describing dir and its children (but not its parents)
//
//	to Factory.UID and Factory.GID
func (f *Factory) DirLayer(withID string, fromDir string, createdBy string) (layer Layer, err error) {
	fromDir, err = filepath.Abs(fromDir)
	if err != nil {
		return Layer{}, err
	}
	parents, err := parents(fromDir)
	if err != nil {
		return Layer{}, err
	}
	return f.writeLayer(withID, createdBy, func(tw *archive.NormalizingTarWriter) error {
		if err := archive.AddFilesToArchive(tw, parents); err != nil {
			return err
		}
		tw.WithUID(f.UID)
		tw.WithGID(f.GID)
		return archive.AddDirToArchive(tw, fromDir)
	})
}
