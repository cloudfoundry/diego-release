package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"

	"code.cloudfoundry.org/lager/v3"
	routing_api "code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/metrics"
)

func handleProcessRequestError(w http.ResponseWriter, procErr error, log lager.Logger) {
	log.Error("error", procErr)
	err := routing_api.NewError(routing_api.ProcessRequestError, "Cannot process request: "+procErr.Error())
	retErr := marshalRoutingApiError(err, log)

	w.WriteHeader(http.StatusBadRequest)
	_, writeErr := w.Write(retErr)
	log.Error("error writing to request", writeErr)
}

func handleNotFoundError(w http.ResponseWriter, err error, log lager.Logger) {
	log.Error("error", err)
	retErr := marshalRoutingApiError(routing_api.NewError(routing_api.ResourceNotFoundError, err.Error()), log)

	w.WriteHeader(http.StatusNotFound)
	_, writeErr := w.Write(retErr)
	log.Error("error writing to request", writeErr)
}

func handleApiError(w http.ResponseWriter, apiErr *routing_api.Error, log lager.Logger) {
	log.Error("error", apiErr)
	retErr := marshalRoutingApiError(*apiErr, log)

	w.WriteHeader(http.StatusBadRequest)
	_, writeErr := w.Write(retErr)
	log.Error("error writing to request", writeErr)
}

func handleDBCommunicationError(w http.ResponseWriter, err error, log lager.Logger) {
	log.Error("error", err)
	retErr := marshalRoutingApiError(routing_api.NewError(routing_api.DBCommunicationError, err.Error()), log)

	w.WriteHeader(http.StatusServiceUnavailable)
	_, writeErr := w.Write(retErr)
	log.Error("error writing to request", writeErr)
}

func handleGuidGenerationError(w http.ResponseWriter, err error, log lager.Logger) {
	log.Error("error", err)
	retErr := marshalRoutingApiError(routing_api.NewError(routing_api.GuidGenerationError, err.Error()), log)

	w.WriteHeader(http.StatusInternalServerError)
	_, writeErr := w.Write(retErr)
	log.Error("error generating guid", writeErr)
}

func handleUnauthorizedError(w http.ResponseWriter, err error, log lager.Logger) {
	log.Error("error", err)

	retErr := marshalRoutingApiError(routing_api.NewError(routing_api.UnauthorizedError, err.Error()), log)
	metrics.IncrementTokenError()

	if bytes.Contains(retErr, []byte("Token does not have ")) {
		newRegEx := regexp.MustCompile("Token does not have .* scope")
		retErr = newRegEx.ReplaceAll(retErr, []byte("You are not authorized to perform the requested action"))
	}
	w.WriteHeader(http.StatusUnauthorized)
	_, writeErr := w.Write(retErr)
	log.Error("error writing to request", writeErr)
}

func marshalRoutingApiError(err routing_api.Error, log lager.Logger) []byte {
	retErr, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		log.Error("could-not-marshal-json", jsonErr)
	}

	return retErr
}
