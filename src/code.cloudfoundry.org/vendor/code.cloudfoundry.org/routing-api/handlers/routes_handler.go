package handlers

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-api/uaaclient"
)

type RoutesHandler struct {
	uaaClient uaaclient.TokenValidator
	maxTTL    int
	validator RouteValidator
	db        db.DB
	logger    lager.Logger
}

func NewRoutesHandler(uaaClient uaaclient.TokenValidator, maxTTL int, validator RouteValidator, database db.DB, logger lager.Logger) *RoutesHandler {
	return &RoutesHandler{
		uaaClient: uaaClient,
		maxTTL:    maxTTL,
		validator: validator,
		db:        database,
		logger:    logger,
	}
}

func (h *RoutesHandler) List(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("list-routes")

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RoutingRoutesReadScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}
	routes, err := h.db.ReadRoutes()
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}
	encoder := json.NewEncoder(w)
	err = encoder.Encode(routes)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}
}

func (h *RoutesHandler) Upsert(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("create-route")

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RoutingRoutesWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	decoder := json.NewDecoder(req.Body)

	var routes []models.Route
	err = decoder.Decode(&routes)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	log.Info("request", lager.Data{"route_creation": routes})

	// set defaults
	for i := 0; i < len(routes); i++ {
		routes[i].SetDefaults(h.maxTTL)
	}

	apiErr := h.validator.ValidateCreate(routes, h.maxTTL)
	if apiErr != nil {
		handleApiError(w, apiErr, log)
		return
	}

	for _, route := range routes {
		err = h.db.SaveRoute(route)
		if err != nil {
			handleDBCommunicationError(w, err, log)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *RoutesHandler) Delete(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("delete-route")

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RoutingRoutesWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	decoder := json.NewDecoder(req.Body)

	var routes []models.Route
	err = decoder.Decode(&routes)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	log.Info("request", lager.Data{"route_deletion": routes})

	apiErr := h.validator.ValidateDelete(routes)
	if apiErr != nil {
		handleApiError(w, apiErr, log)
		return
	}

	for _, route := range routes {
		err = h.db.DeleteRoute(route)
		if err != nil {
			if dberr, ok := err.(db.DBError); !ok || dberr.Type != db.KeyNotFound {
				handleDBCommunicationError(w, err, log)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
