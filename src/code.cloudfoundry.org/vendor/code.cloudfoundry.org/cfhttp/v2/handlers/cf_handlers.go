package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func WriteInvalidJSONResponse(w http.ResponseWriter, err error) {
	WriteJSONResponse(w, http.StatusBadRequest, HandlerError{
		Error: err.Error(),
	})
}

func WriteInternalErrorJSONResponse(w http.ResponseWriter, err error) {
	WriteJSONResponse(w, http.StatusInternalServerError, HandlerError{
		Error: err.Error(),
	})
}

func WriteStatusCreatedResponse(w http.ResponseWriter) {
	WriteJSONResponse(w, http.StatusCreated, struct{}{})
}

func WriteStatusAcceptedResponse(w http.ResponseWriter) {
	WriteJSONResponse(w, http.StatusAccepted, struct{}{})
}

func WriteJSONResponse(w http.ResponseWriter, statusCode int, jsonObj interface{}) {
	jsonBytes, err := json.Marshal(jsonObj)
	if err != nil {
		panic("Unable to encode JSON: " + err.Error())
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(jsonBytes)
}

type HandlerError struct {
	Error string `json:"error"`
}
