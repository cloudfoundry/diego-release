package cacheddownloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
)

var ErrUnknownArchiveFormat = errors.New("unknown archive format")

func TarTransform(source string, destination string) (int64, error) {
	file, err := os.Open(source)
	if err != nil {
		return 0, err
	}

	mime, err := mimeType(file)
	if err != nil {
		return 0, err
	}

	err = file.Close()
	if err != nil {
		return 0, err
	}

	switch mime {
	case "application/x-gzip":
		gunzipPath, err := exec.LookPath("gunzip")
		if err == nil {
			return gunzipTarGZToTar(gunzipPath, source, destination)
		}
		return transformTarGZToTar(source, destination)

	case "application/zip":
		return transformZipToTar(source, destination)

	case "application/tar":
		return NoopTransform(source, destination)

	default:
		return 0, ErrUnknownArchiveFormat
	}
}

func mimeType(fd *os.File) (string, error) {
	data := make([]byte, 512)

	_, err := fd.Read(data)
	if err != nil {
		return "", err
	}

	_, err = fd.Seek(0, 0)
	if err != nil {
		return "", err
	}

	// check for tar magic string
	if string(data[257:257+6]) == "ustar\x00" {
		return "application/tar", nil
	}

	return http.DetectContentType(data), nil
}

func transformTarGZToTar(path, destPath string) (int64, error) {
	dest, err := os.OpenFile(destPath, os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer dest.Close()

	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}

	gr, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}

	// #nosec - G110 - We're fine with unbounded file decompression here as we have container filesystem quotas that will prevent this from eating up the entire diego cell disk space
	n, err := io.Copy(dest, gr)
	if err != nil {
		return 0, err
	}

	err = file.Close()
	if err != nil {
		return 0, err
	}

	err = os.Remove(path)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func gunzipTarGZToTar(gunzipPath, path, destPath string) (int64, error) {
	destFile, err := os.OpenFile(destPath, os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer destFile.Close()

	cmd := exec.Command(gunzipPath, "-c", path)
	cmd.Stdout = destFile
	err = cmd.Run()
	if err != nil {
		return 0, err
	}

	err = os.Remove(path)
	if err != nil {
		return 0, err
	}

	fileInfo, err := os.Stat(destPath)
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

func transformZipToTar(path, destPath string) (int64, error) {
	dest, err := os.OpenFile(destPath, os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer dest.Close()

	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0, err
	}
	defer zr.Close()

	tarWriter := tar.NewWriter(dest)

	for _, zipEntry := range zr.File {
		err := writeZipEntryToTar(tarWriter, zipEntry)
		if err != nil {
			return 0, err
		}
	}

	err = tarWriter.Close()
	if err != nil {
		return 0, err
	}

	fi, err := dest.Stat()
	if err != nil {
		return 0, err
	}

	err = zr.Close()
	if err != nil {
		return 0, err
	}

	err = os.Remove(path)
	if err != nil {
		return 0, err
	}

	return fi.Size(), nil
}

func writeZipEntryToTar(tarWriter *tar.Writer, zipEntry *zip.File) error {
	zipInfo := zipEntry.FileInfo()

	if zipInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		return writeSymlinkZipEntryToTar(tarWriter, zipEntry, zipInfo)
	} else {
		return writeRegularZipEntryToTar(tarWriter, zipEntry, zipInfo)
	}
}

func writeRegularZipEntryToTar(tarWriter *tar.Writer, zipEntry *zip.File, zipInfo os.FileInfo) error {
	tarHeader, err := tar.FileInfoHeader(zipInfo, "")
	if err != nil {
		return err
	}

	// file info only populates the base name; we want the full path
	tarHeader.Name = zipEntry.FileHeader.Name

	zipReader, err := zipEntry.Open()
	if err != nil {
		return err
	}

	defer zipReader.Close()

	err = tarWriter.WriteHeader(tarHeader)
	if err != nil {
		return err
	}

	// #nosec - G110 - We're fine with unbounded file decompression here as we have container filesystem quotas that will prevent this from eating up the entire diego cell disk space
	_, err = io.Copy(tarWriter, zipReader)
	if err != nil {
		return err
	}

	err = tarWriter.Flush()
	if err != nil {
		return err
	}

	return nil
}

func writeSymlinkZipEntryToTar(tarWriter *tar.Writer, zipEntry *zip.File, zipInfo os.FileInfo) error {
	zipReader, err := zipEntry.Open()
	if err != nil {
		return err
	}

	defer zipReader.Close()
	payload, err := io.ReadAll(zipReader)
	if err != nil {
		return err
	}
	link := string(payload)

	tarHeader, err := tar.FileInfoHeader(zipInfo, link)
	if err != nil {
		return err
	}

	// file info only populates the base name; we want the full path
	tarHeader.Name = zipEntry.FileHeader.Name

	err = tarWriter.WriteHeader(tarHeader)
	if err != nil {
		return err
	}

	err = tarWriter.Flush()
	if err != nil {
		return err
	}

	return nil
}
