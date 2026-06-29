package handlers

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

type DomainHandler struct {
	db       db.DomainDB
	exitChan chan<- struct{}
}

var (
	ErrDomainMissing = errors.New("domain missing from request")
	ErrMaxAgeMissing = errors.New("max-age directive missing from request")
)

func NewDomainHandler(db db.DomainDB, exitChan chan<- struct{}) *DomainHandler {
	return &DomainHandler{
		db:       db,
		exitChan: exitChan,
	}
}

func (h *DomainHandler) Domains(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("domains").WithTraceInfo(req)
	response := &models.DomainsResponse{}
	response.Domains, err = h.db.FreshDomains(req.Context(), logger)
	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DomainHandler) Upsert(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("upsert").WithTraceInfo(req)

	request := &models.UpsertDomainRequest{}
	response := &models.UpsertDomainResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		err = h.db.UpsertDomain(req.Context(), logger, request.Domain, request.Ttl)
	}

	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}
