package helpers

import (
	"net"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func Callback(listenHost string, handler http.HandlerFunc) (*httptest.Server, string) {
	externallyReachableListener, err := net.Listen("tcp", listenHost+":0")
	Expect(err).NotTo(HaveOccurred())

	server := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			handler(w, r)
		}),
	)

	server.Listener = externallyReachableListener

	server.Start()

	return server, externallyReachableListener.Addr().String()
}
