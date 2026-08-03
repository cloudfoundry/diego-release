package buildpacks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"
	"github.com/cespare/xxhash/v2"
)

func Translate(bps []string, buildpacksDir string, logger *log.Logger) ([]string, error) {
	newList := []string{}

	for _, bp := range bps {
		bpDir := buildpackPath(bp, buildpacksDir)
		downloaded, err := checkIfDownloaded(bpDir)
		if err != nil {
			return nil, err
		}

		if downloaded {
			newList = append(newList, fmt.Sprintf("file://%s", bpDir))
		} else {
			newList = append(newList, bp)
		}
	}

	return newList, nil
}

func checkIfDownloaded(path string) (bool, error) {
	fi, err := os.Stat(path)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	if !fi.IsDir() {
		return false, fmt.Errorf("%q is not a directory", path)
	}

	return true, nil
}

func buildpackPath(name string, path string) string {
	return filepath.Join(path, fmt.Sprintf("%016x", xxhash.Sum64String(name)))
}
