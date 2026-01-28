package healthcheck_test

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"code.cloudfoundry.org/healthcheck"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("HealthCheck", func() {
	itReturnsHealthCheckError := func(healthCheck func() error, code int, reason string) {
		err := healthCheck()
		Expect(err).To(HaveOccurred())
		Expect(err).To(BeAssignableToTypeOf(healthcheck.HealthCheckError{}))
		hErr := err.(healthcheck.HealthCheckError)
		Expect(hErr.Code).To(Equal(code))
		Expect(hErr.Message).To(ContainSubstring(reason))
	}

	var (
		server     *ghttp.Server
		serverAddr string

		ip string

		uri         string
		port        string
		timeout     time.Duration
		serverDelay time.Duration

		hc      healthcheck.HealthCheck
		handler func(http.ResponseWriter, *http.Request)
	)

	BeforeEach(func() {
		ip = getNonLoopbackIP()
		server = ghttp.NewUnstartedServer()

		listener, err := net.Listen("tcp", ip+":0")
		Expect(err).NotTo(HaveOccurred())

		timeout = 100 * time.Millisecond
		serverDelay = 0

		server.HTTPTestServer.Listener = listener
		serverAddr = listener.Addr().String()
		_, port, err = net.SplitHostPort(serverAddr)
		Expect(err).NotTo(HaveOccurred())

		handler = func(http.ResponseWriter, *http.Request) {
			time.Sleep(serverDelay)
		}

	})

	JustBeforeEach(func() {

		server.RouteToHandler("GET", "/api/_ping", handler)
		server.Start()

		hc = healthcheck.NewHealthCheck("tcp", uri, port, timeout)
	})

	AfterEach(func() {
		if server != nil {
			server.CloseClientConnections()
			server.Close()
		}
	})

	Describe("check interfaces", func() {
		It("succeeds when there are healthy interfaces", func() {
			interfaces, err := net.Interfaces()
			Expect(err).NotTo(HaveOccurred())

			err = hc.CheckInterfaces(interfaces)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the server is failing", func() {
			AfterEach(func() {
				server = nil
			})

			It("fails appropriately when there are unhealthy interfaces", func() {
				server.Close()

				interfaces, err := net.Interfaces()
				Expect(err).NotTo(HaveOccurred())

				err = hc.CheckInterfaces(interfaces)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(healthcheck.HealthCheckError{}))

				hErr := err.(healthcheck.HealthCheckError)
				// fails with different error codes on Linux (4) or OSX (64)
				// check to see it was not the NO interfaces error (3)
				Expect(hErr.Code).ToNot(Equal(3))
			})
		})

		It("fails appropriately when there are no interfaces", func() {
			err := hc.CheckInterfaces(nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(healthcheck.HealthCheckError{}))

			hErr := err.(healthcheck.HealthCheckError)
			Expect(hErr.Code).To(Equal(3))
			Expect(hErr.Message).To(ContainSubstring("failure to find suitable interface"))
		})
	})

	Describe("port healthcheck", func() {
		portHealthCheck := func() error {
			return hc.PortHealthCheck(ip)
		}

		BeforeEach(func() {
			uri = ""
		})

		Context("when the address is listening", func() {
			It("succeeds", func() {
				Expect(portHealthCheck()).To(Succeed())
			})
		})

		Context("when the address is not listening", func() {
			BeforeEach(func() {
				port = "-1"
			})

			It("returns healthcheck error with code 4 with an appropriate message", func() {
				errMsg := fmt.Sprintf("failed to make TCP connection to %s:%s: dial tcp: address %s: invalid port", ip, port, port)
				itReturnsHealthCheckError(portHealthCheck, 4, errMsg)
			})
		})

		Context("when the server is slow in responding", func() {
			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("TCP timeout tests are unreliable on Windows")
				}

				timeout = time.Nanosecond
			})

			It("returns healthcheck error with code 64 with an appropriate message", func() {
				errMsg := fmt.Sprintf("failed to make TCP connection to %s:%s: timed out after 0.00 seconds", ip, port)
				itReturnsHealthCheckError(portHealthCheck, 64, errMsg)
			})
		})
	})

	Describe("http healthcheck", func() {
		httpHealthCheck := func() error {
			return hc.HTTPHealthCheck(ip)
		}

		BeforeEach(func() {
			uri = "/api/_ping"
		})

		Context("when the healthcheck is properly invoked", func() {
			Context("when the address is listening", func() {
				It("succeeds", func() {
					Expect(httpHealthCheck()).To(Succeed())
				})
			})

			Context("when the address returns error http code", func() {
				JustBeforeEach(func() {
					server.RouteToHandler("GET", "/api/_ping", ghttp.RespondWith(500, ""))
				})

				It("returns healthcheck error with code 6 with an appropriate message", func() {
					errMsg := fmt.Sprintf(
						"failed to make HTTP request to '%s' on port %s: received status code 500 in",
						uri,
						port,
					)
					itReturnsHealthCheckError(httpHealthCheck, 6, errMsg)
				})
			})

			Context("when the address is not listening", func() {
				BeforeEach(func() {
					port = "100003"
				})

				It("returns healthcheck error with code 5 with an appropriate message", func() {
					errMsg := fmt.Sprintf(
						"failed to make HTTP request to '%s' on port %s: connection refused",
						uri,
						port,
					)
					itReturnsHealthCheckError(httpHealthCheck, 5, errMsg)
				})
			})

			Context("when the server is too slow to respond", func() {
				BeforeEach(func() {
					timeout = time.Nanosecond
					serverDelay = time.Second
				})

				It("returns healthcheck error with code 65 with an appropriate message", func() {
					errMsg := fmt.Sprintf(
						"failed to make HTTP request to '%s' on port %s: timed out after %.2f seconds",
						uri,
						port,
						timeout.Seconds(),
					)
					itReturnsHealthCheckError(httpHealthCheck, 65, errMsg)
				})
			})

			Context("with a tls-aware endpoint", func() {
				var request *http.Request
				BeforeEach(func() {
					uri = "/api/_ping"
					handler = func(resp http.ResponseWriter, req *http.Request) {
						request = req
						time.Sleep(serverDelay)
					}
				})

				It("should set the x-Forwarded-proto header", func() {
					err := hc.HTTPHealthCheck(ip)
					Expect(err).NotTo(HaveOccurred())
					Expect(request.Header.Get("X-Forwarded-Proto")).To(Equal("https"))
				})

				It("should set the User-Agent header", func() {
					err := hc.HTTPHealthCheck(ip)
					Expect(err).NotTo(HaveOccurred())
					Expect(request.Header.Get("User-Agent")).To(Equal("diego-healthcheck"))
				})

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
