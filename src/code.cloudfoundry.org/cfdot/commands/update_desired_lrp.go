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

var updateDesiredLRPCmd = &cobra.Command{
	Use:   "update-desired-lrp process-guid (SPEC|@FILE)",
	Short: "Update a desired LRP",
	Long:  "Update a desired LRP for a process-guid with the given spec. Spec can either be json encoded update to a desired-lrp, e.g. '{\"instances\":\"4\"}', or a file containing json encoded update to a desired-lrp, e.g. @/path/to/spec/file",
	RunE:  updateDesiredLRP,
}

func init() {
	AddBBSAndTimeoutFlags(updateDesiredLRPCmd)
	RootCmd.AddCommand(updateDesiredLRPCmd)
}

func updateDesiredLRP(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return NewCFDotValidationError(cmd, fmt.Errorf("Missing arguments"))
	}

	processGuid, spec, err := ValidateUpdateDesiredLRPArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = UpdateDesiredLRP(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, processGuid, spec)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateUpdateDesiredLRPArguments(args []string) (string, []byte, error) {
	var desiredLRP *models.DesiredLRPUpdate
	var err error
	var spec []byte
	processGuid := args[0]
	argValue := args[1]
	if strings.HasPrefix(argValue, "@") {
		_, err := os.Stat(argValue[1:])
		if err != nil {
			println(err.Error())
			return "", nil, err
		}
		spec, err = os.ReadFile(argValue[1:])
		if err != nil {
			return "", nil, err
		}

	} else {
		spec = []byte(argValue)
	}
	err = json.Unmarshal([]byte(spec), &desiredLRP)
	if err != nil {
		return "", nil, fmt.Errorf("Invalid JSON: %s", err.Error())
	}
	return processGuid, spec, nil
}

func UpdateDesiredLRP(stdout, stderr io.Writer, bbsClient bbs.Client, processGuid string, spec []byte) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("update-desired-lrp"), traceID)

	var desiredLRP *models.DesiredLRPUpdate
	err := json.Unmarshal(spec, &desiredLRP)
	if err != nil {
		return err
	}

	err = bbsClient.UpdateDesiredLRP(logger, traceID, processGuid, desiredLRP)
	if err != nil {
		return err
	}

	return nil
}
