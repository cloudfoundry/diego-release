//go:build windows

package diskcheck

// IsReadOnly is not implemented on Windows. Our Windows SME has indicated that
// read-only filesystem incidents of the type this check guards against (kernel
// remounting a filesystem read-only due to I/O errors) have never been observed
// on Windows Diego Cells. Windows implementation is therefore deferred pending
// evidence that it is needed. This stub ensures the package compiles on Windows
// while making the check a no-op.
func IsReadOnly(_ string) (bool, error) {
	return false, nil
}
