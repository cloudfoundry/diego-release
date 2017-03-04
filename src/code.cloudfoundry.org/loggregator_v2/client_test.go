package loggregator_v2_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/loggregator_v2"
	lfake "github.com/cloudfoundry/dropsonde/log_sender/fake"
	mfake "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/cloudfoundry/dropsonde/logs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testHandler struct{}

var _ = Describe("Client", func() {
	var (
		config    loggregator_v2.MetronConfig
		logger    *lagertest.TestLogger
		client    loggregator_v2.Client
		clientErr error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("loggregator-client")
	})

	JustBeforeEach(func() {
		client, clientErr = loggregator_v2.NewClient(logger, config)
	})

	Context("when v2 api is disabled", func() {
		var (
			logSender    *lfake.FakeLogSender
			metricSender *mfake.FakeMetricSender
		)

		BeforeEach(func() {
			logSender = &lfake.FakeLogSender{}
			metricSender = &mfake.FakeMetricSender{}
			config.UseV2API = false
			logs.Initialize(logSender)
			metrics.Initialize(metricSender, nil)
		})

		It("sends app logs", func() {
			client.SendAppLog("app-id", "message", "source-type", "source-instance")
			Expect(logSender.GetLogs()).To(ConsistOf(lfake.Log{AppId: "app-id", Message: "message",
				SourceType: "source-type", SourceInstance: "source-instance", MessageType: "OUT"}))
		})

		It("sends app error logs", func() {
			client.SendAppErrorLog("app-id", "message", "source-type", "source-instance")
			Expect(logSender.GetLogs()).To(ConsistOf(lfake.Log{AppId: "app-id", Message: "message",
				SourceType: "source-type", SourceInstance: "source-instance", MessageType: "ERR"}))
		})

		It("sends app metrics", func() {
			metric := events.ContainerMetric{
				ApplicationId: proto.String("app-id"),
			}
			client.SendAppMetrics(&metric)
			Expect(metricSender.Events()).To(ConsistOf(&metric))
		})
	})

	Context("when v2 api is enabled", func() {
		var (
			receivers   chan loggregator_v2.Ingress_SenderServer
			grpcRunner  *GrpcRunner
			grpcProcess ifrit.Process
		)

		Context("the cert or key path are invalid", func() {
			BeforeEach(func() {
				config.CertPath = "/some/invalid/path"
			})

			It("returns an error", func() {
				Expect(clientErr).To(HaveOccurred(), "client didn't return an error")
			})
		})

		Context("the ca cert path is invalid", func() {
			BeforeEach(func() {
				config.CACertPath = "/some/invalid/path"
			})

			It("returns an error", func() {
				Expect(clientErr).To(HaveOccurred(), "client didn't return an error")
			})
		})

		Context("the ca cert is invalid", func() {
			BeforeEach(func() {
				config.CACertPath = "fixtures/invalid-ca.crt"
			})

			It("returns an error", func() {
				Expect(clientErr).To(HaveOccurred(), "client didn't return an error")
			})
		})

		Context("cannot connecto to the server", func() {
			BeforeEach(func() {
				config.APIPort = 1234
			})

			It("returns an error", func() {
				Expect(client.SendAppLog("app-id", "message", "source-type", "source-instance")).NotTo(Succeed())
			})
		})

		BeforeEach(func() {
			var err error
			grpcRunner, err = NewGRPCRunner("fixtures/metron.crt", "fixtures/metron.key", "fixtures/CA.crt")
			Expect(err).NotTo(HaveOccurred())
			grpcProcess = ginkgomon.Invoke(grpcRunner)

			config = loggregator_v2.MetronConfig{
				UseV2API:   true,
				APIPort:    grpcRunner.Port(),
				CACertPath: "fixtures/CA.crt",
				CertPath:   "fixtures/client.crt",
				KeyPath:    "fixtures/client.key",
			}
			receivers = grpcRunner.Receivers()
		})

		AfterEach(func() {
			ginkgomon.Interrupt(grpcProcess)
		})

		It("does not return an error", func() {
			Expect(clientErr).NotTo(HaveOccurred())
		})

		It("sends app logs", func() {
			Consistently(func() error {
				return client.SendAppLog("app-id", "message", "source-type", "source-instance")
			}).Should(Succeed())
			var recv loggregator_v2.Ingress_SenderServer
			Eventually(receivers).Should(Receive(&recv))
			env, err := recv.Recv()
			Expect(err).NotTo(HaveOccurred())
			ts := time.Unix(0, env.GetTimestamp())
			Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
			log := env.GetLog()
			Expect(log).NotTo(BeNil())
			Expect(log.GetPayload()).To(Equal([]byte("message")))
			Expect(log.Type).To(Equal(loggregator_v2.Log_OUT))
		})

		It("sends app error logs", func() {
			Consistently(func() error {
				return client.SendAppErrorLog("app-id", "message", "source-type", "source-instance")
			}).Should(Succeed())
			var recv loggregator_v2.Ingress_SenderServer
			Eventually(receivers).Should(Receive(&recv))
			env, err := recv.Recv()
			Expect(err).NotTo(HaveOccurred())
			ts := time.Unix(0, env.GetTimestamp())
			Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
			log := env.GetLog()
			Expect(log).NotTo(BeNil())
			Expect(log.GetPayload()).To(Equal([]byte("message")))
			Expect(log.Type).To(Equal(loggregator_v2.Log_ERR))
		})

		It("sends app metrics", func() {
			metric := events.ContainerMetric{
				ApplicationId:    proto.String("app-id"),
				CpuPercentage:    proto.Float64(10.0),
				MemoryBytes:      proto.Uint64(10),
				DiskBytes:        proto.Uint64(10),
				MemoryBytesQuota: proto.Uint64(20),
				DiskBytesQuota:   proto.Uint64(20),
				InstanceIndex:    proto.Int32(5),
			}
			Consistently(func() error {
				return client.SendAppMetrics(&metric)
			}).Should(Succeed())
			var recv loggregator_v2.Ingress_SenderServer
			Eventually(receivers).Should(Receive(&recv))
			env, err := recv.Recv()
			Expect(err).NotTo(HaveOccurred())
			ts := time.Unix(0, env.GetTimestamp())
			Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
			metrics := env.GetGauge()
			Expect(metrics).NotTo(BeNil())
			Expect(env.GetSourceId()).To(Equal("app-id"))
			Expect(metrics.GetMetrics()).To(HaveLen(6))
			Expect(metrics.GetMetrics()["instance_index"].GetValue()).To(Equal(5.0))
			Expect(metrics.GetMetrics()["cpu"].GetValue()).To(Equal(10.0))
			Expect(metrics.GetMetrics()["memory"].GetValue()).To(Equal(10.0))
			Expect(metrics.GetMetrics()["disk"].GetValue()).To(Equal(10.0))
			Expect(metrics.GetMetrics()["memory_quota"].GetValue()).To(Equal(20.0))
			Expect(metrics.GetMetrics()["disk_quota"].GetValue()).To(Equal(20.0))
		})

		Context("when the server goes away and comes back", func() {
			JustBeforeEach(func() {
				Expect(client.SendAppErrorLog("app-id", "message", "source-type", "source-instance")).To(Succeed())
				ginkgomon.Interrupt(grpcProcess)

				// wait for the client to detect the error
				Eventually(client.SendAppErrorLog("app-id", "message", "source-type", "source-instance")).ShouldNot(Succeed())
				grpcProcess = ginkgomon.Invoke(grpcRunner)
			})

			It("should reconnect", func() {
				Eventually(func() error {
					return client.SendAppErrorLog("app-id", "message", "source-type", "source-instance")
				}).Should(Succeed())
			})
		})
	})
})
