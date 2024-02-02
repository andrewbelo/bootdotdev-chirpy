package routers

import (
	"encoding/json"
	"log"
	"net/http"
)

func marshalError(w http.ResponseWriter, err error, code int) {
	type errorResp struct {
		Error string `json:"error"`
	}
	resp := errorResp{err.Error()}
	dat, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func marshallOK(w http.ResponseWriter, resp interface{}) {
	marshallResponse(w, resp, http.StatusOK)
}

func marshallCreated(w http.ResponseWriter, resp interface{}) {
	marshallResponse(w, resp, http.StatusCreated)
}

func marshallEmptyOK(w http.ResponseWriter) {
	marshallResponse(w, struct{}{}, http.StatusOK)
}

func marshallResponse(w http.ResponseWriter, resp interface{}, status int) {
	dat, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(dat)
}
