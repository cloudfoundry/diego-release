package local

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/buildpacks/imgutil"
)

// ociPlatform converts p for a platform-aware daemon call. It returns false when
// p does not name both an OS and an architecture: the daemon rejects such a
// platform on inspect, history and save with "both OS and Architecture must be
// provided", so the caller must make the call without a platform instead.
func ociPlatform(p imgutil.Platform) (ocispec.Platform, bool) {
	if p.OS == "" || p.Architecture == "" {
		return ocispec.Platform{}, false
	}
	return ocispec.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		OSVersion:    p.OSVersion,
		Variant:      p.Variant,
	}, true
}
