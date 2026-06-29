package handlers

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -generate

//counterfeiter:generate -o fake_controllers/fake_actual_lrp_lifecycle_controller.go . ActualLRPLifecycleController
type ActualLRPLifecycleController interface {
	ClaimActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32, actualLRPInstanceKey *models.ActualLRPInstanceKey) error
	StartActualLRP(ctx context.Context,
		logger lager.Logger,
		actualLRPKey *models.ActualLRPKey,
		actualLRPInstanceKey *models.ActualLRPInstanceKey,
		actualLRPNetInfo *models.ActualLRPNetInfo,
		actualLRPInternalRoutes []*models.ActualLRPInternalRoute,
		actualLRPMetricTags map[string]string,
		routable bool,
		availabilityZone string,
	) error
	CrashActualLRP(ctx context.Context, logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, errorMessage string) error
	FailActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error
	RemoveActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) error
	RetireActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey) error
}

type ActualLRPLifecycleHandler struct {
	controller ActualLRPLifecycleController
	exitChan   chan<- struct{}
}

func NewActualLRPLifecycleHandler(
	controller ActualLRPLifecycleController,
	exitChan chan<- struct{},
) *ActualLRPLifecycleHandler {
	return &ActualLRPLifecycleHandler{
		controller: controller,
		exitChan:   exitChan,
	}
}

func (h *ActualLRPLifecycleHandler) ClaimActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("claim-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.ClaimActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.ClaimActualLRP(req.Context(), logger, request.ProcessGuid, request.Index, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) StartActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("start-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.StartActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}
	routable := true
	if request.RoutableExists() {
		r := request.GetRoutable()
		routable = r
	}

	err = h.controller.StartActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey, request.ActualLrpNetInfo, request.ActualLrpInternalRoutes, request.MetricTags, routable, request.AvailabilityZone)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) StartActualLRP_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("start-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.StartActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}
	routable := true
	if request.RoutableExists() {
		r := request.GetRoutable()
		routable = r
	}

	err = h.controller.StartActualLRP(req.Context(), logger, request.ActualLrpKey, request.ActualLrpInstanceKey, request.ActualLrpNetInfo, []*models.ActualLRPInternalRoute{}, nil, routable, request.AvailabilityZone)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) CrashActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("crash-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.CrashActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	actualLRPKey := request.ActualLrpKey
	actualLRPInstanceKey := request.ActualLrpInstanceKey

	err = h.controller.CrashActualLRP(req.Context(), logger, actualLRPKey, actualLRPInstanceKey, request.ErrorMessage)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) FailActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("fail-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.FailActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.FailActualLRP(req.Context(), logger, request.ActualLrpKey, request.ErrorMessage)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) RemoveActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("remove-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.RemoveActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.RemoveActualLRP(req.Context(), logger, request.ProcessGuid, request.Index, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) RetireActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("retire-actual-lrp").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")
	request := &models.RetireActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	var err error
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.RetireActualLRP(trace.ContextWithRequestId(req), logger, request.ActualLrpKey)
	response.Error = models.ConvertError(err)
}
