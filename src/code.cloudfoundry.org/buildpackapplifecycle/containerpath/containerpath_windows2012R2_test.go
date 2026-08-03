//go:build windows && windows2012R2
// +build windows,windows2012R2

package containerpath_test

import (
	"path/filepath"

	"code.cloudfoundry.org/buildpackapplifecycle/containerpath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("containerpath.For", func() {
	var subject interface {
		For(path ...string) string
	}
	BeforeEach(func() {
		subject = containerpath.New(filepath.Join("C:\\", "varrr", "veecap"))
	})

	It("returns paths relative to %USERPROFILE%", func() {
		Expect(subject.For(filepath.FromSlash("/foo/bar/baz"))).To(Equal(filepath.FromSlash("C:/varrr/veecap/foo/bar/baz")))
	})
})
