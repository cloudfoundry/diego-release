package commands

import (
	"errors"
	"fmt"
	"os"

	"net/url"

	"github.com/spf13/cobra"
)

var (
	bbsUrl string
)

// errors
var (
	errMissingBBSUrl = errors.New("BBS URL not set. Please specify one with the '--bbsURL' flag or the 'BBS_URL' environment variable.")
)

func AddBBSFlags(cmd *cobra.Command) {
	AddTLSFlags(cmd)
	cmd.Flags().StringVar(&bbsUrl, "bbsURL", "", "URL of BBS server to target [environment variable equivalent: BBS_URL]")
	cmd.PreRunE = BBSPrehook
}

func BBSPrehook(cmd *cobra.Command, args []string) error {
	if err := setBBSFlags(cmd, args); err != nil {
		return err
	}
	return tlsPreHook(cmd, args)
}

func setBBSFlags(cmd *cobra.Command, args []string) error {
	var err, returnErr error

	if bbsUrl == "" {
		bbsUrl = os.Getenv("BBS_URL")
	}

	if bbsUrl == "" {
		returnErr = NewCFDotValidationError(cmd, errMissingBBSUrl)
		return returnErr
	}

	Config.BBSUrl = bbsUrl

	var parsedURL *url.URL
	if parsedURL, err = url.Parse(Config.BBSUrl); err != nil {
		returnErr = NewCFDotValidationError(
			cmd,
			fmt.Errorf(
				"The value '%s' is not a valid BBS URL. Please specify one with the '--bbsURL' flag or the 'BBS_URL' environment variable.",
				Config.BBSUrl),
		)
		return returnErr
	}

	if parsedURL.Scheme != "https" {
		returnErr = NewCFDotValidationError(
			cmd,
			fmt.Errorf(
				"The URL '%s' does not have an 'https' scheme. Please "+
					"specify one with the '--bbsURL' flag or the 'BBS_URL' environment "+
					"variable.", Config.BBSUrl),
		)
		return returnErr
	}

	return nil
}

func validateReadableFile(cmd *cobra.Command, filename, filetype string) error {
	file, err := os.Open(filename)
	if err != nil {

		return NewCFDotValidationError(
			cmd,
			fmt.Errorf("%s file '%s' doesn't exist or is not readable: %s", filetype, filename, err.Error()),
		)
	}
	defer file.Close()

	return nil
}
