package credhub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "code.cloudfoundry.org/credhub-cli/credhub"
)

type platformOptions struct {
	CredhubURI string `json:"credhub-uri"`
}

func InterpolateServiceRefs(maxConnectionAttempts int, retryDelay time.Duration) error {
	platformOptions, err := getPlatformOptions()
	if err != nil {
		return fmt.Errorf("unable to get platform options: %s", err)
	}

	if platformOptions.CredhubURI == "" {
		return nil
	}

	if os.Getenv("CREDHUB_SKIP_INTERPOLATION") != "" {
		return nil
	}
	if !strings.Contains(os.Getenv("VCAP_SERVICES"), `"credhub-ref"`) {
		return nil
	}

	ch, err := credhubClient(platformOptions.CredhubURI)
	if err != nil {
		return fmt.Errorf("unable to set up credhub client: %v", err)
	}

	var interpolatedServices string
	for attempt := 1; attempt <= maxConnectionAttempts; attempt++ {
		interpolatedServices, err = ch.InterpolateString(os.Getenv("VCAP_SERVICES"))
		if err == nil {
			break
		}
		fmt.Printf("Failed on attempt %v out of %v: Unable to interpolate credhub references: %v\n", attempt, maxConnectionAttempts, err)
		time.Sleep(retryDelay)
	}

	if err != nil {
		return fmt.Errorf("unable to interpolate credhub references: %v", err)
	}

	if err := os.Setenv("VCAP_SERVICES", interpolatedServices); err != nil {
		return fmt.Errorf("unable to update VCAP_SERVICES with interpolated credhub references: %v", err)
	}
	return nil
}

func getPlatformOptions() (platformOptions, error) {
	var platformOptions platformOptions
	platformOptionString := os.Getenv("VCAP_PLATFORM_OPTIONS")
	if platformOptionString == "" {
		return platformOptions, nil
	}

	err := json.Unmarshal([]byte(platformOptionString), &platformOptions)
	if err != nil {
		return platformOptions, err
	}

	return platformOptions, nil
}

func credhubClient(credhubURI string) (*api.CredHub, error) {
	instanceCertPath := os.Getenv("CF_INSTANCE_CERT")
	instanceKeyPath := os.Getenv("CF_INSTANCE_KEY")
	systemCertsPath := os.Getenv("CF_SYSTEM_CERT_PATH")

	if instanceCertPath == "" || instanceKeyPath == "" {
		return nil, fmt.Errorf("missing CF_INSTANCE_CERT and/or CF_INSTANCE_KEY")
	}
	if systemCertsPath == "" {
		return nil, fmt.Errorf("missing CF_SYSTEM_CERT_PATH")
	}

	caCerts := []string{}
	files, err := os.ReadDir(systemCertsPath)
	if err != nil {
		return nil, fmt.Errorf("can't read contents of system cert path: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".crt") {
			contents, err := os.ReadFile(filepath.Join(systemCertsPath, file.Name()))
			if err != nil {
				return nil, fmt.Errorf("can't read contents of cert in system cert path: %v", err)
			}
			caCerts = append(caCerts, string(contents))
		}
	}

	return api.New(
		credhubURI,
		api.ClientCert(instanceCertPath, instanceKeyPath),
		api.CaCerts(caCerts...),
	)
}
