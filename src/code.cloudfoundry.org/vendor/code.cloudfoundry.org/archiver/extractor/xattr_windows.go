//go:build windows

package extractor

import "archive/tar"

func setXattrsFromTar(_ string, _ *tar.Header) error {
	return nil
}
