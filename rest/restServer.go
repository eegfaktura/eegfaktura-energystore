package rest

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/services"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
	"time"
)

func NewRestServer() *mux.Router {
	jwtWrapper := middleware.JWTMiddleware(viper.GetString("jwt.pubKeyFile"))
	r := mux.NewRouter()
	//s := r.PathPrefix("/rest").Subrouter()
	r.HandleFunc("/eeg/{year}/{month}", jwtWrapper(fetchEnergy())).Methods("GET")
	r.HandleFunc("/eeg/report", jwtWrapper(fetchEnergyReport())).Methods("POST")
	r.HandleFunc("/eeg/lastRecordDate", jwtWrapper(lastRecordDate())).Methods("GET")
	r.HandleFunc("/eeg/excel/export/{year}/{month}", jwtWrapper(exportMeteringData())).Methods("POST")
	r.HandleFunc("/eeg/excel/report/download", jwtWrapper(exportReport())).Methods("POST")
	r.HandleFunc("/eeg/hello", getHello).Methods("GET")
	return r
}

type energyReportRequest struct {
	Year    int    `json:"year"`
	Period  string `json:"type"`
	Segment int    `json:"segment"`
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

		glog.V(4).Infof("FETCH DASHBOARD: %+v %+v (%v/%v)", tenant, claims, year, month)

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

func fetchEnergyReport() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		energy := &model.EegEnergy{}

		var request energyReportRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if energy, err = calculation.EnergyReport(tenant, request.Year, request.Segment, request.Period); err != nil {
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

		glog.V(3).Infof("Send Mail to %s", email)

		err = excel.ExportEnergyDataToMail(tenant, email, year, month, nil)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func exportReport() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {

		var cps excel.ExportCPs
		err := json.NewDecoder(r.Body).Decode(&cps)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		b, err := excel.CreateExcelFile(tenant, time.UnixMilli(cps.Start), time.UnixMilli(cps.End), &cps)
		w.Header().Set("Content-type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="myfile.xlsx"`)
		w.Header().Set("filename", fmt.Sprintf("%s-Energy-Report-%s_%s",
			tenant,
			time.UnixMilli(cps.Start).Format("20060102"),
			time.UnixMilli(cps.End).Format("20060102")))

		if _, err := b.WriteTo(w); err != nil {
			fmt.Fprintf(w, "%s", err)
		}
	}
}
