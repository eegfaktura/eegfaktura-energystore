package rest

import (
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/store"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

func InitQueryApiRouter(r *mux.Router) *mux.Router {
	s := r.PathPrefix("/query").Subrouter()

	s.HandleFunc("/rawdata", middleware.ProtectApi(queryRawData())).Methods("POST")
	s.HandleFunc("/metadata", middleware.ProtectApi(queryMetaData())).Methods("POST")
	return r
}

func queryRawData() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {

		var request struct {
			Cps   []store.TargetMP `json:"cps"`
			Start int64            `json:"start"`
			End   int64            `json:"end"`
		}

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp, err := store.QueryRawData(tenant, time.UnixMilli(request.Start), time.UnixMilli(request.End), request.Cps, r.URL.Query())
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}

func queryMetaData() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		resp, err := store.QueryMetaData(tenant)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}
