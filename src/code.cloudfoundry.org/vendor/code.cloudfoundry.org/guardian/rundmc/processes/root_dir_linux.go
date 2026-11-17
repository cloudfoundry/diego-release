package processes

import (
	"errors"
	"os"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
)

func getRootDir(rootFS, specDir, home string) string {
	dir := specDir
	if dir == "" {
		dir = home
	}

	if dir == "" {
		return "/"
	}

	h, err := pathrs.OpenInRoot(rootFS, dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "/"
		}
		return dir
	}
	_ = h.Close()

	return dir
}
