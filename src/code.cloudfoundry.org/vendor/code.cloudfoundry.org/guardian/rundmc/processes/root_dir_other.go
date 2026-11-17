//go:build !linux
// +build !linux

package processes

import (
	"os"
	"path/filepath"
)

func getRootDir(rootFS, specDir, home string) string {
	dir := specDir
	if dir == "" {
		dir = home
	}

	if dir == "" {
		return "/"
	}

	fullPath := filepath.Join(rootFS, dir)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "/"
	}

	return dir
}
