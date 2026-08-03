package auctiontypes_test

import (
	"code.cloudfoundry.org/auction/auctiontypes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types", func() {
	Describe("PlacementTagMismatchError", func() {
		var (
			err  error
			tags []string
		)

		BeforeEach(func() {
			err = nil
			tags = nil
		})

		JustBeforeEach(func() {
			err = auctiontypes.NewPlacementTagMismatchError(tags)
		})

		Context("when there are no tags", func() {
			It("says no placement tag", func() {
				Expect(err.Error()).To(Equal("found no compatible cell with no placement tags"))
			})
		})

		Context("when there is one tag", func() {
			BeforeEach(func() {
				tags = []string{"a"}
			})

			It("list that tag in the error message", func() {
				Expect(err.Error()).To(Equal("found no compatible cell with placement tag \"a\""))
			})
		})

		Context("when there are two tags", func() {
			BeforeEach(func() {
				tags = []string{"a", "b"}
			})

			It("list that tag in the error message", func() {
				Expect(err.Error()).To(Equal("found no compatible cell with placement tags \"a\" and \"b\""))
			})
		})

		Context("when there are more than two tags", func() {
			BeforeEach(func() {
				tags = []string{"a", "b", "c"}
			})

			It("list that tag in the error message", func() {
				Expect(err.Error()).To(Equal("found no compatible cell with placement tags \"a\", \"b\" and \"c\""))
			})
		})
	})

	Describe("ErrorVolumeDriverMismatch", func() {
		It("prints the proper error message", func() {
			err := auctiontypes.ErrorVolumeDriverMismatch
			Expect(err.Error()).To(Equal("found no compatible cell with required volume drivers"))
		})
	})

	Describe("ErrorCellMismatch", func() {
		It("prints the proper error message", func() {
			err := auctiontypes.ErrorCellMismatch
			Expect(err.Error()).To(Equal("found no compatible cell for required rootfs"))
		})
	})
})
