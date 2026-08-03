package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractWithBaseOverride(reader io.ReadCloser, oldBase, newBase string) error {
	tr := tar.NewReader(reader)
	buf := make([]byte, 32*32*1024)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ensureDir(newName(oldBase, newBase, hdr.Name)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := ensureDir(newName(oldBase, newBase, filepath.Dir(hdr.Name))); err != nil {
				return err
			}

			f, err := os.OpenFile(newName(oldBase, newBase, hdr.Name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}

			if _, err := io.CopyBuffer(f, tr, buf); err != nil {
				return err
			}

			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := ensureDir(newName(oldBase, newBase, filepath.Dir(hdr.Name))); err != nil {
				return err
			}

			if err := os.Symlink(newName(oldBase, newBase, hdr.Linkname), newName(oldBase, newBase, hdr.Name)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported file type: %d", hdr.Typeflag)
		}
	}

	return nil
}

func newName(oldBase, newBase, name string) string {
	if !filepath.IsAbs(name) {
		return name
	}

	return strings.Replace(name, oldBase, newBase, 1)
}

func ensureDir(name string) error {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return os.MkdirAll(name, 0o755)
	}

	return nil
}
