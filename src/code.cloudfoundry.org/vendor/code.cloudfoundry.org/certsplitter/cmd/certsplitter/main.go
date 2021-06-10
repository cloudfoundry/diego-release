package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	certFileFmt = "trusted-ca-%d.crt"
)

type Certs struct {
	TrustedCACertificates []string `json:"trusted_ca_certificates"`
}

func (c *Certs) fixCerts() {
	allCerts := []string{}
	for _, certs := range c.TrustedCACertificates {
		allCerts = append(allCerts, splitCerts(certs)...)
	}
	c.TrustedCACertificates = allCerts
}

func splitCerts(certs string) []string {
	chunks := strings.SplitAfter(certs, "-----END CERTIFICATE-----")
	result := []string{}
	for _, chunk := range chunks {
		start := strings.Index(chunk, "-----BEGIN CERTIFICATE-----")
		if start == -1 {
			continue
		}

		cert := chunk[start:len(chunk)] + "\n"
		result = append(result, cert)
	}
	return result
}

type flagError string

func (e flagError) Error() string {
	return string(e)
}

func (flagError) usageError() {}

type usageError interface {
	usageError()
}

func main() {
	log.SetOutput(os.Stderr)

	must(parseFlags())
	must(writeCerts())
}

func parseFlags() error {
	flag.Parse()

	if len(flag.Args()) < 1 {
		return flagError("must provide path to trusted certificates file")
	}

	if len(flag.Args()) < 2 {
		return flagError("must provide path to destination folder")
	}

	return nil
}

func writeCerts() error {
	trustedCertsPath := flag.Args()[0]
	data, err := ioutil.ReadFile(trustedCertsPath)
	if err != nil {
		return err
	}

	certs := Certs{}
	if strings.Contains(trustedCertsPath, ".json") {
		err := json.Unmarshal(data, &certs)
		if err != nil {
			return err
		}
	} else {
		certs = Certs{TrustedCACertificates: []string{string(data)}}
	}

	certs.fixCerts()

	outputDir := flag.Args()[1]
	for i, c := range certs.TrustedCACertificates {
		filename := path.Join(outputDir, fmt.Sprintf(certFileFmt, i+1))
		err = ioutil.WriteFile(filename, []byte(c), 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func printUsage() {
	log.Println()
	log.Printf("Usage: %s TRUSTED_CERTS_FILE DESTINATION_DIRECTORY\n", filepath.Base(os.Args[0]))
}

func must(err error) {
	if err != nil {
		log.Println(err)
		if _, ok := err.(usageError); ok {
			printUsage()
		}
		os.Exit(1)
	}
}
