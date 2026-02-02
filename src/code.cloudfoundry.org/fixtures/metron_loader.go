package fixtures

import (
	"os"
	"path/filepath"
	"runtime"
)

// anchorDir returns the absolute path to ".../src/code.cloudfoundry.org".
func anchorDir() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)

	for {
		// stop when we are at .../src/code.cloudfoundry.org
		if filepath.Base(dir) == "code.cloudfoundry.org" &&
			filepath.Base(filepath.Dir(dir)) == "src" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic(`could not find "src/code.cloudfoundry.org" in path`)
		}
		dir = parent
	}
}

func fixturesRoot() string {
	root := filepath.Join(anchorDir(), "fixtures")
	if _, err := os.Stat(root); err != nil {
		panic("fixtures directory not found at " + root)
	}
	return root
}

// Path returns a path inside "fixtures/metron", defaulting the "metron" subdir.
func Path(name string) string {
	return filepath.Join(fixturesRoot(), "metron", name)
}
