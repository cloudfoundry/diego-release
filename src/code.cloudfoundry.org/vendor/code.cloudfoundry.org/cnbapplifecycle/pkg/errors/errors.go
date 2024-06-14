package errors

import "errors"

var (
	ErrGenericBuild         = errors.New("generic build failure")
	ErrDownloadingBuildpack = errors.New("downloading buildpacks failed")
	ErrDetecting            = errors.New("detecting failed")
	ErrRestoring            = errors.New("restoring failed")
	ErrBuilding             = errors.New("building failed")
	ErrExporting            = errors.New("exporting failed")
	ErrLaunching            = errors.New("launching failed")
)

var errorMapping = map[error]int{
	ErrGenericBuild:         231,
	ErrDownloadingBuildpack: 232,
	ErrDetecting:            233,
	ErrBuilding:             234,
	ErrExporting:            235,
	ErrLaunching:            236,
	ErrRestoring:            237,
}

func ExitCodeFromError(err error) int {
	rc, ok := errorMapping[err]

	if !ok {
		rc = 1
	}

	return rc
}
