package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

const (
	DefaultPort     = "8080"
	ExitDelay       = 50 * time.Millisecond // Allow HTTP response to be sent before exit
	SigabrtExitCode = 134
	NaturalExitCode = 42 // Exit code when server shuts down naturally
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	server := &http.Server{}

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server.Addr = ":" + port
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("http server failed: %v\n", err)
	}
	os.Exit(NaturalExitCode)
}
