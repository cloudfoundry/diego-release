package version

import "fmt"

const (
	// VersionMajor is for an API incompatible changes
	VersionMajor = 5
	// VersionMinor is for functionality in a backwards-compatible manner
<<<<<<< HEAD
<<<<<<< HEAD
	VersionMinor = 34
=======
	VersionMinor = 33
>>>>>>> 9cd9aa6e7 (Update go.mod dependencies)
=======
	VersionMinor = 32
>>>>>>> a29b1afa1 (Update go.mod dependencies)
	// VersionPatch is for backwards-compatible bug fixes
	VersionPatch = 2

	// VersionDev indicates development branch. Releases will be empty string.
	VersionDev = ""
)

// Version is the specification version that the package types support.
var Version = fmt.Sprintf("%d.%d.%d%s", VersionMajor, VersionMinor, VersionPatch, VersionDev)
