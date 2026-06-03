//go:build !windows

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

const (
	// Envoy-compatible ports that fake-proxy listens on
	HttpPort        = ":61001" // HTTP endpoint for exit commands
	HttpsPort       = ":61443" // HTTPS listener (unused but envoy-compatible)
	AdminPort       = ":61002" // Admin port (unused but envoy-compatible)
	MetricsPort     = ":61003" // Metrics port (unused but envoy-compatible)
	HealthCheckPort = ":61004" // Health check port (unused but envoy-compatible)

	ExitDelay       = 50 * time.Millisecond // Allow HTTP response to be sent before exit
	SigabrtExitCode = 134
	NaturalExitCode = 42 // Exit code when server shuts down naturally
)

func listen(port string) {
	l, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("failed to listen on %s: %v\n", port, err)
		return
	}
	for {
		conn, err := l.Accept()
		if err == nil {
			conn.Close()
		}
	}
}

func main() {
	// Fake-proxy accepts any command line arguments (like envoy does) but ignores them
	// This allows it to be a drop-in replacement for envoy

	go listen(HttpsPort)
	go listen(AdminPort)
	go listen(MetricsPort)
	go listen(HealthCheckPort)

	server := &http.Server{Addr: HttpPort}

	http.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		codeStr := r.URL.Query().Get("code")
		exitCode, _ := strconv.Atoi(codeStr)

		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		go func() {
			time.Sleep(ExitDelay)
			if exitCode == SigabrtExitCode {
				syscall.Kill(syscall.Getpid(), syscall.SIGABRT)
			} else {
				os.Exit(exitCode)
			}
		}()
	})

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("http server failed: %v\n", err)
	}
	os.Exit(NaturalExitCode)
}
