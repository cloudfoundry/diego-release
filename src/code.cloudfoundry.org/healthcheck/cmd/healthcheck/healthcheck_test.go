package main_test

import (
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("HealthCheck", func() {
	var (
		server     *ghttp.Server
		serverAddr string
		port       string
		args       []string
	)

	itExitsWithCode := func(healthCheck func() *gexec.Session, code int, reason string) {
		It("exits with code "+strconv.Itoa(code)+" and logs reason", func() {
			session := healthCheck()
			Eventually(session).Should(gexec.Exit(code))
			Expect(session.Err).To(gbytes.Say(reason))
		})
	}

	itPasses := func(healthCheck func() *gexec.Session) {
		It("exits with code 0 and logs reason", func() {
			session := healthCheck()
			Eventually(session).Should(gexec.Exit(0))
		})
	}

	BeforeEach(func() {
		args = nil

		ip := getNonLoopbackIP()
		server = ghttp.NewUnstartedServer()
		listener, err := net.Listen("tcp", ip+":0")
		Expect(err).NotTo(HaveOccurred())

		server.HTTPTestServer.Listener = listener
		serverAddr = listener.Addr().String()
		server.Start()

		_, port, err = net.SplitHostPort(serverAddr)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("fails when parsing flags", func() {
		It("exits with code 2", func() {
			session, _ := gexec.Start(exec.Command(healthCheck, "-invalid_flag"), GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit(2))
		})
	})

	portHealthCheck := func() *gexec.Session {
		return createPortHealthCheck(args, port)
	}

	httpHealthCheck := func() *gexec.Session {
		return createHTTPHealthCheck(args, port)
	}

	Describe("in startup mode", func() {
		var (
			session    *gexec.Session
			statusCode int64
		)

		BeforeEach(func() {
			statusCode = http.StatusInternalServerError
			server.RouteToHandler("GET", "/api/_ping", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				statusCode := atomic.LoadInt64(&statusCode)
				resp.WriteHeader(int(statusCode))
			}))

			args = []string{"-startup-interval=1s", "-startup-timeout=2s"}
		})

		AfterEach(func() {
			session.Kill()
		})

		It("does not exit until the http server is started", func() {
			session = httpHealthCheck()
			Consistently(session).ShouldNot(gexec.Exit())
			atomic.StoreInt64(&statusCode, http.StatusOK)
			Eventually(session, 2*time.Second).Should(gexec.Exit(0))
		})

		Context("when the timeout is large", func() {
			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("skipping since SIGTERM probably doesn't work on Windows")
				}
				args = []string{"-startup-interval=1s", "-startup-timeout=60s"}
			})

			It("exits with healthcheck error when signalled", func() {
				session = httpHealthCheck()
				Eventually(server.ReceivedRequests, 3*time.Second).Should(HaveLen(2))
				session.Signal(syscall.SIGTERM)
				Eventually(session).Should(gexec.Exit())
			})
		})

		It("runs a healthcheck every startup-interval", func() {
			session = httpHealthCheck()
			start := time.Now()
			Eventually(server.ReceivedRequests, 3*time.Second).Should(HaveLen(2))
			end := time.Now()
			Expect(end.Sub(start)).To(BeNumerically("~", 1*time.Second, 100*time.Millisecond))
		})

		It("exits with healthcheck error after startup-timeout has been reached", func() {
			session = httpHealthCheck()
			Eventually(server.ReceivedRequests).ShouldNot(BeEmpty())
			Eventually(session, 3*time.Second).Should(gexec.Exit(6))
		})

		Context("when startup timeout is set to 0", func() {
			BeforeEach(func() {
				args = []string{"-startup-interval=1s", "-startup-timeout=0s"}
			})

			It("does not timeout", func() {
				session = httpHealthCheck()
				Consistently(session).ShouldNot(gexec.Exit())
			})
		})

		Context("with low startup interval", func() {
			BeforeEach(func() {
				server.HTTPTestServer.Close()
				args = []string{"-startup-interval=10ms"}
			})

			It("continues to retry until the server is started", func() {
				session = portHealthCheck()
				Consistently(session).ShouldNot(gexec.Exit())
				listener, err := net.Listen("tcp", ":"+port)
				Expect(err).NotTo(HaveOccurred())
				defer listener.Close()
				Eventually(session).Should(gexec.Exit())
			})
		})
	})

	Describe("in until-ready mode", func() {
		var (
			session    *gexec.Session
			statusCode int64
		)

		BeforeEach(func() {
			statusCode = http.StatusInternalServerError
			server.RouteToHandler("GET", "/api/_ping", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				statusCode := atomic.LoadInt64(&statusCode)
				resp.WriteHeader(int(statusCode))
			}))

			args = []string{"-until-ready-interval=1s"}
		})

		AfterEach(func() {
			session.Kill()
		})

		It("does not exit until the http server is started", func() {
			session = httpHealthCheck()
			Consistently(session).ShouldNot(gexec.Exit())

			atomic.StoreInt64(&statusCode, http.StatusOK)
			Eventually(session, 2*time.Second).Should(gexec.Exit(0))
		})

		It("runs a healthcheck every until-ready-interval", func() {
			session = httpHealthCheck()
			start := time.Now()
			Eventually(server.ReceivedRequests, 3*time.Second).Should(HaveLen(2))
			end := time.Now()
			Expect(end.Sub(start)).To(BeNumerically("~", 1*time.Second, 100*time.Millisecond))
		})

		Context("with low startup interval", func() {
			BeforeEach(func() {
				server.HTTPTestServer.Close()
				args = []string{"-until-ready-interval=10ms"}
			})

			It("continues to retry until the server is started", func() {
				session = portHealthCheck()
				Consistently(session).ShouldNot(gexec.Exit())
				listener, err := net.Listen("tcp", ":"+port)
				Expect(err).NotTo(HaveOccurred())
				defer listener.Close()
				Eventually(session).Should(gexec.Exit())
			})
		})
	})

	Describe("in liveness mode", func() {
		var (
			session    *gexec.Session
			statusCode int64
		)

		BeforeEach(func() {
			statusCode = http.StatusOK
			server.RouteToHandler("GET", "/api/_ping", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				statusCode := atomic.LoadInt64(&statusCode)
				resp.WriteHeader(int(statusCode))
			}))

			args = []string{"-liveness-interval=1s"}
		})

		AfterEach(func() {
			session.Kill()
		})

		It("does not exit until the http server is down", func() {
			session = httpHealthCheck()
			Consistently(session).ShouldNot(gexec.Exit())
			atomic.StoreInt64(&statusCode, http.StatusInternalServerError)
			Eventually(session, 2*time.Second).Should(gexec.Exit(6))
			Expect(session.Err).NotTo(gbytes.Say("healthcheck failed"))
			Expect(session.Err).To(gbytes.Say("Liveness check unsuccessful"))
			Expect(session.Err).To(gbytes.Say("received status code 500 in"))
		})

		It("runs a healthcheck every liveness-interval", func() {
			session = httpHealthCheck()
			start := time.Now()
			Eventually(server.ReceivedRequests, 3*time.Second).Should(HaveLen(2))
			end := time.Now()
			Expect(end.Sub(start)).To(BeNumerically("~", 1*time.Second, 100*time.Millisecond))
		})
	})

	Describe("in readiness mode", func() {
		var (
			session    *gexec.Session
			statusCode int64
		)

		BeforeEach(func() {
			statusCode = http.StatusOK
			server.RouteToHandler("GET", "/api/_ping", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				statusCode := atomic.LoadInt64(&statusCode)
				resp.WriteHeader(int(statusCode))
			}))

			args = []string{"-readiness-interval=1s"}
		})

		AfterEach(func() {
			session.Kill()
		})

		It("does not exit until the http server is down", func() {
			session = httpHealthCheck()
			Consistently(session).ShouldNot(gexec.Exit())
			atomic.StoreInt64(&statusCode, http.StatusInternalServerError)
			Eventually(session, 2*time.Second).Should(gexec.Exit(6))
			Expect(session.Err).NotTo(gbytes.Say("healthcheck failed"))
			Expect(session.Err).To(gbytes.Say("Readiness check unsuccessful"))
			Expect(session.Err).To(gbytes.Say("received status code 500 in"))
		})

		It("runs a healthcheck every readiness-interval", func() {
			session = httpHealthCheck()
			start := time.Now()
			Eventually(server.ReceivedRequests, 3*time.Second).Should(HaveLen(2))
			end := time.Now()
			Expect(end.Sub(start)).To(BeNumerically("~", 1*time.Second, 100*time.Millisecond))
		})
	})

	Describe("port healthcheck", func() {
		Context("when the address is listening", func() {
			itPasses(portHealthCheck)
		})

		Context("when the address is not listening", func() {
			BeforeEach(func() {
				port = "-1"
			})

			itExitsWithCode(portHealthCheck, 4, "dial tcp: address -1: invalid port")
		})
	})

	Describe("http healthcheck", func() {
		Context("when the healthcheck is properly invoked", func() {
			BeforeEach(func() {
				server.RouteToHandler("GET", "/api/_ping", ghttp.VerifyRequest("GET", "/api/_ping"))
			})

			Context("when the address is listening", func() {
				itPasses(httpHealthCheck)
			})

			Context("when the address returns error http code", func() {
				BeforeEach(func() {
					server.RouteToHandler("GET", "/api/_ping", ghttp.RespondWith(http.StatusInternalServerError, ""))
				})

				itExitsWithCode(httpHealthCheck, 6, "received status code 500 in")
			})
		})
	})
})

func getNonLoopbackIP() string {
	interfaces, err := net.Interfaces()
	Expect(err).NotTo(HaveOccurred())
	for _, intf := range interfaces {
		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}
	Fail("no non-loopback address found")
	panic("non-reachable")
}
