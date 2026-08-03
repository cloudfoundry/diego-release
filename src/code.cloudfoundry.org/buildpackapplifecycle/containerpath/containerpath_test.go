//go:build !windows2012R2
// +build !windows2012R2

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
		subject = containerpath.New("/oooo")
	})

	It("returns paths relative to root", func() {
		Expect(subject.For(filepath.FromSlash("/foo/bar/baz"))).To(Equal(filepath.FromSlash("/foo/bar/baz")))
	})
})
