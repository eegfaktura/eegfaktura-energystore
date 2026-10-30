package rest

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	protobuf "at.ourproject/energystore/protoc"
	"at.ourproject/energystore/services"
	"at.ourproject/energystore/store"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

func InitQueryApiRouter(r *mux.Router) *mux.Router {
	s := r.PathPrefix("/query").Subrouter()

	s.HandleFunc("/rawdata", middleware.ProtectApi(noMembers(queryRawData()))).Methods("POST")
	s.HandleFunc("/{ecid}/metadata", middleware.ProtectApi(noMembers(queryMetaData()))).Methods("POST")
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

		//_maxEnd := time.UnixMilli(request.Start).AddDate(0, 3, 0)
		//_currentEnd := int64(math.Min(float64(request.End), float64(_maxEnd.UnixMilli())))

		var cps []store.TargetMP
		if len(request.Cps) == 0 {
			start := uint64(request.Start)
			end := uint64(request.End)
			meters, err := services.RequestActiveMeteringPoints(tenant, &start, &end)
			if err != nil {
				glog.Error(err)
				respondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			cps = targetsOf(meters)
		} else {
			cps = request.Cps
		}

		resp, err := store.QueryRawData(tenant, request.EcId, time.UnixMilli(request.Start), time.UnixMilli(request.End), cps, r.URL.Query())
		if err != nil {
			respondWithQueryError(w, http.StatusBadRequest, tenant, err)
			return
		}

		if request.Format != nil && strings.ToLower(*request.Format) == "csv" {
			responseWithCsv(w, resp, csvConverter)
			return
		}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}

// targetsOf lists the community's metering points as query targets.
func targetsOf(meters []*protobuf.MeteringPoint) []store.TargetMP {
	cps := make([]store.TargetMP, 0, len(meters))
	for _, meter := range meters {
		cps = append(cps, store.TargetMP{MeteringPoint: meter.MeteringPointId})
	}
	return cps
}

func queryMetaData() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		vars := mux.Vars(r)
		own, ok := ownMeters(w, r, claims, tenant)
		if !ok {
			return
		}
		resp, err := store.QueryMetaData(tenant, vars["ecid"])
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		if own != nil {
			// a member sees the meta data of its own metering points only
			for k := range resp {
				if !own[strings.ToUpper(k)] {
					delete(resp, k)
				}
			}
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
