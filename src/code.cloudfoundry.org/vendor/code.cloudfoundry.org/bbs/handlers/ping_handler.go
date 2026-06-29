package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

type PingHandler struct {
}

func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (h *PingHandler) Ping(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	response := &models.PingResponse{}
	response.Available = true
	writeResponse(w, response)
}
