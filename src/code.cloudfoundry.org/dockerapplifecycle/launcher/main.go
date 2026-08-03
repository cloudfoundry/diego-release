package main

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/credhub-cli/credhub"

	"code.cloudfoundry.org/buildpackapplifecycle/databaseuri"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
)

type PlatformOptions struct {
	CredhubURI string `json:"credhub-uri"`
}

const (
	PlatformOptionsEnvVar  = "VCAP_PLATFORM_OPTIONS"
	CFSystemCertPathEnvVar = "CF_SYSTEM_CERT_PATH"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <ignored> <start command> <metadata>\n", os.Args[0])
		os.Exit(1)
	}

	// os.Args[1] is ignored, but left for backwards compatibility
	startCommand := os.Args[2]
	metadata := os.Args[3]

	platformOptions, err := platformOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid platform options\n")
		os.Exit(3)
	}

	interpolateVCAPServices(platformOptions)
	setDatabaseURL()

	vcapAppEnv := map[string]interface{}{}

	err = json.Unmarshal([]byte(os.Getenv("VCAP_APPLICATION")), &vcapAppEnv)
	if err == nil {
		vcapAppEnv["host"] = "0.0.0.0"

		vcapAppEnv["instance_id"] = os.Getenv("INSTANCE_GUID")

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err == nil {
			vcapAppEnv["port"] = port
		}

		index, err := strconv.Atoi(os.Getenv("INSTANCE_INDEX"))
		if err == nil {
			vcapAppEnv["instance_index"] = index
		}

		mungedAppEnv, err := json.Marshal(vcapAppEnv)
		if err == nil {
			err := os.Setenv("VCAP_APPLICATION", string(mungedAppEnv))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Couldn't set VCAP_APPLICATION env var: %s\n", err)
				os.Exit(1)
			}
		}
	}

	var executionMetadata protocol.ExecutionMetadata
	err = json.Unmarshal([]byte(metadata), &executionMetadata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid metadata - %s\n", err)
		os.Exit(1)
	}

	workdir := "/"
	if executionMetadata.Workdir != "" {
		workdir = executionMetadata.Workdir
	}
	err = os.Chdir(workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't change directories to %s: %s\n", workdir, err)
		os.Exit(1)
	}

	if len(executionMetadata.Entrypoint) == 0 && len(executionMetadata.Cmd) == 0 && startCommand == "" {
		fmt.Fprintf(os.Stderr, "No start command found or specified\n")
		os.Exit(1)
	}

	// https://docs.docker.com/reference/builder/#entrypoint and
	// https://docs.docker.com/reference/builder/#cmd dictate how Entrypoint
	// and Cmd are treated by docker; we follow these rules here
	var argv []string
	if startCommand != "" {
		argv = []string{"/bin/sh", "-c", startCommand}
	} else {
		argv = append(executionMetadata.Entrypoint, executionMetadata.Cmd...)
		argv[0], err = exec.LookPath(argv[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve path: %s\n", err)
			os.Exit(1)
		}
	}

	runtime.GOMAXPROCS(1)
	err = syscall.Exec(argv[0], argv, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run: %s\n", err)
		os.Exit(1)
	}
}

func setDatabaseURL() {
	vcapServices := os.Getenv("VCAP_SERVICES")
	if vcapServices == "" {
		return
	}

	databaseURI := databaseuri.New()
	creds, err := databaseURI.Credentials([]byte(vcapServices))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse vcap services: %s\n", err)
		return
	}
	uri := databaseURI.Uri(creds)
	if uri == "" {
		return
	}
	if err := os.Setenv("DATABASE_URL", uri); err != nil {
		panic(err)
	}
}

func interpolateVCAPServices(platformOptions *PlatformOptions) {
	if platformOptions == nil || platformOptions.CredhubURI == "" {
		return
	}

	vcapServices := os.Getenv("VCAP_SERVICES")
	if !strings.Contains(vcapServices, `"credhub-ref"`) {
		return
	}

	certPath := os.Getenv("CF_INSTANCE_CERT")
	keyPath := os.Getenv("CF_INSTANCE_KEY")
	rootCAs := rootCAs()

	credhubURI := platformOptions.CredhubURI
	_, err := url.ParseRequestURI(credhubURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid CredHub URI: '%s'\n", credhubURI)
		os.Exit(4)
	}

	if certPath == "" || keyPath == "" {
		fmt.Fprintf(os.Stderr, "Unable to load instance identity credentials; CF_INSTANCE_CERT: '%s', CF_INSTANCE_KEY: '%s'\n", certPath, keyPath)
		os.Exit(4)
	}

	ch, err := credhub.New(credhubURI, credhub.ClientCert(certPath, keyPath), credhub.CaCerts(rootCAs...))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create a credhub client: %s\n", err)
		os.Exit(4)
	}

	interpolatedVcapServices, err := ch.InterpolateString(vcapServices)

	if err != nil {
		fmt.Fprint(os.Stderr, formatCredHubErrorMessage(err))
		os.Exit(4)
	}

	err = os.Setenv("VCAP_SERVICES", interpolatedVcapServices)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot set environment variable: %s\n", err)
		os.Exit(4)
	}
}

func rootCAs() []string {
	certsPath := os.Getenv(CFSystemCertPathEnvVar)
	pattern := path.Join(certsPath, "*.crt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to locate system certs: %s\n", err)
		os.Exit(4)
	}
	certs := []string{}
	for _, m := range matches {
		content, err := os.ReadFile(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to read system certs: %s\n", err)
			os.Exit(4)
		}
		certs = append(certs, string(content))
	}
	return certs
}

func platformOptions() (*PlatformOptions, error) {
	jsonPlatformOptions := os.Getenv(PlatformOptionsEnvVar)
	if jsonPlatformOptions == "" {
		return nil, nil
	}
	platformOptions := PlatformOptions{}
	err := json.Unmarshal([]byte(jsonPlatformOptions), &platformOptions)
	if err != nil {
		return nil, err
	}

	return &platformOptions, nil
}

func formatCredHubErrorMessage(err error) string {
	var unknownAuthErr *x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthErr) {
		return fmt.Sprintf("Unable to verify CredHub server: %s", unknownAuthErr)
	}

	if typeErr, ok := err.(*url.Error); ok {
		return formatCredHubErrorMessage(typeErr.Err)
	}

	return fmt.Sprintf("Unable to interpolate credhub references: %s", err)
}
