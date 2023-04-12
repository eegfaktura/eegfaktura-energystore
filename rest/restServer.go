package rest

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/services"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
)

func NewRestServer() *mux.Router {
	jwtWrapper := middleware.JWTMiddleware(viper.GetString("jwt.pubKeyFile"))
	r := mux.NewRouter()
	//s := r.PathPrefix("/rest").Subrouter()
	r.HandleFunc("/eeg/{year}/{month}", jwtWrapper(fetchEnergy())).Methods("GET")
	r.HandleFunc("/eeg/lastRecordDate", jwtWrapper(lastRecordDate())).Methods("GET")
	r.HandleFunc("/eeg/excel/export/{year}/{month}", jwtWrapper(exportMeteringData())).Methods("POST")
	r.HandleFunc("/eeg/hello", getHello).Methods("GET")
	return r
}

func getHello(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"sepp": "hello"})
}

func fetchEnergy() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		var err error
		var year, month int
		vars := mux.Vars(r)
		energy := &model.EegEnergy{}

		fc := ""
		year, err = strconv.Atoi(vars["year"])
		month, err = strconv.Atoi(vars["month"])

		fmt.Printf("FETCH DASHBOARD: %+v %+v (%v/%v) \n", tenant, claims, year, month)

		if energy, err = calculation.EnergyDashboard(tenant, fc, year, month); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := struct {
			Eeg *model.EegEnergy `json:"eeg"`
		}{Eeg: energy}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}

func lastRecordDate() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		lastRecord, err := services.GetLastEnergyEntry(tenant)
		if err != nil {
			respondWithError(w, http.StatusNotFound, "No entry found")
		}
		respondWithJSON(w, http.StatusOK, map[string]interface{}{"periodEnd": lastRecord})
	}
}

func exportMeteringData() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		email := claims.Email
		vars := mux.Vars(r)
		var year, month int
		var err error
		year, err = strconv.Atoi(vars["year"])
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Year not defined")
			return
		}
		month, err = strconv.Atoi(vars["month"])
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Month not defined")
			return
		}

		fmt.Printf("Send Mail to %s\n", email)

		err = excel.ExportEnergyDataToMail(tenant, email, year, month)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
