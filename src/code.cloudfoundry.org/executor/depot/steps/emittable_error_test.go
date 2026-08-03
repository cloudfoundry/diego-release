package steps_test

import (
	"errors"

	"code.cloudfoundry.org/executor/depot/steps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EmittableError", func() {
	wrappedError := errors.New("the wrapped error")

	It("should satisfy the error interface", func() {
		err := steps.NewEmittableError(wrappedError, "Fancy")
		Expect(err).To(HaveOccurred())
	})

	Describe("WrappedError", func() {
		It("returns the wrapped error message", func() {
			err := steps.NewEmittableError(wrappedError, "Fancy emittable message")
			Expect(err.WrappedError()).To(Equal(wrappedError))
		})

		Context("when the wrapped error is nil", func() {
			It("should not blow up", func() {
				err := steps.NewEmittableError(nil, "Fancy emittable message")
				Expect(err.WrappedError()).To(BeNil())
			})
		})
	})

	Describe("Error", func() {
		Context("with no format args", func() {
			It("should just be the message", func() {
				args := []interface{}{}
				Expect(steps.NewEmittableError(wrappedError, "Fancy %s %d", args...).Error()).To(Equal("Fancy %s %d"))
			})
		})

		Context("with format args", func() {
			It("should Sprintf", func() {
				Expect(steps.NewEmittableError(wrappedError, "Fancy %s %d", "hi", 3).Error()).To(Equal("Fancy hi 3"))
			})
		})
	})
})
