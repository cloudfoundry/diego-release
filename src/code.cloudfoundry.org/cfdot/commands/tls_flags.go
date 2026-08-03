package commands

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"code.cloudfoundry.org/cfdot/commands/helpers"

	"github.com/spf13/cobra"
)

// errors
var (
	errMissingCACertFile            = errors.New("--caCertFile must be specified if using HTTPS and --skipCertVerify is not set")
	errMissingClientCertAndKeyFiles = errors.New("--clientCertFile and --clientKeyFile must both be specified for TLS connections.")
)

var (
	Config helpers.TLSConfig
)

func AddTLSFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&Config.SkipCertVerify, "skipCertVerify", false, "when set to true, skips all SSL/TLS certificate verification [environment variable equivalent: SKIP_CERT_VERIFY]")
	cmd.Flags().StringVar(&Config.CACertFile, "caCertFile", "", "path the Certificate Authority (CA) file to use when verifying TLS keypairs [environment variable equivalent: CA_CERT_FILE]")
	cmd.Flags().StringVar(&Config.CertFile, "clientCertFile", "", "path to the TLS client certificate to use during mutual-auth TLS [environment variable equivalent: CLIENT_CERT_FILE]")
	cmd.Flags().StringVar(&Config.KeyFile, "clientKeyFile", "", "path to the TLS client private key file to use during mutual-auth TLS [environment variable equivalent: CLIENT_KEY_FILE]")

	cmd.PreRunE = tlsPreHook
}

func tlsPreHook(cmd *cobra.Command, args []string) error {
	var err, returnErr error

	// Only look at the environment variable if the flag has not been set.
	if !cmd.Flags().Lookup("skipCertVerify").Changed && os.Getenv("SKIP_CERT_VERIFY") != "" {
		Config.SkipCertVerify, err = strconv.ParseBool(os.Getenv("SKIP_CERT_VERIFY"))
		if err != nil {
			returnErr = NewCFDotValidationError(
				cmd,
				fmt.Errorf(
					"The value '%s' is not a valid value for SKIP_CERT_VERIFY. Please specify one of the following valid boolean values: 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False",
					os.Getenv("SKIP_CERT_VERIFY")),
			)
			return returnErr
		}
	}

	if Config.CACertFile == "" {
		Config.CACertFile = os.Getenv("CA_CERT_FILE")
	}

	if Config.CertFile == "" {
		Config.CertFile = os.Getenv("CLIENT_CERT_FILE")
	}

	if Config.KeyFile == "" {
		Config.KeyFile = os.Getenv("CLIENT_KEY_FILE")
	}

	if !Config.SkipCertVerify {
		if Config.CACertFile == "" {
			returnErr = NewCFDotValidationError(cmd, errMissingCACertFile)
			return returnErr
		}

		err := validateReadableFile(cmd, Config.CACertFile, "CA cert")
		if err != nil {
			return err
		}
	}

	if (Config.KeyFile == "") || (Config.CertFile == "") {
		returnErr = NewCFDotValidationError(cmd, errMissingClientCertAndKeyFiles)
		return returnErr
	}

	if Config.KeyFile != "" {
		err := validateReadableFile(cmd, Config.KeyFile, "key")

		if err != nil {
			return err
		}
	}

	if Config.CertFile != "" {
		err := validateReadableFile(cmd, Config.CertFile, "cert")

		if err != nil {
			return err
		}
	}

	return nil
}
