package archive

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func FromDirectory(baseDir string, tw Writer) error {
	var err error

	baseDir = filepath.Clean(baseDir)
	if !filepath.IsAbs(baseDir) {
		baseDir, err = filepath.Abs(baseDir)
		if err != nil {
			return err
		}
	}

	buf := make([]byte, 32*32*1024)
	if err := filepath.Walk(baseDir, func(path string, fi fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if baseDir == path {
			return nil
		}

		if fi.Mode()&os.ModeSocket != 0 {
			return nil
		}

		th, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		th.Name, err = filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}

			if filepath.IsAbs(link) && strings.HasPrefix(link, baseDir) {
				link, err = filepath.Rel(filepath.Dir(path), link)
				if err != nil {
					return err
				}
			}

			th.Linkname = link
		}

		if err := tw.WriteHeader(th); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}

		if _, err := io.CopyBuffer(tw, f, buf); err != nil {
			return err
		}

		if err := f.Close(); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return nil
}
