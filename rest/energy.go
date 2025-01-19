package rest

import (
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/services"
	"at.ourproject/energystore/store"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

func InitQueryApiRouter(r *mux.Router) *mux.Router {
	s := r.PathPrefix("/query").Subrouter()

	s.HandleFunc("/rawdata", middleware.ProtectApi(queryRawData())).Methods("POST")
	s.HandleFunc("/{ecid}/metadata", middleware.ProtectApi(queryMetaData())).Methods("POST")
	return r
}

func queryRawData() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {

		var request struct {
			Cps    []store.TargetMP `json:"cps"`
			EcId   string           `json:"ecId"`
			Start  int64            `json:"start"`
			End    int64            `json:"end"`
			Format *string          `json:"format,omitempty"`
		}

		body, err := io.ReadAll(r.Body)
		glog.V(4).Infof("API-DATA: %s - %v", string(body), err)

		err = json.Unmarshal(body, &request)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		var cps []store.TargetMP
		if len(request.Cps) == 0 {
			start := uint64(request.Start)
			end := uint64(request.End)
			meters, err := services.RequestActiveMeteringPoints(tenant, &start, &end)
			if err != nil {
				glog.Error(err)
				respondWithError(w, http.StatusBadRequest, err.Error())
			}

			cps = make([]store.TargetMP, len(meters))
			for _, meter := range meters {
				cps = append(cps, store.TargetMP{MeteringPoint: meter.MeteringPointId})
			}
		} else {
			cps = request.Cps
		}

		resp, err := store.QueryRawData(tenant, request.EcId, time.UnixMilli(request.Start), time.UnixMilli(request.End), cps, r.URL.Query())
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if request.Format != nil && strings.ToLower(*request.Format) == "csv" {
			responseWithCsv(w, resp, csvConverter)
		}
		respondWithJSON(w, http.StatusOK, &resp)
	}
}

func queryMetaData() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		vars := mux.Vars(r)
		resp, err := store.QueryMetaData(tenant, vars["ecid"])
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}

func csvConverter(data map[string]*store.RawDataResult) ([]byte, error) {

	appendHeader := func(direction model.MeterDirection, name string) []string {
		var header []string
		if direction == model.PRODUCER_DIRECTION {
			header = []string{name, name}
		} else {
			header = []string{name, name, name}
		}
		return header
	}

	header := []string{}
	header2 := []string{}
	internalStruct := map[int64][]string{}
	for m, v := range data {
		header = append(header, appendHeader(v.Direction, m)...)
		header2 = append(header2, appendHeader(v.Direction, string(v.Direction))...)
		for _, d := range v.Data {
			cell := []string{}
			for _, e := range d.Value {
				cell = append(cell, fmt.Sprintf("%f", e))
			}
			if _, ok := internalStruct[d.Ts]; !ok {
				internalStruct[d.Ts] = []string{}
			}
			internalStruct[d.Ts] = append(internalStruct[d.Ts], cell...)
		}
	}

	sortedKeys := make([]int64, 0, len(internalStruct))
	for k := range internalStruct {
		sortedKeys = append(sortedKeys, k)
	}

	sort.Slice(sortedKeys, func(i, j int) bool { return sortedKeys[i] < sortedKeys[j] })
	res := [][]string{{}}
	res = append(res, append([]string{""}, header...))
	res = append(res, append([]string{"time"}, header2...))
	for _, sk := range sortedKeys {
		v := internalStruct[sk]
		res = append(res, append([]string{time.UnixMilli(sk).Format("2006-01-02 15:04:05")}, v...))
	}

	var b bytes.Buffer
	csvWriter := csv.NewWriter(&b)

	err := csvWriter.WriteAll(res)
	return b.Bytes(), err
}
