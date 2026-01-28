package diegonats_test

import (
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager/v3/lagertest"
	. "code.cloudfoundry.org/route-emitter/diegonats"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Starting the NatsClientRunner process", func() {
	var natsClient NATSClient
	var natsClientRunner ifrit.Runner
	var natsClientProcess ifrit.Process

	BeforeEach(func() {
		natsAddress := fmt.Sprintf("127.0.0.1:%d", natsPort)
		natsClient = NewClient()
		natsClientRunner = NewClientRunner(natsAddress, "nats", "nats", lagertest.NewTestLogger("test"), natsClient)
	})

	AfterEach(func() {
		stopNATS()
		if natsClientProcess != nil {
			natsClientProcess.Signal(os.Interrupt)
			Eventually(natsClientProcess.Wait(), 5).Should(Receive())
		}
	})

	Describe("when NATS is up", func() {
		BeforeEach(func() {
			startNATS()
			natsClientProcess = ifrit.Invoke(natsClientRunner)
		})

		AfterEach(func() {
			stopNATS()
		})

		It("connects to NATS", func() {
			Expect(natsClient.Ping()).To(BeTrue())
		})

		It("disconnects when it receives a signal", func() {
			natsClientProcess.Signal(os.Interrupt)
			Eventually(natsClientProcess.Wait(), 5).Should(Receive())
		})

		It("exits with an error when nats connection is closed permanently", func() {
			errorChan := natsClientProcess.Wait()

			natsClient.Close()

			Eventually(errorChan).Should(Receive(Equal(errors.New("nats closed unexpectedly"))))
		})

		It("reconnects when nats server goes down and comes back up", func() {
			stopNATS()
			Eventually(natsClient.Ping).Should(BeFalse())

			startNATS()
			Eventually(natsClient.Ping, 60).Should(BeTrue())
		})
	})

	Describe("when NATS is not up", func() {
		BeforeEach(func() {
			natsClientProcess = ifrit.Invoke(natsClientRunner)
		})

		It("errors with a connection failure", func() {
			var err error
			Eventually(natsClientProcess.Wait(), 5).Should(Receive(&err))
		})
	})
})
