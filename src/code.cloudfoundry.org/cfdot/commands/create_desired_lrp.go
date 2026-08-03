package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

var createDesiredLRPCmd = &cobra.Command{
	Use:   "create-desired-lrp (SPEC|@FILE)",
	Short: "Create a desired LRP",
	Long:  "Create a desired LRP from the given spec. Spec can either be json encoded desired-lrp, e.g. '{\"process_guid\":\"some-guid\"}' or a file containing json encoded desired-lrp, e.g. @/path/to/spec/file",
	RunE:  createDesiredLRP,
}

func init() {
	AddBBSAndTimeoutFlags(createDesiredLRPCmd)
	RootCmd.AddCommand(createDesiredLRPCmd)
}

func createDesiredLRP(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return NewCFDotValidationError(cmd, fmt.Errorf("missing spec argument"))
	}

	spec, err := ValidateCreateDesiredLRPArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = CreateDesiredLRP(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, spec)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateCreateDesiredLRPArguments(args []string) ([]byte, error) {
	var desiredLRP *models.DesiredLRP
	var err error
	var spec []byte
	argValue := args[0]
	if strings.HasPrefix(argValue, "@") {
		_, err := os.Stat(argValue[1:])
		if err != nil {
			return nil, err
		}
		spec, err = os.ReadFile(argValue[1:])
		if err != nil {
			return nil, err
		}

	} else {
		spec = []byte(argValue)
	}
	err = json.Unmarshal([]byte(spec), &desiredLRP)
	if err != nil {
		return nil, fmt.Errorf("Invalid JSON: %s", err.Error())
	}
	return spec, nil
}

func CreateDesiredLRP(stdout, stderr io.Writer, bbsClient bbs.Client, spec []byte) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("create-desired-lrp"), traceID)

	var desiredLRP *models.DesiredLRP
	err := json.Unmarshal(spec, &desiredLRP)
	if err != nil {
		return err
	}

	err = bbsClient.DesireLRP(logger, traceID, desiredLRP)
	if err != nil {
		return err
	}

	return nil
}
