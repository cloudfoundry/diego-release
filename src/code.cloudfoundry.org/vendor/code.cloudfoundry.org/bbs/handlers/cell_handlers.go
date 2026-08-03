package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/lager/v3"
)

type CellHandler struct {
	serviceClient serviceclient.ServiceClient
	exitChan      chan<- struct{}
}

func NewCellHandler(serviceClient serviceclient.ServiceClient, exitChan chan<- struct{}) *CellHandler {
	return &CellHandler{
		serviceClient: serviceClient,
		exitChan:      exitChan,
	}
}

func (h *CellHandler) Cells(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("cells").WithTraceInfo(req)
	response := &models.CellsResponse{}
	cellSet, err := h.serviceClient.Cells(logger)
	cells := []*models.CellPresence{}
	for _, cp := range cellSet {
		cells = append(cells, cp)
	}
	response.Cells = cells
	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}
