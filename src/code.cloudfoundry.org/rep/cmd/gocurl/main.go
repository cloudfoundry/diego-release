package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/tlsconfig"
)

var (
	tlsCACertPath = flag.String("cacert", "", "CA certificate for API server")
	tlsCertPath   = flag.String("cert", "", "TLS certificate for API server")
	tlsKeyPath    = flag.String("key", "", "TLS private key for API server")
	httpMethod    = flag.String("X", "GET", "HTTP method")
	maxTime       = flag.Duration("max-time", 0, "Maximum time that you allow the whole operation to take.")
	customHeader  = flag.String("H", "", "Custom Header")
)

func main() {
	flag.Parse()
	if len(flag.Args()) != 1 {
		log.Println("Must provide a URL to contact")
		os.Exit(1)
	}

	var tlsConfig *tls.Config
	var err error
	if *tlsKeyPath != "" || *tlsCertPath != "" || *tlsCACertPath != "" {
		tlsConfig, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(*tlsCertPath, *tlsKeyPath),
		).Client(tlsconfig.WithAuthorityFromFile(*tlsCACertPath))
		if err != nil {
			log.Printf("TLS config mismatch: %s\n", err)
			os.Exit(1)
		}
	}

	url := flag.Arg(0)
	var req *http.Request
	switch *httpMethod {
	case "GET":
		req, err = http.NewRequest("GET", url, nil)
	case "POST":
		req, err = http.NewRequest("POST", url, bytes.NewReader([]byte{}))
	default:
		err = errors.New("Currently only supports GET and POST methods")
	}
	if err != nil {
		log.Printf("Failed to generate request: %s\n", err)
		os.Exit(1)
	}

	if *customHeader != "" {
		headerNameAndValue := strings.Split(*customHeader, "=")
		req.Header.Add(headerNameAndValue[0], headerNameAndValue[1])
	}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: *maxTime,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to contact %s: %s\n", url, err)
		os.Exit(1)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Error talking to %s: status code %d\n", url, resp.StatusCode)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s", string(body))
	os.Exit(0)
}
