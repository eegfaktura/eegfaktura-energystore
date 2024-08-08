package rest

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type HttpError struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithStatus(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
}

func respondWithHttpError(w http.ResponseWriter, httpCode int, error *HttpError) {
	respondWithJSON(w, httpCode, map[string]interface{}{"error": error})
}

func respondWith(w http.ResponseWriter, httpCode int, tenant, data interface{}) {
	switch e := data.(type) {
	case *model.VfeegError:
		log.WithField("tenant", tenant).Error(e.Error())
		httpStatus := e.HttpCode
		if httpStatus == 0 {
			httpStatus = httpCode
		}
		respondWithHttpError(w, httpStatus, &HttpError{Error: e.Tag, Code: e.Code, Message: e.Error()})
		return
	default:
		respondWithJSON(w, httpCode, data)
	}
}

func responseWithCsv(w http.ResponseWriter, data map[string]*store.RawDataResult, convert func(input map[string]*store.RawDataResult) (output []byte, err error)) {
	content, err := convert(data)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
