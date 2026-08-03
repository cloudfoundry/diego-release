package portauthority_test

import (
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Portallocator", func() {
	var (
		allocator portauthority.PortAllocator
		port      uint16
		err       error
	)

	BeforeEach(func() {
		allocator, err = portauthority.New(30, 65355)
		Expect(err).NotTo(HaveOccurred())
	})

	It("starts allocating at the beginning of the range, inclusive", func() {
		port, err = allocator.ClaimPorts(1)
		Expect(err).NotTo(HaveOccurred())
		Expect(port).To(Equal(uint16(30)))
	})

	It("returns a different int each time NextPort is called", func() {
		Expect(allocator.ClaimPorts(1)).Should(BeEquivalentTo(30))
		Expect(allocator.ClaimPorts(1)).Should(BeEquivalentTo(31))
		Expect(allocator.ClaimPorts(1)).Should(BeEquivalentTo(32))
	})

	Context("when the allocator runs out of ports in the range", func() {
		BeforeEach(func() {
			allocator, err = portauthority.New(30, 30)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			port, err := allocator.ClaimPorts(1)
			Expect(err).NotTo(HaveOccurred())
			Expect(port).To(BeEquivalentTo(30))

			port, err = allocator.ClaimPorts(1)
			Expect(port).To(BeZero())
			Expect(err).To(MatchError("insufficient ports available"))
		})
	})

	Context("when ClaimPorts is asked for more than one port", func() {
		BeforeEach(func() {
			port, err = allocator.ClaimPorts(4)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the first of those ports", func() {
			Expect(port).To(BeEquivalentTo(30))
		})

		It("skips those P ports the next time it is called", func() {
			port, err = allocator.ClaimPorts(4)
			Expect(err).NotTo(HaveOccurred())
			Expect(port).To(BeEquivalentTo(34))
		})

		Context("and there aren't enough ports available", func() {
			It("errors and returns 0 as a port", func() {
				port, err = allocator.ClaimPorts(65355)
				Expect(err).To(HaveOccurred())
				Expect(port).To(BeZero())
			})
		})
	})

	Context("when a range outside the port spec is requested", func() {
		It("errors", func() {
			allocator, err = portauthority.New(30, 65536)
			Expect(err).To(MatchError("Invalid port range requested. Ports can only be numbers between 0-65535"))
		})
	})
})
