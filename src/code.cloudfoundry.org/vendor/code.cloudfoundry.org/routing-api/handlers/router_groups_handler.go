package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-api/uaaclient"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/rata"
)

const portWarning = "Warning: if routes are registered for ports that are not " +
	"in the new range, modifying your load balancer to remove these ports will " +
	"result in backends for those routes becoming inaccessible."

type RouterGroupsHandler struct {
	uaaClient uaaclient.TokenValidator
	logger    lager.Logger
	db        db.DB
}

func NewRouteGroupsHandler(uaaClient uaaclient.TokenValidator, logger lager.Logger, db db.DB) *RouterGroupsHandler {
	return &RouterGroupsHandler{
		uaaClient: uaaClient,
		logger:    logger,
		db:        db,
	}
}

func (h *RouterGroupsHandler) ListRouterGroups(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("list-router-groups")
	log.Debug("started")
	defer log.Debug("completed")

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RouterGroupsReadScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	var routerGroups []models.RouterGroup

	routerGroupName := req.URL.Query().Get("name")
	if routerGroupName != "" {
		var rg models.RouterGroup
		rg, err = h.db.ReadRouterGroupByName(routerGroupName)
		if rg == (models.RouterGroup{}) {
			handleNotFoundError(w, fmt.Errorf("router group '%s' not found", routerGroupName), log)
			return
		}
		routerGroups = []models.RouterGroup{rg}
	} else {
		routerGroups, err = h.db.ReadRouterGroups()
	}

	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}

	jsonBytes, err := json.Marshal(routerGroups)
	if err != nil {
		log.Error("failed-to-marshal", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonBytes)
	if err != nil {
		log.Error("failed-to-write-to-response", err)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
}

func (h *RouterGroupsHandler) UpdateRouterGroup(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("update-router-group")
	log.Debug("started")
	defer log.Debug("completed")
	defer func() {
		err := req.Body.Close()
		if err != nil {
			log.Error("failed-to-close-request-body", err)
		}
	}()

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RouterGroupsWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	bodyDecoder := json.NewDecoder(req.Body)
	var updatedGroup models.RouterGroup
	err = bodyDecoder.Decode(&updatedGroup)

	if err != nil {

		handleProcessRequestError(w, err, log)
		return
	}

	guid := rata.Param(req, "guid")
	rg, err := h.db.ReadRouterGroup(guid)
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}

	if rg == (models.RouterGroup{}) {
		handleNotFoundError(w, fmt.Errorf("router group '%s' does not exist", guid), log)
		return
	}

	if rg.ReservablePorts != updatedGroup.ReservablePorts {
		rg.ReservablePorts = updatedGroup.ReservablePorts

		err = rg.Validate()

		if err != nil {
			handleProcessRequestError(w, err, log)
			return
		}

		err = h.db.SaveRouterGroup(rg)

		if err != nil {
			handleDBCommunicationError(w, err, log)
			return
		}
	}

	jsonBytes, err := json.Marshal(rg)
	if err != nil {
		log.Error("failed-to-marshal", err)
	}

	addWarningsHeader(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonBytes)
	if err != nil {
		log.Error("failed-to-write-to-response", err)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
}

func (h *RouterGroupsHandler) DeleteRouterGroup(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("delete-router-group")
	log.Debug("started")
	defer log.Debug("completed")

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RouterGroupsWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	guid := rata.Param(req, "guid")
	err = h.db.DeleteRouterGroup(guid)
	if err != nil {
		if dberr, ok := err.(db.DBError); !ok || dberr.Type != db.KeyNotFound {
			handleDBCommunicationError(w, err, log)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
	w.Header().Set("Content-Length", "0")
}

func (h *RouterGroupsHandler) CreateRouterGroup(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("create-router-group")
	log.Debug("started")
	defer log.Debug("completed")

	defer func() {
		err := req.Body.Close()
		if err != nil {
			log.Error("failed-to-close-request-body", err)
		}
	}()

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RouterGroupsWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	var rg models.RouterGroup
	bodyDecoder := json.NewDecoder(req.Body)
	err = bodyDecoder.Decode(&rg)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	routerGroups, err := h.db.ReadRouterGroups()
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}
	if existingRg := routerGroupExist(routerGroups, rg); existingRg != nil {
		w.WriteHeader(http.StatusOK)
		writeRouterGroupResponse(w, *existingRg, log)
		return
	}
	guid, err := uuid.NewV4()
	if err != nil {
		handleGuidGenerationError(w, err, log)
		return
	}
	rg.Guid = guid.String()
	routerGroups = append(routerGroups, rg)
	err = routerGroups.Validate()
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	err = h.db.SaveRouterGroup(rg)
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeRouterGroupResponse(w, rg, log)
}

func writeRouterGroupResponse(w http.ResponseWriter, rg models.RouterGroup, log lager.Logger) {
	jsonBytes, err := json.Marshal(rg)
	if err != nil {
		log.Error("failed-to-marshal", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonBytes)
	if err != nil {
		log.Error("failed-to-write-to-response", err)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
}

func routerGroupExist(rgs models.RouterGroups, rg models.RouterGroup) *models.RouterGroup {
	for _, r := range rgs {
		if r.Name == rg.Name && r.Type == rg.Type {
			return &r
		}
	}
	return nil
}

func addWarningsHeader(w http.ResponseWriter) {
	w.Header().Set("X-Cf-Warnings", url.QueryEscape(portWarning))
}
