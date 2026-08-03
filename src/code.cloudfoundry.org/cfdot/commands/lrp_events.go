package commands

import (
	"encoding/json"
	"io"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

var (
	lrpEventsCellIdFlag             string
	lrpEventsExcludeActualLRPGroups bool
)

var lrpEventsCmd = &cobra.Command{
	Use:   "lrp-events",
	Short: "Subscribe to BBS LRP events",
	Long:  "Subscribe to BBS LRP events",
	RunE:  lrpEvents,
}

type LRPEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func init() {
	AddBBSFlags(lrpEventsCmd)

	lrpEventsCmd.Flags().StringVarP(&lrpEventsCellIdFlag, "cell-id", "c", "", "retrieve only events for the given cell id")
	lrpEventsCmd.Flags().BoolVarP(&lrpEventsExcludeActualLRPGroups, "exclude-actual-lrp-groups", "x", false, "exclude actual lrp group events")

	RootCmd.AddCommand(lrpEventsCmd)
}

func lrpEvents(cmd *cobra.Command, args []string) error {
	err := validateLRPEventsArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	if !lrpEventsExcludeActualLRPGroups {
		err = printLRPGroupEventsWarning(cmd.OutOrStderr())
		if err != nil {
			return NewCFDotError(cmd, err)
		}
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = LRPEvents(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, lrpEventsCellIdFlag, lrpEventsExcludeActualLRPGroups)
	if err != nil {
		return NewCFDotError(cmd, err)
	}
	return nil
}

func validateLRPEventsArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func LRPEvents(stdout, stderr io.Writer, bbsClient bbs.Client, cellID string, excludeActualLRPGroups bool) error {
	logger := globalLogger.Session("lrp-events")

	oldEventStream := make(chan models.Event)
	newEventStream := make(chan models.Event)
	oldErrChan := make(chan error)
	newErrChan := make(chan error)

	readEvent := func(es events.EventSource, eventStreamChan chan models.Event, errChan chan error) {
		for {
			event, err := es.Next()
			if err != nil {
				errChan <- err
				return
			}
			eventStreamChan <- event
		}
	}

	encoder := json.NewEncoder(stdout)
	eventStreamCount := 1

	if !excludeActualLRPGroups {
		//lint:ignore SA1019 - if this flag is set, we're intentionally using this deprecated behavior in conjunction with the new behavior
		oldES, err := bbsClient.SubscribeToEventsByCellID(logger, cellID)
		if err != nil {
			return models.ConvertError(err)
		}
		defer oldES.Close()

		eventStreamCount += 1

		go readEvent(oldES, oldEventStream, oldErrChan)
	}

	instanceES, err := bbsClient.SubscribeToInstanceEventsByCellID(logger, cellID)
	if err != nil {
		return models.ConvertError(err)
	}
	defer instanceES.Close()

	go readEvent(instanceES, newEventStream, newErrChan)

	ret := &multierror.Error{}
	var event models.Event
	var lrpEvent LRPEvent
	for {
		var err error
		select {
		case e := <-oldEventStream:
			switch e.EventType() {
			//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
			case models.EventTypeActualLRPCreated, models.EventTypeActualLRPChanged, models.EventTypeActualLRPRemoved:
				event = e
			default:
				continue
			}
		case event = <-newEventStream:
		case err = <-oldErrChan:
			multierror.Append(ret, err)
		case err = <-newErrChan:
			multierror.Append(ret, err)
		}

		if len(ret.Errors) >= eventStreamCount {
			for _, err := range ret.Errors {
				if err != io.EOF {
					return ret.ErrorOrNil()
				}
			}
			return nil
		}

		if err != nil {
			continue
		}

		lrpEvent.Type = event.EventType()
		lrpEvent.Data = event
		err = encoder.Encode(lrpEvent)
		if err != nil {
			logger.Error("failed-to-marshal", err)
		}
	}
}

func printLRPGroupEventsWarning(stderr io.Writer) error {
	_, err := io.WriteString(stderr,
		`Event types "actual_lrp_created", "actual_lrp_changed" and "actual_lrp_removed" are deprecated. `+
			`Use "--exclude-actual-lrp-groups" flag to exclude them.`+"\n")
	return err
}
