package handlers

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"github.com/gogo/protobuf/proto"
)

//counterfeiter:generate -o fake_controllers/fake_evacuation_controller.go . EvacuationController
type EvacuationController interface {
	RemoveEvacuatingActualLRP(context.Context, lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey) error
	EvacuateClaimedActualLRP(context.Context, lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey) (bool, error)
	EvacuateCrashedActualLRP(context.Context, lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey, string) error
	EvacuateRunningActualLRP(context.Context, lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey, *models.ActualLRPNetInfo, []*models.ActualLRPInternalRoute, map[string]string, bool, string) (bool, error)
	EvacuateStoppedActualLRP(context.Context, lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey) error
}

type EvacuationHandler struct {
	controller EvacuationController
	exitChan   chan<- struct{}
}

func NewEvacuationHandler(
	controller EvacuationController,
	exitChan chan<- struct{},
) *EvacuationHandler {
	return &EvacuationHandler{
		controller: controller,
		exitChan:   exitChan,
	}
}

type MessageValidator interface {
	proto.Message
	Validate() error
	Unmarshal(data []byte) error
}

func (h *EvacuationHandler) RemoveEvacuatingActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("remove-evacuating-actual-lrp").WithTraceInfo(req)
	logger.Info("started")
	defer logger.Info("completed")

	request := &models.RemoveEvacuatingActualLRPRequest{}
	response := &models.RemoveEvacuatingActualLRPResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.RemoveEvacuatingActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
}

func (h *EvacuationHandler) EvacuateClaimedActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("evacuate-claimed-actual-lrp").WithTraceInfo(req)
	logger.Info("started")
	defer logger.Info("completed")

	request := &models.EvacuateClaimedActualLRPRequest{}
	response := &models.EvacuationResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		response.KeepContainer = true
		return
	}

	keepContainer, err := h.controller.EvacuateClaimedActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
	response.KeepContainer = keepContainer
}

func (h *EvacuationHandler) EvacuateCrashedActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("evacuate-crashed-actual-lrp").WithTraceInfo(req)
	logger.Info("started")
	defer logger.Info("completed")

	request := &models.EvacuateCrashedActualLRPRequest{}
	response := &models.EvacuationResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.EvacuateCrashedActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey, request.ErrorMessage)
	response.Error = models.ConvertError(err)
}

func (h *EvacuationHandler) commonEvacuateRunningActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request, useInternalRoutesAndTags bool) {
	logger = logger.Session("evacuate-running-actual-lrp").WithTraceInfo(req)
	logger.Info("starting")
	defer logger.Info("completed")

	response := &models.EvacuationResponse{}
	response.KeepContainer = true
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	request := &models.EvacuateRunningActualLRPRequest{}
	err := parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	var keepContainer bool
	var actualLrpInternalRoutes []*models.ActualLRPInternalRoute
	var metricTags map[string]string
	if useInternalRoutesAndTags {
		actualLrpInternalRoutes = request.ActualLrpInternalRoutes
		metricTags = request.MetricTags
	}
	routable := true
	if request.RoutableExists() {
		r := request.GetRoutable()
		routable = r
	}

	keepContainer, err = h.controller.EvacuateRunningActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey, request.ActualLrpNetInfo, actualLrpInternalRoutes, metricTags, routable, request.AvailabilityZone)
	response.Error = models.ConvertError(err)
	response.KeepContainer = keepContainer
}

func (h *EvacuationHandler) EvacuateRunningActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	useInternalRoutesAndTags := true
	h.commonEvacuateRunningActualLRP(logger, w, req, useInternalRoutesAndTags)
}

func (h *EvacuationHandler) EvacuateRunningActualLRP_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	useInternalRoutesAndTags := false
	h.commonEvacuateRunningActualLRP(logger, w, req, useInternalRoutesAndTags)
}

func (h *EvacuationHandler) EvacuateStoppedActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("evacuate-stopped-actual-lrp").WithTraceInfo(req)

	request := &models.EvacuateStoppedActualLRPRequest{}
	response := &models.EvacuationResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-to-parse-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.EvacuateStoppedActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
}
