// +build modules

package modules

import (
	_ "code.cloudfoundry.org/certsplitter/cmd/certsplitter"
)

// imporing modules that are needed for building and testing this module
// these modules are not imported in code, but they build binaries
// that are needed at runtime
