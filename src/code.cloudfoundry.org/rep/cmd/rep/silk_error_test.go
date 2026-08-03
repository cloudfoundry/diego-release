package main

import (
	"errors"
	"fmt"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("isTransientSilkError", func() {
	DescribeTable("returns true for transient silk-daemon connection errors",
		func(err error) {
			Expect(isTransientSilkError(err)).To(BeTrue())
		},
		Entry("ECONNREFUSED – socket exists but nothing listening", syscall.ECONNREFUSED),
		Entry("ENOENT – socket file does not exist yet", syscall.ENOENT),
		Entry("ECONNRESET – daemon starting but dropped connection", syscall.ECONNRESET),
		Entry("wrapped ECONNREFUSED", fmt.Errorf("dial failed: %w", syscall.ECONNREFUSED)),
		Entry("wrapped ENOENT", fmt.Errorf("stat failed: %w", syscall.ENOENT)),
		Entry("wrapped ECONNRESET", fmt.Errorf("read failed: %w", syscall.ECONNRESET)),
	)

	DescribeTable("returns false for genuine (non-transient) errors",
		func(err error) {
			Expect(isTransientSilkError(err)).To(BeFalse())
		},
		Entry("generic error", errors.New("something else went wrong")),
		Entry("EPERM – permission denied", syscall.EPERM),
		Entry("EACCES – access denied", syscall.EACCES),
		Entry("EINVAL – invalid argument", syscall.EINVAL),
		Entry("nil error", nil),
	)
})
