package rest

import (
	"at.ourproject/energystore/internal/logging/glog"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"encoding/json"
	"errors"
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

// respondWithQueryError answers an error of a data query. A read error of the store (ebow.ErrReadRecord)
// may carry local file paths in its cause: it is logged, the client gets a generic text (#31). Other
// errors are passed through as before.
func respondWithQueryError(w http.ResponseWriter, code int, tenant string, err error) {
	if errors.Is(err, ebow.ErrReadRecord) {
		glog.Errorf("%v tenant=%s", err, tenant)
	}
	respondWithError(w, code, ebow.ClientMessage(err))
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
		glog.Errorf("%v tenant=%s", e.Error(), tenant)
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
