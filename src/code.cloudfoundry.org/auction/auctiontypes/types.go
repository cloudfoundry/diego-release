package auctiontypes

import (
	"errors"
	"strings"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"github.com/tedsuo/ifrit"
)

// Auction Runners

var ErrorCellMismatch = errors.New("found no compatible cell for required rootfs")
var ErrorVolumeDriverMismatch = errors.New("found no compatible cell with required volume drivers")

type PlacementTagMismatchError struct {
	tags []string
}

func NewPlacementTagMismatchError(tags []string) error {
	return PlacementTagMismatchError{tags: tags}
}

func (e PlacementTagMismatchError) Error() string {
	switch len(e.tags) {
	case 0:
		return "found no compatible cell with no placement tags"
	case 1:
		return "found no compatible cell with placement tag \"" + e.tags[0] + "\""
	default:
		count := len(e.tags) - 1
		lastTag := e.tags[count]
		tags := make([]string, count)
		for i, tag := range e.tags[:count] {
			tags[i] = "\"" + tag + "\""
		}
		return "found no compatible cell with placement tags " + strings.Join(tags, ", ") + " and \"" + lastTag + "\""
	}
}

var ErrorNothingToStop = errors.New("nothing to stop")
var ErrorCellCommunication = errors.New("unable to communicate to compatible cells")
var ErrorExceededInflightCreation = errors.New("waiting to start instance: reached in-flight start limit")

//go:generate counterfeiter -o fakes/fake_auction_runner.go . AuctionRunner
type AuctionRunner interface {
	ifrit.Runner
	ScheduleLRPsForAuctions([]auctioneer.LRPStartRequest, string)
	ScheduleTasksForAuctions([]auctioneer.TaskStartRequest, string)
}

type AuctionRunnerDelegate interface {
	FetchCellReps(lager.Logger, string) (map[string]rep.Client, error)
	AuctionCompleted(lager.Logger, string, AuctionResults)
}

//go:generate counterfeiter -o fakes/fake_metric_emitter.go . AuctionMetricEmitterDelegate
type AuctionMetricEmitterDelegate interface {
	FetchStatesCompleted(time.Duration) error
	FailedCellStateRequest() error
	AuctionCompleted(AuctionResults) error
}

type AuctionRequest struct {
	LRPs  []LRPAuction
	Tasks []TaskAuction
}

type AuctionResults struct {
	SuccessfulLRPs  []LRPAuction
	SuccessfulTasks []TaskAuction
	FailedLRPs      []LRPAuction
	FailedTasks     []TaskAuction
}

// LRPStart and Task Auctions

type AuctionRecord struct {
	Winner   string
	Attempts int

	QueueTime    time.Time
	WaitDuration time.Duration

	PlacementError string
}

func NewAuctionRecord(now time.Time) AuctionRecord {
	return AuctionRecord{QueueTime: now}
}

type LRPAuction struct {
	rep.LRP
	AuctionRecord
}

func NewLRPAuction(lrp rep.LRP, now time.Time) LRPAuction {
	return LRPAuction{
		lrp,
		NewAuctionRecord(now),
	}
}

func (a *LRPAuction) Copy() LRPAuction {
	return LRPAuction{a.LRP.Copy(), a.AuctionRecord}
}

type TaskAuction struct {
	rep.Task
	AuctionRecord
}

func NewTaskAuction(task rep.Task, now time.Time) TaskAuction {
	return TaskAuction{
		task,
		NewAuctionRecord(now),
	}
}

func (a *TaskAuction) Copy() TaskAuction {
	return TaskAuction{a.Task.Copy(), a.AuctionRecord}
}
