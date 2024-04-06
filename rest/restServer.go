package rest

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/services"
	"at.ourproject/energystore/store"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"time"
)

func NewRestServer() *mux.Router {
	//jwtWrapper := middleware.JWTMiddleware(viper.GetString("jwt.pubKeyFile"))
	r := mux.NewRouter()
	//s := r.PathPrefix("/rest").Subrouter()
	//r.HandleFunc("/eeg/{year}/{month}", jwtWrapper(fetchEnergy())).Methods("GET")
	r.HandleFunc("/eeg/report", middleware.ProtectApp(fetchEnergyReport())).Methods("POST")
	r.HandleFunc("/eeg/v2/{ecid}/report", middleware.ProtectApp(fetchEnergyReportV2())).Methods("POST")
	r.HandleFunc("/eeg/v2/{ecid}/intradayreport", middleware.ProtectApp(fetchIntraDayReportV2())).Methods("POST")
	r.HandleFunc("/eeg/v2/{ecid}/summary", middleware.ProtectApp(fetchSummaryReportV2())).Methods("POST")
	r.HandleFunc("/eeg/{ecid}/lastRecordDate", middleware.ProtectApp(lastRecordDate())).Methods("GET")
	r.HandleFunc("/eeg/{ecid}/excel/export/{year}/{month}", middleware.ProtectApp(exportMeteringData())).Methods("POST")
	r.HandleFunc("/eeg/{ecid}/excel/report/download", middleware.ProtectApp(exportReport())).Methods("POST")

	r = InitQueryApiRouter(r)
	return r
}

// fetchEnergyReport Rest endpoint retrieve energy values of requested participant and period pattern.
func fetchEnergyReport() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		energy := &model.EegEnergy{}

		var request model.EnergyReportRequest
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

// fetchEnergyReport Rest endpoint retrieve energy values of requested participant and period pattern.
func fetchEnergyReportV2() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		vars := mux.Vars(r)
		ecid := vars["ecid"]
		startMonitor := time.Now()
		fmt.Printf("Start Time Monitor fetchEnergyReport. %d\n", startMonitor.UnixMilli())
		energy := &model.ReportResponse{}

		var request model.ReportRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if energy, err = calculation.EnergyReportV2(tenant, ecid, request.Participants, request.ReportInterval.Year, request.ReportInterval.Segment, request.ReportInterval.Period); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		fmt.Printf("Time Monitor fetchEnergyReport. %v\n", time.Now().Sub(startMonitor))
		//resp := struct {
		//	Eeg *model.ReportResponse `json:"eeg"`
		//}{Eeg: energy}

		respondWithJSON(w, http.StatusOK, &energy)
	}
}

func lastRecordDate() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		vars := mux.Vars(r)
		ecid := vars["ecid"]

		lastRecord, err := services.GetLastEnergyEntry(tenant, ecid)
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
		ecid := vars["ecid"]
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

		err = excel.ExportEnergyDataToMail(tenant, ecid, email, year, month, nil)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func exportReport() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {

		vars := mux.Vars(r)
		ecid := vars["ecid"]

		var cps excel.ExportCPs
		err := json.NewDecoder(r.Body).Decode(&cps)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		//b, err := excel.CreateExcelFile(tenant, time.UnixMilli(cps.Start), time.UnixMilli(cps.End), &cps)
		b, err := excel.ExportEnergyToExcel(tenant, ecid, time.UnixMilli(cps.Start), time.UnixMilli(cps.End), &cps)
		if err != nil {
			glog.Errorf("Create Energy Export: %v", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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

func fetchIntraDayReportV2() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		vars := mux.Vars(r)
		ecid := vars["ecid"]

		startMonitor := time.Now()
		fmt.Printf("Start Time Monitor fetchIntraDayReport. %d\n", startMonitor.UnixMilli())
		var request struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		}

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp, err := store.QueryIntraDayReport(tenant, ecid, time.UnixMilli(request.Start), time.UnixMilli(request.End))
		fmt.Printf("Time Monitor fetchIntraDayReport. %v\n", time.Now().Sub(startMonitor))
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondWithJSON(w, http.StatusOK, &resp)
	}
}

func fetchSummaryReportV2() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		vars := mux.Vars(r)
		ecid := vars["ecid"]

		startMonitor := time.Now()
		fmt.Printf("Start Time Monitor fetchSummaryReport. %d\n", startMonitor.UnixMilli())

		var request model.EnergyReportRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp, err := calculation.EnergySummary(tenant, ecid, request.Year, request.Segment, request.Period)
		fmt.Printf("Time Monitor fetchSummaryReport. %v\n", time.Now().Sub(startMonitor))
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}
