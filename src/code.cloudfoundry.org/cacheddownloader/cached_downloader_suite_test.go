package cacheddownloader_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCachedDownloader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CachedDownloader Suite")
}

func createTarBuffer(content string, numFiles int) *bytes.Buffer {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)
	// Add some files to the archive.
	var files = []struct {
		Name, Body string
		Type       byte
		Mode       int64
	}{
		{"readme.txt", "This archive contains some text files.", tar.TypeReg, 0600},
		{"diego.txt", "Diego names:\nVizzini\nGeoffrey\nPrincess Buttercup\n", tar.TypeReg, 0600},
		{"testdir", "", tar.TypeDir, 0766},
		{"testdir/file.txt", content, tar.TypeReg, 0600},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.Type,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatalln(err)
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			log.Fatalln(err)
		}
	}

	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("file_%d", i)
		hdr := &tar.Header{
			Name:     filename,
			Typeflag: tar.TypeReg,
			Mode:     0600,
			Size:     int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatalln(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			log.Fatalln(err)
		}

	}
	// Make sure to check the error on Close.
	if err := tw.Close(); err != nil {
		log.Fatalln(err)
	}

	return buf
}
