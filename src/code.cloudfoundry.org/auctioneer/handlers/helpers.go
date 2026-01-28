package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func writeInvalidJSONResponse(w http.ResponseWriter, err error) {
	writeJSONResponse(w, http.StatusBadRequest, HandlerError{
		Error: err.Error(),
	})
}

func writeInternalErrorJSONResponse(w http.ResponseWriter, err error) {
	writeJSONResponse(w, http.StatusInternalServerError, HandlerError{
		Error: err.Error(),
	})
}

func writeStatusAcceptedResponse(w http.ResponseWriter) {
	writeJSONResponse(w, http.StatusAccepted, struct{}{})
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, jsonObj interface{}) {
	jsonBytes, err := json.Marshal(jsonObj)
	if err != nil {
		panic("Unable to encode JSON: " + err.Error())
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	w.Write(jsonBytes)
}

type HandlerError struct {
	Error string `json:"error"`
}
