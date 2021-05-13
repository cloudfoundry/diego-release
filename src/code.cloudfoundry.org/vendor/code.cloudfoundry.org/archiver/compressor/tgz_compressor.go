package compressor

import (
	"compress/gzip"
	"os"
)

type Compressor interface {
	Compress(src string, dst string) error
}

func NewTgz() Compressor {
	return &tgzCompressor{}
}

type tgzCompressor struct{}

func (compressor *tgzCompressor) Compress(src string, dest string) error {
	fw, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer fw.Close()

	gw := gzip.NewWriter(fw)
	defer gw.Close()

	return WriteTar(src, gw)
}
