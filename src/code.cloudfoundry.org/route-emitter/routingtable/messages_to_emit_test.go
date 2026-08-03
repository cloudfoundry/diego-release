package routingtable_test

import (
	"code.cloudfoundry.org/route-emitter/routingtable"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessagesToEmit", func() {
	var (
		messagesToEmit   routingtable.MessagesToEmit
		messages1        []routingtable.RegistryMessage
		internalMessages []routingtable.RegistryMessage
	)

	BeforeEach(func() {
		messagesToEmit = routingtable.MessagesToEmit{}
		messages1 = []routingtable.RegistryMessage{
			{
				Host: "1.1.1.1",
				Port: 61000,
				App:  "log-guid-2",
				URIs: []string{"host1.example.com"},
			},
			{
				Host: "1.1.1.1",
				Port: 61001,
				App:  "log-guid-1",
				URIs: []string{"host1.example.com"},
			},
			{
				Host: "1.1.1.1",
				Port: 61003,
				App:  "log-guid-2",
				URIs: []string{"host2.example.com", "host3.example.com"},
			},
			{
				Host: "1.1.1.1",
				Port: 61004,
				App:  "log-guid-3",
				URIs: []string{"host3.example.com"},
			},
		}

		internalMessages = []routingtable.RegistryMessage{
			{
				Host: "1.1.1.1",
				URIs: []string{"host1.internal.local", "host2.internal.local"},
			},
			{
				Host: "1.1.1.1",
				URIs: []string{"host3.internal.local", "host4.internal.local"},
			},
		}
	})

	Describe("RouteRegistrationCount", func() {
		Context("when there are registration messages", func() {
			BeforeEach(func() {
				messagesToEmit.RegistrationMessages = messages1
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteRegistrationCount()).To(BeEquivalentTo(5))
			})
		})

		Context("when registration messages is nil", func() {
			BeforeEach(func() {
				messagesToEmit.RegistrationMessages = nil
				messagesToEmit.UnregistrationMessages = messages1
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteRegistrationCount()).To(BeEquivalentTo(0))
			})
		})
	})

	Describe("RouteUnregistrationCount", func() {
		Context("when there are unregistration messages", func() {
			BeforeEach(func() {
				messagesToEmit.UnregistrationMessages = messages1
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteUnregistrationCount()).To(BeEquivalentTo(5))
			})
		})

		Context("when registration messages is nil", func() {
			BeforeEach(func() {
				messagesToEmit.RegistrationMessages = messages1
				messagesToEmit.UnregistrationMessages = nil
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteUnregistrationCount()).To(BeEquivalentTo(0))
			})
		})
	})

	Describe("InternalRouteRegistrationCount", func() {
		Context("when there are internal registration messages", func() {
			BeforeEach(func() {
				messagesToEmit.InternalRegistrationMessages = internalMessages
			})

			It("adds the number of hostnames in each internal route message", func() {
				Expect(messagesToEmit.InternalRouteRegistrationCount()).To(BeEquivalentTo(4))
			})
		})

		Context("when internal registration messages is nil", func() {
			BeforeEach(func() {
				messagesToEmit.InternalRegistrationMessages = nil
				messagesToEmit.InternalUnregistrationMessages = internalMessages
			})

			It("adds the number of hostnames in each internal route message", func() {
				Expect(messagesToEmit.InternalRouteRegistrationCount()).To(BeEquivalentTo(0))
			})
		})
	})

	Describe("InternalRouteUnregistrationCount", func() {
		Context("when there are internal unregistration messages", func() {
			BeforeEach(func() {
				messagesToEmit.InternalUnregistrationMessages = internalMessages
			})

			It("adds the number of hostnames in each internal route message", func() {
				Expect(messagesToEmit.InternalRouteUnregistrationCount()).To(BeEquivalentTo(4))
			})
		})

		Context("when internal unregistration messages is nil", func() {
			BeforeEach(func() {
				messagesToEmit.InternalRegistrationMessages = internalMessages
				messagesToEmit.InternalUnregistrationMessages = nil
			})

			It("adds the number of hostnames in each internal route message", func() {
				Expect(messagesToEmit.InternalRouteUnregistrationCount()).To(BeEquivalentTo(0))
			})
		})
	})
})
