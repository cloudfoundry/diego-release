package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
)

type LRPAuctionHandler struct {
	runner auctiontypes.AuctionRunner
}

func NewLRPAuctionHandler(runner auctiontypes.AuctionRunner) *LRPAuctionHandler {
	return &LRPAuctionHandler{
		runner: runner,
	}
}

func (*LRPAuctionHandler) logSession(logger lager.Logger) lager.Logger {
	return logger.Session("lrp-auction-handler")
}

func (h *LRPAuctionHandler) Create(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = h.logSession(logger).Session("create").WithTraceInfo(r)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("failed-to-read-request-body", err)
		writeInternalErrorJSONResponse(w, err)
		return
	}

	starts := []auctioneer.LRPStartRequest{}
	err = json.Unmarshal(payload, &starts)
	if err != nil {
		logger.Error("malformed-json", err)
		writeInvalidJSONResponse(w, err)
		return
	}

	validStarts := make([]auctioneer.LRPStartRequest, 0, len(starts))
	lrpGuids := make(map[string][]int)
	for i := range starts {
		start := &starts[i]
		if err := start.Validate(); err == nil {
			validStarts = append(validStarts, *start)
			indices := lrpGuids[start.ProcessGuid]
			indices = append(indices, start.Indices...)
			lrpGuids[start.ProcessGuid] = indices
		} else {
			logger.Error("start-validate-failed", err, lager.Data{"lrp-start": start})
		}
	}

	h.runner.ScheduleLRPsForAuctions(validStarts, trace.RequestIdFromRequest(r))

	logLRPGuids(lrpGuids, logger)

	writeStatusAcceptedResponse(w)
}

func logLRPGuids(lrps map[string][]int, logger lager.Logger) {
	type lrpStruct struct {
		Guid    string `json:"guid"`
		Indices []int  `json:"indices"`
	}

	lrpArray := []lrpStruct{}

	for guid, indices := range lrps {
		lrp := lrpStruct{guid, indices}
		lrpArray = append(lrpArray, lrp)
	}

	logger.Info("submitted", lager.Data{"lrps": lrpArray})
}
