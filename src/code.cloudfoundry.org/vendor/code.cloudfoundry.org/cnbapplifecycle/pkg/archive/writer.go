package archive

import (
	"archive/tar"
)

type Writer interface {
	WriteHeader(*tar.Header) error
	Write([]byte) (int, error)
	Close() error
}
