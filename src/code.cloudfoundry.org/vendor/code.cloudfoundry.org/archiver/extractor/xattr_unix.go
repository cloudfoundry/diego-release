//go:build unix

package extractor

import (
	"archive/tar"
	"errors"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func setXattrsFromTar(path string, hdr *tar.Header) (err error) {
	const paxSchilyXattr = "SCHILY.xattr."

	for key, value := range hdr.PAXRecords {
		if !strings.HasPrefix(key, paxSchilyXattr) {
			continue
		}

		err = unix.Lsetxattr(path, key[len(paxSchilyXattr):], []byte(value), 0)
		if err != nil && !errors.Is(err, syscall.ENOTSUP) && !errors.Is(err, syscall.EPERM) {
			return err
		}
	}

	return nil
}
