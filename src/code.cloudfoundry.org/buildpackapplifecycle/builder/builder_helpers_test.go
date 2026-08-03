package main_test

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func buildBuilder(additionalFlags []string) string {
	builder, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/builder", additionalFlags...)
	Expect(err).NotTo(HaveOccurred())

	return builder
}
